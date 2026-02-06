package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// AgentCaller is the interface for running subagents.
type AgentCaller interface {
	RunAgent(ctx context.Context, agentID, prompt string, depth int, model string) (string, error)
}

type callAgentArgs struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
	Model   string `json:"model,omitempty"`
}

// NewCallAgentTool creates a tool for synchronous subagent invocation.
func NewCallAgentTool(caller AgentCaller) *Tool {
	return &Tool{
		Name:        "call_agent",
		Description: "Call another agent synchronously and get its response. Use this to delegate tasks to specialized agents.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"agent_id": {
					"type": "string",
					"description": "The ID of the agent to call. If omitted, defaults to the current agent."
				},
				"prompt": {
					"type": "string",
					"description": "The instruction for the agent"
				},
				"model": {
					"type": "string",
					"description": "Optional model override for this agent call"
				}
			},
			"required": ["prompt"]
		}`),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			var args callAgentArgs
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			// Determine agent ID (from args or context)
			if args.AgentID == "" {
				args.AgentID = GetAgentID(ctx)
				if args.AgentID == "" {
					return "", fmt.Errorf("agent_id is required (no current agent in context)")
				}
			}
			if args.Prompt == "" {
				return "", fmt.Errorf("prompt is required")
			}

			// Get current depth and check limit
			currentDepth := GetDepth(ctx)
			newDepth := currentDepth + 1

			if newDepth > DefaultMaxDepth {
				return "", fmt.Errorf("max agent depth exceeded (%d)", DefaultMaxDepth)
			}

			// Call the subagent
			response, err := caller.RunAgent(ctx, args.AgentID, args.Prompt, newDepth, args.Model)
			if err != nil {
				return fmt.Sprintf("Error calling agent: %s", err.Error()), nil
			}

			return response, nil
		},
	}
}
