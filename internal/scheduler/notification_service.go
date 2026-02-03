package scheduler

import (
	"context"
	"fmt"

	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
)

// NotificationService provides notification channel operations for tools.
type NotificationService struct {
	queries *store.Queries
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(queries *store.Queries) *NotificationService {
	return &NotificationService{queries: queries}
}

// GetNotificationChannel retrieves a channel's type and config by name.
// Used by the legacy notify tool.
func (s *NotificationService) GetNotificationChannel(ctx context.Context, name string) (string, string, error) {
	channel, err := s.queries.GetNotificationChannelByName(ctx, name)
	if err != nil {
		return "", "", fmt.Errorf("channel not found: %w", err)
	}
	return channel.Type, channel.Config, nil
}

// ListNotificationChannelsByIDs returns channels matching the given IDs.
// Implements tool.NotificationChannelLister.
func (s *NotificationService) ListNotificationChannelsByIDs(ctx context.Context, ids []string) ([]tool.NotificationChannel, error) {
	// Fetch all channels and filter by IDs
	// (SQLite doesn't support IN with dynamic arrays easily)
	allChannels, err := s.queries.ListNotificationChannels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	var result []tool.NotificationChannel
	for _, c := range allChannels {
		if idSet[c.ID] {
			result = append(result, tool.NotificationChannel{
				ID:          c.ID,
				Name:        c.Name,
				Description: c.Description,
				JSONSchema:  c.JsonSchema,
				Type:        c.Type,
				Config:      c.Config,
			})
		}
	}
	return result, nil
}

// GetNotificationChannelByName returns a channel by name.
// Implements tool.NotificationChannelLister.
func (s *NotificationService) GetNotificationChannelByName(ctx context.Context, name string) (*tool.NotificationChannel, error) {
	channel, err := s.queries.GetNotificationChannelByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	return &tool.NotificationChannel{
		ID:          channel.ID,
		Name:        channel.Name,
		Description: channel.Description,
		JSONSchema:  channel.JsonSchema,
		Type:        channel.Type,
		Config:      channel.Config,
	}, nil
}
