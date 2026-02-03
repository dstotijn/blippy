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

	"github.com/dstotijn/blippy/internal/openrouter"
	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
)

type Service struct {
	queries      *store.Queries
	db           *sql.DB
	orClient     *openrouter.Client
	model        string
	toolExecutor *tool.Executor
}

func NewService(db *sql.DB, orClient *openrouter.Client, model string, toolExecutor *tool.Executor) *Service {
	return &Service{
		queries:      store.New(db),
		db:           db,
		orClient:     orClient,
		model:        model,
		toolExecutor: toolExecutor,
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

func (s *Service) Chat(ctx context.Context, req *connect.Request[ChatRequest], stream *connect.ServerStream[ChatEvent]) error {
	// Get conversation
	conv, err := s.queries.GetConversation(ctx, req.Msg.ConversationId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("conversation not found"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Get agent for system prompt and tools
	agent, err := s.queries.GetAgent(ctx, conv.AgentID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Get existing messages for conversation history
	existingMsgs, err := s.queries.GetMessagesByConversation(ctx, conv.ID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Save user message
	now := time.Now().UTC()
	userMsgID := uuid.NewString()
	_, err = s.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             userMsgID,
		ConversationID: conv.ID,
		Role:           "user",
		Content:        req.Msg.Content,
		ToolExecutions: "[]",
		CreatedAt:      now.Format(time.RFC3339),
	})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Parse enabled tools from JSON
	var enabledTools []string
	if agent.EnabledTools != "" {
		_ = json.Unmarshal([]byte(agent.EnabledTools), &enabledTools)
	}

	// Parse enabled notification channels from JSON
	var enabledNotificationChannels []string
	if agent.EnabledNotificationChannels != "" {
		_ = json.Unmarshal([]byte(agent.EnabledNotificationChannels), &enabledNotificationChannels)
	}

	// Set conversation ID and agent ID in context for tool execution
	ctx = tool.WithConversationID(ctx, conv.ID)
	ctx = tool.WithAgentID(ctx, conv.AgentID)

	// Build input array with conversation history
	var inputs []openrouter.Input
	for _, msg := range existingMsgs {
		if msg.Role == "user" {
			inputs = append(inputs, openrouter.Input{
				Type: "message",
				Role: "user",
				Content: []openrouter.ContentPart{
					{Type: "input_text", Text: msg.Content},
				},
			})
		} else if msg.Role == "assistant" {
			inputs = append(inputs, openrouter.Input{
				Type:   "message",
				Role:   "assistant",
				ID:     msg.ID, // Use stored message ID
				Status: "completed",
				Content: []openrouter.ContentPart{
					{Type: "output_text", Text: msg.Content},
				},
				Annotations: []any{},
			})
		}
	}
	// Add the new user message
	inputs = append(inputs, openrouter.Input{
		Type: "message",
		Role: "user",
		Content: []openrouter.ContentPart{
			{Type: "input_text", Text: req.Msg.Content},
		},
	})

	// Get tools for agent
	tools, err := s.toolExecutor.GetToolsForAgent(ctx, enabledTools, enabledNotificationChannels)
	if err != nil {
		return fmt.Errorf("get tools: %w", err)
	}

	// Build initial OpenRouter request
	orReq := &openrouter.ResponseRequest{
		Model:        s.model,
		Input:        inputs,
		Instructions: agent.SystemPrompt,
		Tools:        tools,
	}

	return s.streamWithToolExecution(ctx, conv, orReq, userMsgID, req.Msg.Content, stream, nil)
}

// storedToolExec represents a tool execution for JSON storage
type storedToolExec struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Result string `json:"result"`
}

func (s *Service) streamWithToolExecution(
	ctx context.Context,
	conv store.Conversation,
	orReq *openrouter.ResponseRequest,
	userMsgID string,
	userMsgContent string,
	stream *connect.ServerStream[ChatEvent],
	toolExecs []storedToolExec,
) error {
	events, errs := s.orClient.CreateResponseStream(ctx, orReq)

	var fullContent string
	var responseID string

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Stream ended
				return s.finishChat(ctx, conv, userMsgID, userMsgContent, fullContent, responseID, toolExecs, stream)
			}

			// Only stream text deltas, skip function call argument deltas
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				fullContent += event.Delta
				if err := stream.Send(&ChatEvent{
					Event: &ChatEvent_Delta{
						Delta: &ChatDelta{Content: event.Delta},
					},
				}); err != nil {
					return err
				}
			}

			// Handle response completion (may contain function calls)
			if event.Response != nil {
				responseID = event.Response.ID

				// Check for function calls in output
				toolInputs, err := s.toolExecutor.ProcessOutput(ctx, event.Response.Output)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}

				if len(toolInputs) > 0 {
					// Send tool execution events to client and collect for storage
					for _, input := range toolInputs {
						if input.Type != "function_call_output" {
							continue
						}

						// Find the tool name and arguments from the original output
						var toolName, toolArgs string
						for _, out := range event.Response.Output {
							if out.Type == "function_call" && out.CallID == input.CallID {
								toolName = out.Name
								toolArgs = out.Arguments
								break
							}
						}

						// Collect for storage
						toolExecs = append(toolExecs, storedToolExec{
							Name:   toolName,
							Input:  toolArgs,
							Result: input.Output,
						})

						if err := stream.Send(&ChatEvent{
							Event: &ChatEvent_ToolExecution{
								ToolExecution: &ToolExecution{
									Name:   toolName,
									Status: "completed",
									Result: input.Output,
									Input:  toolArgs,
								},
							},
						}); err != nil {
							return err
						}
					}

					// Continue conversation with tool results appended to history
					// OpenRouter doesn't support previous_response_id, so we append to input
					orReq.Input = append(orReq.Input, toolInputs...)

					// Recursively continue streaming with accumulated tool executions
					return s.streamWithToolExecution(ctx, conv, orReq, userMsgID, userMsgContent, stream, toolExecs)
				}
			}

		case err := <-errs:
			if err != nil {
				log.Printf("Stream error: %v", err)
				_ = stream.Send(&ChatEvent{
					Event: &ChatEvent_Error{
						Error: &ChatError{Message: err.Error()},
					},
				})
				return connect.NewError(connect.CodeInternal, err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Service) finishChat(
	ctx context.Context,
	conv store.Conversation,
	userMsgID string,
	userMsgContent string,
	content string,
	responseID string,
	toolExecs []storedToolExec,
	stream *connect.ServerStream[ChatEvent],
) error {
	if content == "" {
		return nil
	}

	// Serialize tool executions to JSON
	toolExecsJSON := "[]"
	if len(toolExecs) > 0 {
		data, err := json.Marshal(toolExecs)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		toolExecsJSON = string(data)
	}

	msgID := uuid.NewString()
	_, err := s.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             msgID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
		ToolExecutions: toolExecsJSON,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Generate title if this is the first turn
	var title string
	if conv.Title == "" {
		generated, err := s.orClient.GenerateTitle(ctx, s.model, userMsgContent, content)
		if err != nil {
			log.Printf("Failed to generate title: %v", err)
		} else {
			title = generated
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if responseID != "" || title != "" {
		newTitle := conv.Title
		if title != "" {
			newTitle = title
		}
		_, err = s.queries.UpdateConversation(ctx, store.UpdateConversationParams{
			ID:                 conv.ID,
			Title:              newTitle,
			PreviousResponseID: responseID,
			UpdatedAt:          now,
		})
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	return stream.Send(&ChatEvent{
		Event: &ChatEvent_Done{
			Done: &ChatDone{
				UserMessageId:      userMsgID,
				AssistantMessageId: msgID,
				ResponseId:         responseID,
				Title:              title,
			},
		},
	})
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

	// Parse tool executions from JSON
	var toolExecs []storedToolExec
	if m.ToolExecutions != "" && m.ToolExecutions != "[]" {
		_ = json.Unmarshal([]byte(m.ToolExecutions), &toolExecs)
	}

	protoToolExecs := make([]*StoredToolExecution, len(toolExecs))
	for i, te := range toolExecs {
		protoToolExecs[i] = &StoredToolExecution{
			Name:   te.Name,
			Input:  te.Input,
			Result: te.Result,
		}
	}

	return &Message{
		Id:             m.ID,
		ConversationId: m.ConversationID,
		Role:           m.Role,
		Content:        m.Content,
		CreatedAt:      timestamppb.New(createdAt),
		ToolExecutions: protoToolExecs,
	}
}
