package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dstotijn/blippy/internal/openrouter"
	"github.com/dstotijn/blippy/internal/pubsub"
	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
)

// Loop executes the agentic LLM loop, publishing events to a broker.
type Loop struct {
	Queries      *store.Queries
	ORClient     *openrouter.Client
	ToolExecutor *tool.Executor
	Broker       *pubsub.Broker
	DefaultModel string
}

// TurnOpts configures a single agent turn.
type TurnOpts struct {
	Conv        store.Conversation
	Request     *openrouter.ResponseRequest
	UserContent string // for title generation
}

// TextDelta represents a chunk of streamed text from the LLM.
type TextDelta struct {
	Content string
}

// ToolResult represents the outcome of a single tool execution.
type ToolResult struct {
	Name   string
	Input  string
	Result string
}

// MessageDone signals that a message has been persisted.
type MessageDone struct {
	MessageID string
	Role      string
	ItemsJSON string
	CreatedAt string
}

// TurnStarted signals that a new agent turn has begun.
type TurnStarted struct{}

// TurnDone signals that the agent turn has completed.
type TurnDone struct {
	Title string
}

// Error signals that an error occurred during processing.
type Error struct {
	Message string
}

// StoredItem represents an item in the message items JSON array.
type StoredItem struct {
	Type   string `json:"type"`              // "text" or "tool_execution"
	Text   string `json:"text,omitempty"`    // for type="text"
	Name   string `json:"name,omitempty"`    // for type="tool_execution"
	Input  string `json:"input,omitempty"`   // for type="tool_execution"
	Result string `json:"result,omitempty"`  // for type="tool_execution"
	ID     string `json:"id,omitempty"`      // function call ID
	CallID string `json:"call_id,omitempty"` // for history reconstruction
}

// RunTurn executes the agentic loop, publishing events to the broker.
// Returns the assistant's text response.
func (l *Loop) RunTurn(ctx context.Context, opts TurnOpts) (string, error) {
	defer l.Broker.ClearBusy(opts.Conv.ID)

	response, err := l.runLoop(ctx, opts.Conv, opts.Request, opts.UserContent, nil)
	if err != nil {
		l.Broker.Publish(opts.Conv.ID, Error{Message: err.Error()})
		l.Broker.Publish(opts.Conv.ID, TurnDone{})
		return "", err
	}

	return response, nil
}

func (l *Loop) runLoop(ctx context.Context, conv store.Conversation, orReq *openrouter.ResponseRequest, userContent string, priorItems []StoredItem) (string, error) {
	events, errs := l.ORClient.CreateResponseStream(ctx, orReq)

	var currentText string
	var responseID string

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Stream ended — finalize
				var items []StoredItem
				items = append(items, priorItems...)
				if currentText != "" {
					items = append(items, StoredItem{Type: "text", Text: currentText})
				}
				return l.finishTurn(ctx, conv, userContent, items, responseID)
			}

			// Publish text deltas
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				currentText += event.Delta
				l.Broker.Publish(conv.ID, TextDelta{Content: event.Delta})
			}

			// Handle response completion (may contain function calls)
			if event.Response != nil {
				responseID = event.Response.ID

				// Prepare items before ProcessOutput (callback appends to this slice)
				var items []StoredItem
				items = append(items, priorItems...)
				if currentText != "" {
					items = append(items, StoredItem{Type: "text", Text: currentText})
				}

				toolInputs, err := l.ToolExecutor.ProcessOutput(ctx, event.Response.Output, func(r tool.ToolResult) {
					decodedName := tool.DecodeToolName(r.Name)
					items = append(items, StoredItem{
						Type:   "tool_execution",
						ID:     r.ID,
						CallID: r.CallID,
						Name:   decodedName,
						Input:  r.Arguments,
						Result: r.Output,
					})
					l.Broker.Publish(conv.ID, ToolResult{
						Name:   decodedName,
						Input:  r.Arguments,
						Result: r.Output,
					})
				})
				if err != nil {
					return "", fmt.Errorf("process output: %w", err)
				}

				if len(toolInputs) > 0 {
					orReq.Input = append(orReq.Input, toolInputs...)
					return l.runLoop(ctx, conv, orReq, userContent, items)
				}
			}

		case err := <-errs:
			if err != nil {
				return "", fmt.Errorf("stream error: %w", err)
			}

		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (l *Loop) finishTurn(ctx context.Context, conv store.Conversation, userContent string, items []StoredItem, responseID string) (string, error) {
	if len(items) == 0 {
		l.Broker.Publish(conv.ID, TurnDone{})
		return "", nil
	}

	// Persist assistant message
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal items: %w", err)
	}

	msgID := uuid.NewString()
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = l.Queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             msgID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Items:          string(itemsJSON),
		CreatedAt:      createdAt,
	})
	if err != nil {
		return "", fmt.Errorf("create assistant message: %w", err)
	}

	// Publish message_created event
	l.Broker.Publish(conv.ID, MessageDone{
		MessageID: msgID,
		Role:      "assistant",
		ItemsJSON: string(itemsJSON),
		CreatedAt: createdAt,
	})

	// Generate title if this is the first turn
	var title string
	if conv.Title == "" {
		plainText := PlainTextFromItems(items)
		if userContent != "" {
			generated, err := l.ORClient.GenerateTitle(ctx, l.DefaultModel, userContent, plainText)
			if err != nil {
				log.Printf("Failed to generate title: %v", err)
			} else {
				title = generated
			}
		}
	}

	// Update conversation with response ID and title
	now := time.Now().UTC().Format(time.RFC3339)
	if responseID != "" || title != "" {
		newTitle := conv.Title
		if title != "" {
			newTitle = title
		}
		_, err = l.Queries.UpdateConversation(ctx, store.UpdateConversationParams{
			ID:                 conv.ID,
			Title:              newTitle,
			PreviousResponseID: responseID,
			UpdatedAt:          now,
		})
		if err != nil {
			return "", fmt.Errorf("update conversation: %w", err)
		}
	}

	// Publish turn done
	l.Broker.Publish(conv.ID, TurnDone{Title: title})

	return PlainTextFromItems(items), nil
}

// PlainTextFromItems concatenates all text items into a single string.
func PlainTextFromItems(items []StoredItem) string {
	var parts []string
	for _, item := range items {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// BuildHistoryInputs converts a stored message into OpenRouter input items.
func BuildHistoryInputs(msg store.Message) []openrouter.Input {
	var items []StoredItem
	if msg.Items != "" && msg.Items != "[]" {
		_ = json.Unmarshal([]byte(msg.Items), &items)
	}

	if msg.Role == "user" {
		text := PlainTextFromItems(items)
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

		text := PlainTextFromItems(items)
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
