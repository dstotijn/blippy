package runner

import (
	"context"
	"encoding/json"
	"fmt"
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
	model        string
	toolExecutor *tool.Executor
}

// RunOpts configures a single agent run.
type RunOpts struct {
	AgentID string
	Prompt  string
	Depth   int
}

// RunResult contains the outcome of an agent run.
type RunResult struct {
	ConversationID string
	Response       string
}

// New creates a new Runner.
func New(queries *store.Queries, orClient *openrouter.Client, model string, toolExecutor *tool.Executor) *Runner {
	return &Runner{
		queries:      queries,
		orClient:     orClient,
		model:        model,
		toolExecutor: toolExecutor,
	}
}

// storedToolExec represents a tool execution for JSON storage
type storedToolExec struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Result string `json:"result"`
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

	// Create new conversation
	now := time.Now().UTC()
	convID := uuid.NewString()
	conv, err := r.queries.CreateConversation(ctx, store.CreateConversationParams{
		ID:                 convID,
		AgentID:            opts.AgentID,
		Title:              "",
		PreviousResponseID: "",
		CreatedAt:          now.Format(time.RFC3339),
		UpdatedAt:          now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	// Save user message
	userMsgID := uuid.NewString()
	_, err = r.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             userMsgID,
		ConversationID: conv.ID,
		Role:           "user",
		Content:        opts.Prompt,
		ToolExecutions: "[]",
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
		Model:        r.model,
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
func (r *Runner) runLoop(ctx context.Context, conv store.Conversation, orReq *openrouter.ResponseRequest, toolExecs []storedToolExec) (string, error) {
	events, errs := r.orClient.CreateResponseStream(ctx, orReq)

	var fullContent string
	var responseID string

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Stream ended, save assistant message
				if fullContent != "" {
					// Serialize tool executions to JSON
					toolExecsJSON := "[]"
					if len(toolExecs) > 0 {
						data, _ := json.Marshal(toolExecs)
						toolExecsJSON = string(data)
					}

					msgID := uuid.NewString()
					_, err := r.queries.CreateMessage(ctx, store.CreateMessageParams{
						ID:             msgID,
						ConversationID: conv.ID,
						Role:           "assistant",
						Content:        fullContent,
						ToolExecutions: toolExecsJSON,
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
						if userContent != "" {
							generated, err := r.orClient.GenerateTitle(ctx, r.model, userContent, fullContent)
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
				return fullContent, nil
			}

			// Collect text deltas
			if event.Type == "response.output_text.delta" && event.Delta != "" {
				fullContent += event.Delta
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
					// Extract tool execution details from toolInputs
					// toolInputs contains pairs: function_call echo, then function_call_output
					for _, input := range toolInputs {
						if input.Type == "function_call_output" {
							// Find the corresponding function_call to get the name and args
							for _, fc := range toolInputs {
								if fc.Type == "function_call" && fc.CallID == input.CallID {
									toolExecs = append(toolExecs, storedToolExec{
										Name:   fc.Name,
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

					// Recursively continue the loop with accumulated tool executions
					return r.runLoop(ctx, conv, orReq, toolExecs)
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
