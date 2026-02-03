package agent

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (s *Service) CreateAgent(ctx context.Context, req *connect.Request[CreateAgentRequest]) (*connect.Response[Agent], error) {
	now := time.Now().UTC()

	enabledTools, err := json.Marshal(req.Msg.EnabledTools)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	enabledNotificationChannels, err := json.Marshal(req.Msg.EnabledNotificationChannels)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	agent, err := s.queries.CreateAgent(ctx, store.CreateAgentParams{
		ID:                          uuid.NewString(),
		Name:                        req.Msg.Name,
		Description:                 req.Msg.Description,
		SystemPrompt:                req.Msg.SystemPrompt,
		EnabledTools:                string(enabledTools),
		EnabledNotificationChannels: string(enabledNotificationChannels),
		CreatedAt:                   now.Format(time.RFC3339),
		UpdatedAt:                   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoAgent(agent)), nil
}

func (s *Service) GetAgent(ctx context.Context, req *connect.Request[GetAgentRequest]) (*connect.Response[Agent], error) {
	agent, err := s.queries.GetAgent(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("agent not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoAgent(agent)), nil
}

func (s *Service) ListAgents(ctx context.Context, req *connect.Request[ListAgentsRequest]) (*connect.Response[ListAgentsResponse], error) {
	agents, err := s.queries.ListAgents(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoAgents := make([]*Agent, len(agents))
	for i, a := range agents {
		protoAgents[i] = toProtoAgent(a)
	}

	return connect.NewResponse(&ListAgentsResponse{Agents: protoAgents}), nil
}

func (s *Service) UpdateAgent(ctx context.Context, req *connect.Request[UpdateAgentRequest]) (*connect.Response[Agent], error) {
	enabledTools, err := json.Marshal(req.Msg.EnabledTools)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	enabledNotificationChannels, err := json.Marshal(req.Msg.EnabledNotificationChannels)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	agent, err := s.queries.UpdateAgent(ctx, store.UpdateAgentParams{
		ID:                          req.Msg.Id,
		Name:                        req.Msg.Name,
		Description:                 req.Msg.Description,
		SystemPrompt:                req.Msg.SystemPrompt,
		EnabledTools:                string(enabledTools),
		EnabledNotificationChannels: string(enabledNotificationChannels),
		UpdatedAt:                   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("agent not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoAgent(agent)), nil
}

func (s *Service) DeleteAgent(ctx context.Context, req *connect.Request[DeleteAgentRequest]) (*connect.Response[Empty], error) {
	if err := s.queries.DeleteAgent(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&Empty{}), nil
}

func toProtoAgent(a store.Agent) *Agent {
	var enabledTools []string
	_ = json.Unmarshal([]byte(a.EnabledTools), &enabledTools)

	var enabledNotificationChannels []string
	_ = json.Unmarshal([]byte(a.EnabledNotificationChannels), &enabledNotificationChannels)

	createdAt, _ := time.Parse(time.RFC3339, a.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, a.UpdatedAt)

	return &Agent{
		Id:                          a.ID,
		Name:                        a.Name,
		Description:                 a.Description,
		SystemPrompt:                a.SystemPrompt,
		EnabledTools:                enabledTools,
		EnabledNotificationChannels: enabledNotificationChannels,
		CreatedAt:                   timestamppb.New(createdAt),
		UpdatedAt:                   timestamppb.New(updatedAt),
	}
}
