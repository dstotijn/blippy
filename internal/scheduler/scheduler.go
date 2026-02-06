package scheduler

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/dstotijn/blippy/internal/runner"
	"github.com/dstotijn/blippy/internal/store"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const tickInterval = 10 * time.Second

// Scheduler manages trigger execution.
type Scheduler struct {
	db      *sql.DB
	queries *store.Queries
	runner  *runner.Runner

	mu     sync.Mutex
	stop   chan struct{}
	done   chan struct{}
	logger *slog.Logger
}

// New creates a new Scheduler.
func New(db *sql.DB, queries *store.Queries, runner *runner.Runner, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		db:      db,
		queries: queries,
		runner:  runner,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		logger:  logger,
	}
}

// Start begins the scheduler tick loop.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Scheduler) run(ctx context.Context) {
	defer close(s.done)

	// Initial sync of cron triggers
	if err := s.syncCronTriggers(ctx); err != nil {
		s.logger.Error("failed to sync cron triggers on startup", "error", err)
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				s.logger.Error("scheduler tick error", "error", err)
			}
		}
	}
}

func (s *Scheduler) syncCronTriggers(ctx context.Context) error {
	triggers, err := s.queries.ListAllTriggers(ctx)
	if err != nil {
		return err
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	now := time.Now()

	for _, trigger := range triggers {
		// Skip triggers without cron expression
		if !trigger.CronExpr.Valid || trigger.CronExpr.String == "" {
			continue
		}

		schedule, err := parser.Parse(trigger.CronExpr.String)
		if err != nil {
			s.logger.Warn("invalid cron expression", "trigger_id", trigger.ID, "cron_expr", trigger.CronExpr.String, "error", err)
			continue
		}

		nextRun := schedule.Next(now)
		if err := s.queries.UpdateTriggerNextRun(ctx, store.UpdateTriggerNextRunParams{
			ID:        trigger.ID,
			NextRunAt: sql.NullString{String: nextRun.Format(time.RFC3339), Valid: true},
			UpdatedAt: now.Format(time.RFC3339),
		}); err != nil {
			s.logger.Error("failed to update trigger next run", "trigger_id", trigger.ID, "error", err)
		}
	}

	return nil
}

func (s *Scheduler) tick(ctx context.Context) error {
	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	triggers, err := s.queries.GetDueTriggers(ctx, sql.NullString{String: nowStr, Valid: true})
	if err != nil {
		return err
	}

	for _, trigger := range triggers {
		if err := s.executeTrigger(ctx, trigger); err != nil {
			s.logger.Error("failed to execute trigger", "trigger_id", trigger.ID, "error", err)
		}
	}

	return nil
}

func (s *Scheduler) executeTrigger(ctx context.Context, trigger store.Trigger) error {
	now := time.Now()
	nowStr := now.Format(time.RFC3339)
	runID := uuid.NewString()

	// Create trigger run record
	_, err := s.queries.CreateTriggerRun(ctx, store.CreateTriggerRunParams{
		ID:        runID,
		TriggerID: trigger.ID,
		Status:    "running",
		StartedAt: nowStr,
	})
	if err != nil {
		return err
	}

	// Execute the agent run
	result, runErr := s.runner.Run(ctx, runner.RunOpts{
		AgentID: trigger.AgentID,
		Prompt:  trigger.Prompt,
		Depth:   0,
		Model:   trigger.Model,
	})

	// Update trigger run with result
	finishedAt := time.Now().Format(time.RFC3339)
	status := "completed"
	var errorMessage sql.NullString
	var conversationID sql.NullString

	if runErr != nil {
		status = "failed"
		errorMessage = sql.NullString{String: runErr.Error(), Valid: true}
	} else if result != nil {
		conversationID = sql.NullString{String: result.ConversationID, Valid: result.ConversationID != ""}
	}

	if err := s.queries.UpdateTriggerRun(ctx, store.UpdateTriggerRunParams{
		ID:             runID,
		Status:         status,
		ErrorMessage:   errorMessage,
		ConversationID: conversationID,
		FinishedAt:     sql.NullString{String: finishedAt, Valid: true},
	}); err != nil {
		s.logger.Error("failed to update trigger run", "run_id", runID, "error", err)
	}

	// Handle cron vs one-shot triggers
	if trigger.CronExpr.Valid && trigger.CronExpr.String != "" {
		// Cron trigger: compute next run time
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(trigger.CronExpr.String)
		if err != nil {
			s.logger.Error("failed to parse cron expression", "trigger_id", trigger.ID, "error", err)
		} else {
			nextRun := schedule.Next(time.Now())
			if err := s.queries.UpdateTriggerNextRun(ctx, store.UpdateTriggerNextRunParams{
				ID:        trigger.ID,
				NextRunAt: sql.NullString{String: nextRun.Format(time.RFC3339), Valid: true},
				UpdatedAt: time.Now().Format(time.RFC3339),
			}); err != nil {
				s.logger.Error("failed to update trigger next run", "trigger_id", trigger.ID, "error", err)
			}
		}
	} else {
		// One-shot trigger: delete it
		if err := s.queries.DeleteTrigger(ctx, trigger.ID); err != nil {
			s.logger.Error("failed to delete one-shot trigger", "trigger_id", trigger.ID, "error", err)
		}
	}

	if conversationID.Valid {
		s.logger.Info("trigger execution completed", "trigger_id", trigger.ID, "run_id", runID, "conversation_id", conversationID.String)
	}

	return nil
}
