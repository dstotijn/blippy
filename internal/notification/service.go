package notification

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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

func (s *Service) CreateNotificationChannel(ctx context.Context, req *connect.Request[CreateNotificationChannelRequest]) (*connect.Response[NotificationChannel], error) {
	now := time.Now().UTC()

	channel, err := s.queries.CreateNotificationChannel(ctx, store.CreateNotificationChannelParams{
		ID:          uuid.NewString(),
		Name:        req.Msg.Name,
		Type:        req.Msg.Type,
		Config:      req.Msg.Config,
		Description: req.Msg.Description,
		JsonSchema:  req.Msg.JsonSchema,
		CreatedAt:   now.Format(time.RFC3339),
		UpdatedAt:   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoNotificationChannel(channel)), nil
}

func (s *Service) GetNotificationChannel(ctx context.Context, req *connect.Request[GetNotificationChannelRequest]) (*connect.Response[NotificationChannel], error) {
	channel, err := s.queries.GetNotificationChannel(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("notification channel not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoNotificationChannel(channel)), nil
}

func (s *Service) ListNotificationChannels(ctx context.Context, req *connect.Request[ListNotificationChannelsRequest]) (*connect.Response[ListNotificationChannelsResponse], error) {
	channels, err := s.queries.ListNotificationChannels(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoChannels := make([]*NotificationChannel, len(channels))
	for i, c := range channels {
		protoChannels[i] = toProtoNotificationChannel(c)
	}

	return connect.NewResponse(&ListNotificationChannelsResponse{Channels: protoChannels}), nil
}

func (s *Service) UpdateNotificationChannel(ctx context.Context, req *connect.Request[UpdateNotificationChannelRequest]) (*connect.Response[NotificationChannel], error) {
	now := time.Now().UTC()

	channel, err := s.queries.UpdateNotificationChannel(ctx, store.UpdateNotificationChannelParams{
		ID:          req.Msg.Id,
		Name:        req.Msg.Name,
		Type:        req.Msg.Type,
		Config:      req.Msg.Config,
		Description: req.Msg.Description,
		JsonSchema:  req.Msg.JsonSchema,
		UpdatedAt:   now.Format(time.RFC3339),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("notification channel not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoNotificationChannel(channel)), nil
}

func (s *Service) DeleteNotificationChannel(ctx context.Context, req *connect.Request[DeleteNotificationChannelRequest]) (*connect.Response[Empty], error) {
	if err := s.queries.DeleteNotificationChannel(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&Empty{}), nil
}

func toProtoNotificationChannel(c store.NotificationChannel) *NotificationChannel {
	createdAt, _ := time.Parse(time.RFC3339, c.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, c.UpdatedAt)

	return &NotificationChannel{
		Id:          c.ID,
		Name:        c.Name,
		Type:        c.Type,
		Config:      c.Config,
		Description: c.Description,
		JsonSchema:  c.JsonSchema,
		CreatedAt:   timestamppb.New(createdAt),
		UpdatedAt:   timestamppb.New(updatedAt),
	}
}
