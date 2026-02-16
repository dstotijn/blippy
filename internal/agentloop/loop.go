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
	Conv              store.Conversation
	Agent             store.Agent
	UserContent       string
	History           []store.Message // nil = no history
	ModelOverride     string          // optional: overrides agent model
	ExtraInstructions string          // prepended to system prompt
	Depth             int             // for recursion tracking
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

// SaveUserMessage persists a user message and publishes a MessageDone event.
// Returns the message ID. Call this before starting the turn goroutine so the
// caller can return the ID to the client synchronously.
func (l *Loop) SaveUserMessage(ctx context.Context, convID, content string) (string, error) {
	msgID := uuid.NewString()
	items, _ := json.Marshal([]StoredItem{{Type: "text", Text: content}})
	itemsStr := string(items)
	createdAt := time.Now().UTC().Format(time.RFC3339)

	_, err := l.Queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             msgID,
		ConversationID: convID,
		Role:           "user",
		Items:          itemsStr,
		CreatedAt:      createdAt,
	})
	if err != nil {
		return "", fmt.Errorf("create user message: %w", err)
	}

	l.Broker.Publish(convID, MessageDone{
		MessageID: msgID,
		Role:      "user",
		ItemsJSON: itemsStr,
		CreatedAt: createdAt,
	})

	return msgID, nil
}

// prepareTurn builds the OpenRouter request from TurnOpts.
// Returns the request and per-tool filesystem root mapping for context injection.
func (l *Loop) prepareTurn(ctx context.Context, opts TurnOpts) (*openrouter.ResponseRequest, map[string][]tool.FilesystemRoot, error) {
	// Parse enabled tools from agent JSON
	var enabledTools []string
	if opts.Agent.EnabledTools != "" {
		_ = json.Unmarshal([]byte(opts.Agent.EnabledTools), &enabledTools)
	}

	// Parse enabled notification channels from agent JSON
	var enabledNotificationChannels []string
	if opts.Agent.EnabledNotificationChannels != "" {
		_ = json.Unmarshal([]byte(opts.Agent.EnabledNotificationChannels), &enabledNotificationChannels)
	}

	// Parse per-root filesystem tool config from agent JSON
	var storedFSRoots []struct {
		RootID       string   `json:"root_id"`
		EnabledTools []string `json:"enabled_tools"`
	}
	if opts.Agent.EnabledFilesystemRoots != "" {
		_ = json.Unmarshal([]byte(opts.Agent.EnabledFilesystemRoots), &storedFSRoots)
	}
	fsRootConfigs := make([]tool.AgentFilesystemRootConfig, len(storedFSRoots))
	for i, r := range storedFSRoots {
		fsRootConfigs[i] = tool.AgentFilesystemRootConfig{
			RootID:       r.RootID,
			EnabledTools: r.EnabledTools,
		}
	}

	// Get tools for agent
	tools, fsToolRoots, err := l.ToolExecutor.GetToolsForAgent(ctx, enabledTools, enabledNotificationChannels, fsRootConfigs)
	if err != nil {
		return nil, nil, fmt.Errorf("get tools: %w", err)
	}

	// Resolve model: ModelOverride > Agent.Model > DefaultModel
	model := l.DefaultModel
	if opts.Agent.Model != "" {
		model = opts.Agent.Model
	}
	if opts.ModelOverride != "" {
		model = opts.ModelOverride
	}

	// Build input array with optional conversation history
	var inputs []openrouter.Input
	for _, msg := range opts.History {
		inputs = append(inputs, BuildHistoryInputs(msg)...)
	}
	inputs = append(inputs, openrouter.Input{
		Type: "message",
		Role: "user",
		Content: []openrouter.ContentPart{
			{Type: "input_text", Text: opts.UserContent},
		},
	})

	// Inject memory guidance if any memory tool is enabled.
	var memorySection string
	memoryTools := []string{"memory_view", "memory_create", "memory_edit", "memory_delete"}
	for _, t := range enabledTools {
		for _, mt := range memoryTools {
			if t == mt {
				var sb strings.Builder
				sb.WriteString("## Memory\n")
				sb.WriteString("You have persistent memory across conversations via memory tools.\n")
				sb.WriteString("MEMORY.md is your index file — it is loaded here at the start of every conversation.\n")
				sb.WriteString("Keep MEMORY.md concise and use it to reference detailed topic files (e.g. projects/acme.md).\n")
				sb.WriteString("Always update MEMORY.md when you create or delete other memory files.\n\n")

				file, err := l.Queries.GetAgentFile(ctx, store.GetAgentFileParams{
					AgentID: opts.Agent.ID,
					Path:    "memories/MEMORY.md",
				})
				if err == nil {
					sb.WriteString("### MEMORY.md\n")
					sb.WriteString(file.Content)
					sb.WriteString("\n\n")
				}

				memorySection = sb.String()
				goto doneMemory
			}
		}
	}
doneMemory:

	// Build instructions
	instructions := opts.ExtraInstructions + memorySection + opts.Agent.SystemPrompt

	return &openrouter.ResponseRequest{
		Model:        model,
		Input:        inputs,
		Instructions: instructions,
		Tools:        tools,
	}, fsToolRoots, nil
}

// RunTurn executes the agentic loop, publishing events to the broker.
// Returns the assistant's text response.
func (l *Loop) RunTurn(ctx context.Context, opts TurnOpts) (string, error) {
	defer l.Broker.ClearBusy(opts.Conv.ID)

	// Set context values for tool execution
	ctx = tool.WithConversationID(ctx, opts.Conv.ID)
	ctx = tool.WithAgentID(ctx, opts.Conv.AgentID)
	if opts.Depth > 0 {
		ctx = tool.WithDepth(ctx, opts.Depth)
	}

	var forwardedHostEnvVars []string
	if opts.Agent.ForwardedHostEnvVars != "" {
		_ = json.Unmarshal([]byte(opts.Agent.ForwardedHostEnvVars), &forwardedHostEnvVars)
	}
	if len(forwardedHostEnvVars) > 0 {
		ctx = tool.WithHostEnvVars(ctx, forwardedHostEnvVars)
	}

	orReq, fsToolRoots, err := l.prepareTurn(ctx, opts)
	if err != nil {
		l.Broker.Publish(opts.Conv.ID, Error{Message: err.Error()})
		l.Broker.Publish(opts.Conv.ID, TurnDone{})
		return "", err
	}

	if len(fsToolRoots) > 0 {
		ctx = tool.WithFSToolRoots(ctx, fsToolRoots)
	}

	response, err := l.runLoop(ctx, opts.Conv, orReq, opts.UserContent, nil)
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
