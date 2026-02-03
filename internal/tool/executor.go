package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dstotijn/blippy/internal/openrouter"
)

// NotificationChannelLister retrieves notification channels.
type NotificationChannelLister interface {
	ListNotificationChannelsByIDs(ctx context.Context, ids []string) ([]NotificationChannel, error)
	GetNotificationChannelByName(ctx context.Context, name string) (*NotificationChannel, error)
}

// Executor handles tool execution within a conversation
type Executor struct {
	registry           *Registry
	notificationLister NotificationChannelLister
}

// NewExecutor creates a tool executor
func NewExecutor(registry *Registry, notificationLister NotificationChannelLister) *Executor {
	return &Executor{
		registry:           registry,
		notificationLister: notificationLister,
	}
}

// ProcessOutput checks response output for function calls and executes them
// Returns inputs to append to conversation for continuation, or nil if no tools called
// Since OpenRouter doesn't support previous_response_id, we need to send both
// the function_call items (echoing what the model said) and function_call_output items
func (e *Executor) ProcessOutput(ctx context.Context, output []openrouter.OutputItem) ([]openrouter.Input, error) {
	var toolCalls []openrouter.OutputItem
	for _, item := range output {
		if item.Type == "function_call" {
			toolCalls = append(toolCalls, item)
		}
	}

	if len(toolCalls) == 0 {
		return nil, nil
	}

	var inputs []openrouter.Input

	// First, echo back the function calls from the model's response
	for _, call := range toolCalls {
		inputs = append(inputs, openrouter.Input{
			Type:      "function_call",
			ID:        call.ID,
			CallID:    call.CallID,
			Name:      call.Name,
			Arguments: call.Arguments,
		})
	}

	// Then, add the outputs for each function call
	for _, call := range toolCalls {
		result, err := e.executeTool(ctx, call.Name, json.RawMessage(call.Arguments))
		if err != nil {
			result = fmt.Sprintf("Error: %s", err.Error())
		}
		// API requires output to be non-empty
		if result == "" {
			result = "(no output)"
		}

		inputs = append(inputs, openrouter.Input{
			Type:   "function_call_output",
			CallID: call.CallID,
			Output: result,
		})
	}

	return inputs, nil
}

// executeTool runs a tool, handling both static registry tools and dynamic notification tools
func (e *Executor) executeTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	// Handle dynamic notification channel tools
	if strings.HasPrefix(name, "notify:") {
		channelName := strings.TrimPrefix(name, "notify:")
		if e.notificationLister == nil {
			return "", fmt.Errorf("notification channels not configured")
		}

		channel, err := e.notificationLister.GetNotificationChannelByName(ctx, channelName)
		if err != nil {
			return fmt.Sprintf("Channel '%s' not found", channelName), nil
		}

		tool := BuildNotificationTool(*channel)
		return tool.Handler(ctx, args)
	}

	// Handle static registry tools
	return e.registry.Execute(ctx, name, args)
}

// GetToolsForAgent returns tool definitions for enabled tools and notification channels
func (e *Executor) GetToolsForAgent(ctx context.Context, enabledTools []string, enabledNotificationChannels []string) ([]map[string]any, error) {
	// Get static tools from registry
	tools := e.registry.List(enabledTools)

	// Add dynamic notification channel tools
	if len(enabledNotificationChannels) > 0 && e.notificationLister != nil {
		channels, err := e.notificationLister.ListNotificationChannelsByIDs(ctx, enabledNotificationChannels)
		if err != nil {
			return nil, fmt.Errorf("list notification channels: %w", err)
		}

		for _, channel := range channels {
			t := BuildNotificationTool(channel)
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
	}

	return tools, nil
}
