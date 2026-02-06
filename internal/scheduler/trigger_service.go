package scheduler

import (
	"context"
	"time"

	"github.com/dstotijn/blippy/internal/store"
	"github.com/google/uuid"
)

// TriggerService provides trigger operations for tools.
type TriggerService struct {
	queries *store.Queries
}

// NewTriggerService creates a new TriggerService.
func NewTriggerService(queries *store.Queries) *TriggerService {
	return &TriggerService{queries: queries}
}

// CreateTrigger creates a new trigger and returns its ID.
func (s *TriggerService) CreateTrigger(ctx context.Context, agentID, name, prompt string, cronExpr *string, nextRunAt time.Time, model, title string) (string, error) {
	now := time.Now().Format(time.RFC3339)
	id := uuid.NewString()

	var cronExprValue string
	if cronExpr != nil {
		cronExprValue = *cronExpr
	}

	_, err := s.queries.CreateTrigger(ctx, store.CreateTriggerParams{
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
