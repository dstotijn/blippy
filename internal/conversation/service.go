package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dstotijn/blippy/internal/agentloop"
	"github.com/dstotijn/blippy/internal/pubsub"
	"github.com/dstotijn/blippy/internal/store"
)

type Service struct {
	queries *store.Queries
	db      *sql.DB
	broker  *pubsub.Broker
	loop    *agentloop.Loop
}

func NewService(db *sql.DB, broker *pubsub.Broker, loop *agentloop.Loop) *Service {
	return &Service{
		queries: store.New(db),
		db:      db,
		broker:  broker,
		loop:    loop,
	}
}

func (s *Service) CreateConversation(ctx context.Context, req *connect.Request[CreateConversationRequest]) (*connect.Response[Conversation], error) {
	now := time.Now().UTC()

	conv, err := s.queries.CreateConversation(ctx, store.CreateConversationParams{
		ID:                 uuid.NewString(),
		AgentID:            req.Msg.AgentId,
		Title:              "",
		PreviousResponseID: "",
		CreatedAt:          now.Format(time.RFC3339),
		UpdatedAt:          now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoConversation(conv)), nil
}

func (s *Service) GetConversation(ctx context.Context, req *connect.Request[GetConversationRequest]) (*connect.Response[Conversation], error) {
	conv, err := s.queries.GetConversation(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("conversation not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoConversation(conv)), nil
}

func (s *Service) ListConversations(ctx context.Context, req *connect.Request[ListConversationsRequest]) (*connect.Response[ListConversationsResponse], error) {
	var convs []store.Conversation
	var err error

	if req.Msg.AgentId != "" {
		convs, err = s.queries.ListConversations(ctx, req.Msg.AgentId)
	} else {
		convs, err = s.queries.ListAllConversations(ctx)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoConvs := make([]*Conversation, len(convs))
	for i, c := range convs {
		protoConvs[i] = toProtoConversation(c)
	}

	return connect.NewResponse(&ListConversationsResponse{Conversations: protoConvs}), nil
}

func (s *Service) DeleteConversation(ctx context.Context, req *connect.Request[DeleteConversationRequest]) (*connect.Response[Empty], error) {
	if err := s.queries.DeleteConversation(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&Empty{}), nil
}

func (s *Service) GetMessages(ctx context.Context, req *connect.Request[GetMessagesRequest]) (*connect.Response[GetMessagesResponse], error) {
	msgs, err := s.queries.GetMessagesByConversation(ctx, req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoMsgs := make([]*Message, len(msgs))
	for i, m := range msgs {
		protoMsgs[i] = toProtoMessage(m)
	}

	return connect.NewResponse(&GetMessagesResponse{Messages: protoMsgs}), nil
}

// Chat saves the user message, starts background LLM processing, and returns immediately.
func (s *Service) Chat(ctx context.Context, req *connect.Request[ChatRequest]) (*connect.Response[ChatResponse], error) {
	// Get conversation
	conv, err := s.queries.GetConversation(ctx, req.Msg.ConversationId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("conversation not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if conversation is already busy
	if !s.broker.SetBusy(conv.ID) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("conversation is already processing"))
	}

	// Get agent for system prompt and tools
	agent, err := s.queries.GetAgent(ctx, conv.AgentID)
	if err != nil {
		s.broker.ClearBusy(conv.ID)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get existing messages for conversation history
	existingMsgs, err := s.queries.GetMessagesByConversation(ctx, conv.ID)
	if err != nil {
		s.broker.ClearBusy(conv.ID)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Save user message
	userMsgID, err := s.loop.SaveUserMessage(ctx, conv.ID, req.Msg.Content)
	if err != nil {
		s.broker.ClearBusy(conv.ID)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish turn started
	s.broker.Publish(conv.ID, agentloop.TurnStarted{})

	// Start background processing (not tied to HTTP request)
	go func() {
		if _, err := s.loop.RunTurn(context.Background(), agentloop.TurnOpts{
			Conv:        conv,
			Agent:       agent,
			UserContent: req.Msg.Content,
			History:     existingMsgs,
		}); err != nil {
			log.Printf("Background agent turn error (conv %s): %v", conv.ID, err)
		}
	}()

	return connect.NewResponse(&ChatResponse{UserMessageId: userMsgID}), nil
}

// WatchEvents streams conversation events to the client via pub/sub.
func (s *Service) WatchEvents(ctx context.Context, req *connect.Request[WatchEventsRequest], stream *connect.ServerStream[WatchEventsEvent]) error {
	convID := req.Msg.ConversationId

	// Validate conversation exists
	if _, err := s.queries.GetConversation(ctx, convID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("conversation not found"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	sub := s.broker.Subscribe(convID)
	defer s.broker.Unsubscribe(sub)

	// If the conversation is currently busy, send initial TurnStarted event
	if s.broker.IsBusy(convID) {
		if err := stream.Send(&WatchEventsEvent{
			Event: &WatchEventsEvent_TurnStarted{TurnStarted: &TurnStarted{}},
		}); err != nil {
			return err
		}
	}

	for {
		select {
		case event, ok := <-sub.C:
			if !ok {
				return nil
			}

			protoEvent, err := toProtoWatchEvent(event)
			if err != nil {
				return err
			}
			if err := stream.Send(protoEvent); err != nil {
				return err
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func toProtoWatchEvent(event any) (*WatchEventsEvent, error) {
	switch e := event.(type) {
	case agentloop.TextDelta:
		return &WatchEventsEvent{
			Event: &WatchEventsEvent_TextDelta{
				TextDelta: &TextDelta{Content: e.Content},
			},
		}, nil
	case agentloop.ToolResult:
		return &WatchEventsEvent{
			Event: &WatchEventsEvent_ToolResult{
				ToolResult: &ToolResult{
					Name:   e.Name,
					Input:  e.Input,
					Result: e.Result,
				},
			},
		}, nil
	case agentloop.MessageDone:
		// Parse items JSON to build proto message
		var items []agentloop.StoredItem
		if e.ItemsJSON != "" && e.ItemsJSON != "[]" {
			_ = json.Unmarshal([]byte(e.ItemsJSON), &items)
		}
		createdAt, _ := time.Parse(time.RFC3339, e.CreatedAt)
		protoItems := storedItemsToProto(items)

		return &WatchEventsEvent{
			Event: &WatchEventsEvent_MessageCreated{
				MessageCreated: &MessageCreated{
					Message: &Message{
						Id:        e.MessageID,
						Role:      e.Role,
						CreatedAt: timestamppb.New(createdAt),
						Items:     protoItems,
					},
				},
			},
		}, nil
	case agentloop.TurnDone:
		return &WatchEventsEvent{
			Event: &WatchEventsEvent_Done{
				Done: &TurnDone{Title: e.Title},
			},
		}, nil
	case agentloop.TurnStarted:
		return &WatchEventsEvent{
			Event: &WatchEventsEvent_TurnStarted{TurnStarted: &TurnStarted{}},
		}, nil
	case agentloop.Error:
		return &WatchEventsEvent{
			Event: &WatchEventsEvent_Error{
				Error: &WatchError{Message: e.Message},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown event type: %T", event)
	}
}

func storedItemsToProto(items []agentloop.StoredItem) []*MessageItem {
	protoItems := make([]*MessageItem, len(items))
	for i, item := range items {
		switch item.Type {
		case "text":
			protoItems[i] = &MessageItem{
				Item: &MessageItem_Text{
					Text: &TextItem{Content: item.Text},
				},
			}
		case "tool_execution":
			protoItems[i] = &MessageItem{
				Item: &MessageItem_ToolExecution{
					ToolExecution: &ToolExecutionItem{
						Name:   item.Name,
						Input:  item.Input,
						Result: item.Result,
					},
				},
			}
		default:
			protoItems[i] = &MessageItem{}
		}
	}
	return protoItems
}

func toProtoConversation(c store.Conversation) *Conversation {
	createdAt, _ := time.Parse(time.RFC3339, c.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, c.UpdatedAt)

	return &Conversation{
		Id:                 c.ID,
		AgentId:            c.AgentID,
		Title:              c.Title,
		PreviousResponseId: c.PreviousResponseID,
		CreatedAt:          timestamppb.New(createdAt),
		UpdatedAt:          timestamppb.New(updatedAt),
	}
}

func toProtoMessage(m store.Message) *Message {
	createdAt, _ := time.Parse(time.RFC3339, m.CreatedAt)

	var items []agentloop.StoredItem
	if m.Items != "" && m.Items != "[]" {
		_ = json.Unmarshal([]byte(m.Items), &items)
	}

	return &Message{
		Id:             m.ID,
		ConversationId: m.ConversationID,
		Role:           m.Role,
		CreatedAt:      timestamppb.New(createdAt),
		Items:          storedItemsToProto(items),
	}
}
