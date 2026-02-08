package trigger

import (
	"context"
	"time"

	"github.com/dstotijn/blippy/internal/store"
	"github.com/google/uuid"
)

// Creator provides trigger creation for tools.
// Implements tool.TriggerCreator.
type Creator struct {
	queries *store.Queries
}

// NewCreator creates a new Creator.
func NewCreator(queries *store.Queries) *Creator {
	return &Creator{queries: queries}
}

// CreateTrigger creates a new trigger and returns its ID.
func (c *Creator) CreateTrigger(ctx context.Context, agentID, name, prompt string, cronExpr *string, nextRunAt time.Time, model, title string) (string, error) {
	now := time.Now().Format(time.RFC3339)
	id := uuid.NewString()

	var cronExprValue string
	if cronExpr != nil {
		cronExprValue = *cronExpr
	}

	_, err := c.queries.CreateTrigger(ctx, store.CreateTriggerParams{
		ID:                id,
		AgentID:           agentID,
		Name:              name,
		Prompt:            prompt,
		CronExpr:          store.NewNullString(cronExprValue),
		Enabled:           1,
		NextRunAt:         store.NewNullString(nextRunAt.Format(time.RFC3339)),
		Model:             model,
		ConversationTitle: title,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		return "", err
	}

	return id, nil
}
