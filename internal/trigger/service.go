package trigger

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dstotijn/blippy/internal/store"
)

type Service struct {
	queries *store.Queries
}

func NewService(db *sql.DB) *Service {
	return &Service{
		queries: store.New(db),
	}
}

func (s *Service) CreateTrigger(ctx context.Context, req *connect.Request[CreateTriggerRequest]) (*connect.Response[Trigger], error) {
	now := time.Now().UTC()

	// Compute next_run_at based on cron_expr or delay
	var nextRunAt sql.NullString
	var cronExpr sql.NullString

	if req.Msg.CronExpr != "" {
		// Parse cron expression to compute next run time
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(req.Msg.CronExpr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid cron expression: "+err.Error()))
		}
		nextRun := schedule.Next(now)
		nextRunAt = sql.NullString{String: nextRun.Format(time.RFC3339), Valid: true}
		cronExpr = sql.NullString{String: req.Msg.CronExpr, Valid: true}
	} else if req.Msg.Delay != "" {
		// Parse delay duration
		duration, err := time.ParseDuration(req.Msg.Delay)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid delay duration: "+err.Error()))
		}
		nextRun := now.Add(duration)
		nextRunAt = sql.NullString{String: nextRun.Format(time.RFC3339), Valid: true}
		// No cron expression for one-time delayed triggers
	}

	trigger, err := s.queries.CreateTrigger(ctx, store.CreateTriggerParams{
		ID:        uuid.NewString(),
		AgentID:   req.Msg.AgentId,
		Name:      req.Msg.Name,
		Prompt:    req.Msg.Prompt,
		CronExpr:  cronExpr,
		Enabled:   1, // Enabled by default
		NextRunAt: nextRunAt,
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoTrigger(trigger)), nil
}

func (s *Service) GetTrigger(ctx context.Context, req *connect.Request[GetTriggerRequest]) (*connect.Response[Trigger], error) {
	trigger, err := s.queries.GetTrigger(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("trigger not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoTrigger(trigger)), nil
}

func (s *Service) ListTriggers(ctx context.Context, req *connect.Request[ListTriggersRequest]) (*connect.Response[ListTriggersResponse], error) {
	var triggers []store.Trigger
	var err error

	if req.Msg.AgentId != "" {
		triggers, err = s.queries.ListTriggersByAgent(ctx, req.Msg.AgentId)
	} else {
		triggers, err = s.queries.ListAllTriggers(ctx)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoTriggers := make([]*Trigger, len(triggers))
	for i, t := range triggers {
		protoTriggers[i] = toProtoTrigger(t)
	}

	return connect.NewResponse(&ListTriggersResponse{Triggers: protoTriggers}), nil
}

func (s *Service) UpdateTrigger(ctx context.Context, req *connect.Request[UpdateTriggerRequest]) (*connect.Response[Trigger], error) {
	now := time.Now().UTC()

	// Compute next_run_at if cron_expr is provided
	var nextRunAt sql.NullString
	var cronExpr sql.NullString

	if req.Msg.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(req.Msg.CronExpr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid cron expression: "+err.Error()))
		}
		nextRun := schedule.Next(now)
		nextRunAt = sql.NullString{String: nextRun.Format(time.RFC3339), Valid: true}
		cronExpr = sql.NullString{String: req.Msg.CronExpr, Valid: true}
	}

	var enabled int64
	if req.Msg.Enabled {
		enabled = 1
	}

	trigger, err := s.queries.UpdateTrigger(ctx, store.UpdateTriggerParams{
		ID:        req.Msg.Id,
		Name:      req.Msg.Name,
		Prompt:    req.Msg.Prompt,
		CronExpr:  cronExpr,
		Enabled:   enabled,
		NextRunAt: nextRunAt,
		UpdatedAt: now.Format(time.RFC3339),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("trigger not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoTrigger(trigger)), nil
}

func (s *Service) DeleteTrigger(ctx context.Context, req *connect.Request[DeleteTriggerRequest]) (*connect.Response[Empty], error) {
	if err := s.queries.DeleteTrigger(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&Empty{}), nil
}

func toProtoTrigger(t store.Trigger) *Trigger {
	createdAt, _ := time.Parse(time.RFC3339, t.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, t.UpdatedAt)

	proto := &Trigger{
		Id:        t.ID,
		AgentId:   t.AgentID,
		Name:      t.Name,
		Prompt:    t.Prompt,
		Enabled:   t.Enabled == 1,
		CreatedAt: timestamppb.New(createdAt),
		UpdatedAt: timestamppb.New(updatedAt),
	}

	if t.CronExpr.Valid {
		proto.CronExpr = t.CronExpr.String
	}

	if t.NextRunAt.Valid {
		nextRunAt, _ := time.Parse(time.RFC3339, t.NextRunAt.String)
		proto.NextRunAt = timestamppb.New(nextRunAt)
	}

	return proto
}
