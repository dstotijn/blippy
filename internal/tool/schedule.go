package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// TriggerCreator is the interface for creating triggers.
type TriggerCreator interface {
	CreateTrigger(ctx context.Context, agentID, name, prompt string, cronExpr *string, nextRunAt time.Time, model string) (string, error)
}

type scheduleArgs struct {
	Prompt  string `json:"prompt"`
	Delay   string `json:"delay,omitempty"`
	Cron    string `json:"cron,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	Model   string `json:"model,omitempty"`
}

// NewScheduleAgentRunTool creates a tool for scheduling future agent runs.
func NewScheduleAgentRunTool(creator TriggerCreator) *Tool {
	return &Tool{
		Name:        "schedule_agent_run",
		Description: "Schedule a future agent run. Use delay for one-time runs (e.g., '1h', '30m') or cron for recurring (e.g., '0 9 * * *' for daily at 9am).",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"prompt": {
					"type": "string",
					"description": "The instruction for the scheduled run"
				},
				"delay": {
					"type": "string",
					"description": "Delay before running (e.g., '1h', '30m', '24h'). Mutually exclusive with cron."
				},
				"cron": {
					"type": "string",
					"description": "Cron expression for recurring runs (e.g., '0 9 * * *'). Mutually exclusive with delay."
				},
				"agent_id": {
					"type": "string",
					"description": "Agent to run. Defaults to current agent if not specified."
				},
				"model": {
					"type": "string",
					"description": "Optional model override for the scheduled run"
				}
			},
			"required": ["prompt"]
		}`),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			var args scheduleArgs
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			// Validate prompt required
			if args.Prompt == "" {
				return "", fmt.Errorf("prompt is required")
			}

			// Validate delay/cron mutually exclusive
			if args.Delay != "" && args.Cron != "" {
				return "", fmt.Errorf("delay and cron are mutually exclusive")
			}
			if args.Delay == "" && args.Cron == "" {
				return "", fmt.Errorf("either delay or cron must be specified")
			}

			// Determine agent ID (from args or context)
			agentID := args.AgentID
			if agentID == "" {
				agentID = GetAgentID(ctx)
				if agentID == "" {
					return "", fmt.Errorf("agent_id is required (no current agent in context)")
				}
			}

			var nextRunAt time.Time
			var cronExpr *string

			if args.Delay != "" {
				// Parse delay to compute nextRunAt
				duration, err := time.ParseDuration(args.Delay)
				if err != nil {
					return "", fmt.Errorf("invalid delay format: %w", err)
				}
				if duration <= 0 {
					return "", fmt.Errorf("delay must be positive")
				}
				nextRunAt = time.Now().Add(duration)
			} else {
				// Parse cron expression
				parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
				schedule, err := parser.Parse(args.Cron)
				if err != nil {
					return "", fmt.Errorf("invalid cron expression: %w", err)
				}
				nextRunAt = schedule.Next(time.Now())
				cronExpr = &args.Cron
			}

			// Generate a name from the prompt
			name := truncate(args.Prompt, 50)

			// Call creator.CreateTrigger()
			triggerID, err := creator.CreateTrigger(ctx, agentID, name, args.Prompt, cronExpr, nextRunAt, args.Model)
			if err != nil {
				return "", fmt.Errorf("create trigger: %w", err)
			}

			// Return success message with trigger ID
			if cronExpr != nil {
				return fmt.Sprintf("Scheduled recurring run (trigger %s). Next run at %s.", triggerID, nextRunAt.Format(time.RFC3339)), nil
			}
			return fmt.Sprintf("Scheduled one-time run (trigger %s) at %s.", triggerID, nextRunAt.Format(time.RFC3339)), nil
		},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
