package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dstotijn/blippy/internal/agentloop"
	"github.com/dstotijn/blippy/internal/openrouter"
	"github.com/dstotijn/blippy/internal/pubsub"
	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
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
	broker       *pubsub.Broker
	loop         *agentloop.Loop
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
func New(queries *store.Queries, orClient *openrouter.Client, defaultModel string, toolExecutor *tool.Executor, broker *pubsub.Broker) *Runner {
	return &Runner{
		queries:      queries,
		orClient:     orClient,
		defaultModel: defaultModel,
		toolExecutor: toolExecutor,
		broker:       broker,
		loop: &agentloop.Loop{
			Queries:      queries,
			ORClient:     orClient,
			ToolExecutor: toolExecutor,
			Broker:       broker,
			DefaultModel: defaultModel,
		},
	}
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

	// Mark conversation as busy and publish turn started
	r.broker.SetBusy(conv.ID)
	r.broker.Publish(conv.ID, agentloop.TurnStarted{})

	// Save user message
	userMsgID := uuid.NewString()
	userItems, _ := json.Marshal([]agentloop.StoredItem{{Type: "text", Text: opts.Prompt}})
	userItemsStr := string(userItems)
	createdAt := now.Format(time.RFC3339)
	_, err = r.queries.CreateMessage(ctx, store.CreateMessageParams{
		ID:             userMsgID,
		ConversationID: conv.ID,
		Role:           "user",
		Items:          userItemsStr,
		CreatedAt:      createdAt,
	})
	if err != nil {
		r.broker.ClearBusy(conv.ID)
		return nil, fmt.Errorf("create user message: %w", err)
	}

	// Publish user message event
	r.broker.Publish(conv.ID, agentloop.MessageDone{
		MessageID: userMsgID,
		Role:      "user",
		ItemsJSON: userItemsStr,
		CreatedAt: createdAt,
	})

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
		r.broker.ClearBusy(conv.ID)
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
	response, err := r.loop.RunTurn(ctx, agentloop.TurnOpts{
		Conv:        conv,
		Request:     orReq,
		UserContent: opts.Prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("run turn: %w", err)
	}

	return &RunResult{
		ConversationID: conv.ID,
		Response:       response,
	}, nil
}
