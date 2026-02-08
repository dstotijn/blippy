package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dstotijn/blippy/internal/agentloop"
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
	queries *store.Queries
	broker  *pubsub.Broker
	loop    *agentloop.Loop
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
func New(queries *store.Queries, broker *pubsub.Broker, loop *agentloop.Loop) *Runner {
	return &Runner{
		queries: queries,
		broker:  broker,
		loop:    loop,
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

	// Create new conversation
	now := time.Now().UTC()
	conv, err := r.queries.CreateConversation(ctx, store.CreateConversationParams{
		ID:                 uuid.NewString(),
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
	if _, err := r.loop.SaveUserMessage(ctx, conv.ID, opts.Prompt); err != nil {
		r.broker.ClearBusy(conv.ID)
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// Execute agentic loop
	response, err := r.loop.RunTurn(ctx, agentloop.TurnOpts{
		Conv:              conv,
		Agent:             agent,
		UserContent:       opts.Prompt,
		ModelOverride:     opts.Model,
		ExtraInstructions: autonomousInstructions,
		Depth:             opts.Depth,
	})
	if err != nil {
		return nil, fmt.Errorf("run turn: %w", err)
	}

	return &RunResult{
		ConversationID: conv.ID,
		Response:       response,
	}, nil
}
