package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dstotijn/blippy/internal/openrouter"
	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
	"github.com/google/uuid"
)

// autonomousInstructions is prepended to agent system prompts to ensure
// the agent works without user interaction during scheduled/webhook runs.
const autonomousInstructions = `You are running autonomously without user interaction. A user is NOT present and cannot respond to questions or provide feedback.

CRITICAL: You must complete the task independently:
- Do NOT ask clarifying questions or request user input
- Make reasonable assumptions when details are ambiguous
- Use your available tools to accomplish the task
- If a tool call fails, immediately retry with a corrected approach - do not just explain what you would do
- Keep working until the task is complete or truly impossible
- Only stop with a text response when you have finished the task or cannot proceed

`

// Runner executes agent conversations without streaming.
type Runner struct {
	queries      *store.Queries
	orClient     *openrouter.Client
	defaultModel string
	toolExecutor *tool.Executor
}

// RunOpts configures a single agent run.
type RunOpts struct {
	AgentID string
	Prompt  string
	Depth   int
	Model   string
	Title   string
}

// RunResult contains the outcome of an agent run.
type RunResult struct {
	ConversationID string
	Response       string
}

// New creates a new Runner.
func New(queries *store.Queries, orClient *openrouter.Client, defaultModel string, toolExecutor *tool.Executor) *Runner {
	return &Runner{
		queries:      queries,
		orClient:     orClient,
		defaultModel: defaultModel,
		toolExecutor: toolExecutor,
	}
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

// Run executes a conversation with an agent and returns the final response.
func (r *Runner) Run(ctx context.Context, opts RunOpts) (*RunResult, error) {
	// Check depth limit
	if opts.Depth > tool.DefaultMaxDepth {
		return nil, fmt.Errorf("max depth exceeded: %d > %d", opts.Depth, tool.DefaultMaxDepth)
	}

	// Fetch agent from database
	agent, err := r.queries.GetAgent(ctx, opts.AgentID)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}

	// Resolve model: opts.Model > agent.Model > defaultModel
	model := r.defaultModel
	if agent.Model != "" {
		model = agent.Model
	}
	if opts.Model != "" {
		model = opts.Model
	}

	// Create new conversation
	now := time.Now().UTC()
	convID := uuid.NewString()
	conv, err := r.queries.CreateConversation(ctx, store.CreateConversationParams{
		ID:                 convID,
		AgentID:            opts.AgentID,
		Title:              opts.Title,
		PreviousResponseID: "",
		CreatedAt:          now.Format(time.RFC3339),
		UpdatedAt:          now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	// Save user message
	userMsgID := uuid.NewString()
	userItems, _ := json.Marshal([]storedItem{{Type: "text", Text: opts.Prompt}})
	_, err = r.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             userMsgID,
		ConversationID: conv.ID,
		Role:           "user",
		Items:          string(userItems),
		CreatedAt:      now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create user message: %w", err)
	}

	// Set up context with depth and conversation ID
	ctx = tool.WithDepth(ctx, opts.Depth)
	ctx = tool.WithConversationID(ctx, conv.ID)
	ctx = tool.WithAgentID(ctx, opts.AgentID)

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

	// Build initial input
	inputs := []openrouter.Input{
		{
			Type: "message",
			Role: "user",
			Content: []openrouter.ContentPart{
				{Type: "input_text", Text: opts.Prompt},
			},
		},
	}

	// Get tools for agent
	tools, err := r.toolExecutor.GetToolsForAgent(ctx, enabledTools, enabledNotificationChannels)
	if err != nil {
		return nil, fmt.Errorf("get tools: %w", err)
	}

	// Build OpenRouter request with autonomous instructions prepended
	instructions := autonomousInstructions + agent.SystemPrompt
	orReq := &openrouter.ResponseRequest{
		Model:        model,
		Input:        inputs,
		Instructions: instructions,
		Tools:        tools,
	}

	// Execute agentic loop
	response, err := r.runLoop(ctx, conv, orReq, nil)
	if err != nil {
		return nil, fmt.Errorf("run loop: %w", err)
	}

	return &RunResult{
		ConversationID: conv.ID,
		Response:       response,
	}, nil
}

// runLoop executes the agentic loop, processing tool calls until complete.
func (r *Runner) runLoop(ctx context.Context, conv store.Conversation, orReq *openrouter.ResponseRequest, priorItems []storedItem) (string, error) {
	events, errs := r.orClient.CreateResponseStream(ctx, orReq)

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

				if len(items) > 0 {
					itemsJSON, _ := json.Marshal(items)

					msgID := uuid.NewString()
					_, err := r.queries.CreateMessage(ctx, store.CreateMessageParams{
						ID:             msgID,
						ConversationID: conv.ID,
						Role:           "assistant",
						Items:          string(itemsJSON),
						CreatedAt:      time.Now().UTC().Format(time.RFC3339),
					})
					if err != nil {
						return "", fmt.Errorf("create assistant message: %w", err)
					}

					// Generate title if this is the first turn
					title := conv.Title
					if conv.Title == "" {
						// Get user message content from the first input
						var userContent string
						for _, input := range orReq.Input {
							if input.Role == "user" && len(input.Content) > 0 {
								userContent = input.Content[0].Text
								break
							}
						}
						plainText := plainTextFromItems(items)
						if userContent != "" {
							generated, err := r.orClient.GenerateTitle(ctx, r.defaultModel, userContent, plainText)
							if err == nil {
								title = generated
							}
						}
					}

					// Update conversation with response ID and title
					now := time.Now().UTC().Format(time.RFC3339)
					if responseID != "" || title != conv.Title {
						_, err = r.queries.UpdateConversation(ctx, store.UpdateConversationParams{
							ID:                 conv.ID,
							Title:              title,
							PreviousResponseID: responseID,
							UpdatedAt:          now,
						})
						if err != nil {
							return "", fmt.Errorf("update conversation: %w", err)
						}
					}
				}

				return plainTextFromItems(append(priorItems, storedItem{Type: "text", Text: currentText})), nil
			}

			// Collect text deltas
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				currentText += event.Delta
			}

			// Handle response completion (may contain function calls)
			if event.Response != nil {
				responseID = event.Response.ID

				// Check for function calls in output
				toolInputs, err := r.toolExecutor.ProcessOutput(ctx, event.Response.Output)
				if err != nil {
					return "", fmt.Errorf("process output: %w", err)
				}

				if len(toolInputs) > 0 {
					// Finalize current text as an item
					var items []storedItem
					items = append(items, priorItems...)
					if currentText != "" {
						items = append(items, storedItem{Type: "text", Text: currentText})
					}

					// Extract tool execution details
					for _, input := range toolInputs {
						if input.Type == "function_call_output" {
							for _, fc := range toolInputs {
								if fc.Type == "function_call" && fc.CallID == input.CallID {
									items = append(items, storedItem{
										Type:   "tool_execution",
										ID:     fc.ID,
										CallID: input.CallID,
										Name:   tool.DecodeToolName(fc.Name),
										Input:  fc.Arguments,
										Result: input.Output,
									})
									break
								}
							}
						}
					}

					// Continue conversation with tool results appended to history
					orReq.Input = append(orReq.Input, toolInputs...)

					return r.runLoop(ctx, conv, orReq, items)
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
