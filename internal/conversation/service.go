package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
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
	defaultModel string
	toolExecutor *tool.Executor
}

func NewService(db *sql.DB, orClient *openrouter.Client, defaultModel string, toolExecutor *tool.Executor) *Service {
	return &Service{
		queries:      store.New(db),
		db:           db,
		orClient:     orClient,
		defaultModel: defaultModel,
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

	// Resolve model: agent.Model if set, else default
	model := s.defaultModel
	if agent.Model != "" {
		model = agent.Model
	}

	// Get existing messages for conversation history
	existingMsgs, err := s.queries.GetMessagesByConversation(ctx, conv.ID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Save user message
	now := time.Now().UTC()
	userMsgID := uuid.NewString()
	userItems, _ := json.Marshal([]storedItem{{Type: "text", Text: req.Msg.Content}})
	_, err = s.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             userMsgID,
		ConversationID: conv.ID,
		Role:           "user",
		Items:          string(userItems),
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
		inputs = append(inputs, buildHistoryInputs(msg)...)
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
		Model:        model,
		Input:        inputs,
		Instructions: agent.SystemPrompt,
		Tools:        tools,
	}

	return s.streamWithToolExecution(ctx, conv, orReq, userMsgID, req.Msg.Content, stream, nil)
}

// storedItem represents an item in the message items JSON array.
type storedItem struct {
	Type   string `json:"type"`              // "text" or "tool_execution"
	Text   string `json:"text,omitempty"`    // for type="text"
	Name   string `json:"name,omitempty"`    // for type="tool_execution"
	Input  string `json:"input,omitempty"`   // for type="tool_execution"
	Result string `json:"result,omitempty"`  // for type="tool_execution"
	ID     string `json:"id,omitempty"`      // function call ID
	CallID string `json:"call_id,omitempty"` // for history reconstruction
}

func (s *Service) streamWithToolExecution(
	ctx context.Context,
	conv store.Conversation,
	orReq *openrouter.ResponseRequest,
	userMsgID string,
	userMsgContent string,
	stream *connect.ServerStream[ChatEvent],
	priorItems []storedItem,
) error {
	events, errs := s.orClient.CreateResponseStream(ctx, orReq)

	var currentText string
	var responseID string

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Stream ended — finalize current text and store
				var items []storedItem
				items = append(items, priorItems...)
				if currentText != "" {
					items = append(items, storedItem{Type: "text", Text: currentText})
				}
				return s.finishChat(ctx, conv, userMsgID, userMsgContent, items, responseID, stream)
			}

			// Only stream text deltas, skip function call argument deltas
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				currentText += event.Delta
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
					// Finalize current text as an item
					var items []storedItem
					items = append(items, priorItems...)
					if currentText != "" {
						items = append(items, storedItem{Type: "text", Text: currentText})
					}

					// Send tool execution events to client and add to items
					for _, input := range toolInputs {
						if input.Type != "function_call_output" {
							continue
						}

						// Find the tool name, arguments, and ID from the original output
						var toolName, toolArgs, toolID string
						for _, out := range event.Response.Output {
							if out.Type == "function_call" && out.CallID == input.CallID {
								toolID = out.ID
								toolName = tool.DecodeToolName(out.Name)
								toolArgs = out.Arguments
								break
							}
						}

						items = append(items, storedItem{
							Type:   "tool_execution",
							ID:     toolID,
							CallID: input.CallID,
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
					orReq.Input = append(orReq.Input, toolInputs...)

					// Recursively continue streaming with accumulated items
					return s.streamWithToolExecution(ctx, conv, orReq, userMsgID, userMsgContent, stream, items)
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
	items []storedItem,
	responseID string,
	stream *connect.ServerStream[ChatEvent],
) error {
	if len(items) == 0 {
		return nil
	}

	// Serialize items to JSON
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	msgID := uuid.NewString()
	_, err = s.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             msgID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Items:          string(itemsJSON),
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Generate title if this is the first turn
	var title string
	if conv.Title == "" {
		// Derive plain text for title generation
		plainText := plainTextFromItems(items)
		generated, err := s.orClient.GenerateTitle(ctx, s.defaultModel, userMsgContent, plainText)
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

// plainTextFromItems concatenates all text items into a single string.
func plainTextFromItems(items []storedItem) string {
	var parts []string
	for _, item := range items {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// buildHistoryInputs converts a stored message into OpenRouter input items.
func buildHistoryInputs(msg store.Message) []openrouter.Input {
	var items []storedItem
	if msg.Items != "" && msg.Items != "[]" {
		_ = json.Unmarshal([]byte(msg.Items), &items)
	}

	if msg.Role == "user" {
		text := plainTextFromItems(items)
		return []openrouter.Input{{
			Type: "message",
			Role: "user",
			Content: []openrouter.ContentPart{
				{Type: "input_text", Text: text},
			},
		}}
	}

	if msg.Role == "assistant" {
		var inputs []openrouter.Input
		// Emit items in order: text segments become assistant messages,
		// tool executions become function_call + function_call_output pairs.
		for i, item := range items {
			switch item.Type {
			case "tool_execution":
				callID := item.CallID
				if callID == "" {
					callID = fmt.Sprintf("call_%s_%d", msg.ID, i)
				}
				fcID := item.ID
				if fcID == "" {
					fcID = fmt.Sprintf("fc_%s_%d", msg.ID, i)
				}
				inputs = append(inputs, openrouter.Input{
					Type:      "function_call",
					ID:        fcID,
					CallID:    callID,
					Name:      tool.EncodeToolName(item.Name),
					Arguments: item.Input,
				})
				inputs = append(inputs, openrouter.Input{
					Type:   "function_call_output",
					ID:     fmt.Sprintf("fc_out_%s_%d", msg.ID, i),
					CallID: callID,
					Output: item.Result,
				})
			}
		}

		// Emit a single assistant message with all text content combined
		text := plainTextFromItems(items)
		if text != "" {
			inputs = append(inputs, openrouter.Input{
				Type:   "message",
				Role:   "assistant",
				ID:     msg.ID,
				Status: "completed",
				Content: []openrouter.ContentPart{
					{Type: "output_text", Text: text},
				},
				Annotations: []any{},
			})
		}

		return inputs
	}

	return nil
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

	var items []storedItem
	if m.Items != "" && m.Items != "[]" {
		_ = json.Unmarshal([]byte(m.Items), &items)
	}

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

	return &Message{
		Id:             m.ID,
		ConversationId: m.ConversationID,
		Role:           m.Role,
		CreatedAt:      timestamppb.New(createdAt),
		Items:          protoItems,
	}
}
