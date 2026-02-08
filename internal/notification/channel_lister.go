package notification

import (
	"context"
	"fmt"

	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
)

// ChannelLister provides notification channel operations for tools.
// Implements tool.NotificationChannelLister.
type ChannelLister struct {
	queries *store.Queries
}

// NewChannelLister creates a new ChannelLister.
func NewChannelLister(queries *store.Queries) *ChannelLister {
	return &ChannelLister{queries: queries}
}

// ListNotificationChannelsByIDs returns channels matching the given IDs.
func (l *ChannelLister) ListNotificationChannelsByIDs(ctx context.Context, ids []string) ([]tool.NotificationChannel, error) {
	// Fetch all channels and filter by IDs
	// (SQLite doesn't support IN with dynamic arrays easily)
	allChannels, err := l.queries.ListNotificationChannels(ctx)
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
func (l *ChannelLister) GetNotificationChannelByName(ctx context.Context, name string) (*tool.NotificationChannel, error) {
	channel, err := l.queries.GetNotificationChannelByName(ctx, name)
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
