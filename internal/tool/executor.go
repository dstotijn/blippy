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

// FilesystemRootLister retrieves filesystem roots.
type FilesystemRootLister interface {
	ListFilesystemRootsByIDs(ctx context.Context, ids []string) ([]FilesystemRoot, error)
}

// Executor handles tool execution within a conversation
type Executor struct {
	registry           *Registry
	notificationLister NotificationChannelLister
	filesystemLister   FilesystemRootLister
}

// NewExecutor creates a tool executor
func NewExecutor(registry *Registry, notificationLister NotificationChannelLister, filesystemLister FilesystemRootLister) *Executor {
	return &Executor{
		registry:           registry,
		notificationLister: notificationLister,
		filesystemLister:   filesystemLister,
	}
}

// ToolResult represents a single completed tool execution.
type ToolResult struct {
	CallID    string
	ID        string // function call ID
	Name      string // API-encoded name
	Arguments string
	Output    string
}

// ProcessOutput checks response output for function calls and executes them concurrently.
// Returns inputs to append to conversation for continuation, or nil if no tools called.
// The onResult callback is invoked for each tool as it completes, enabling callers to
// stream results to clients incrementally. Since OpenRouter doesn't support
// previous_response_id, we send both the function_call items (echoing what the model
// said) and function_call_output items.
func (e *Executor) ProcessOutput(ctx context.Context, output []openrouter.OutputItem, onResult func(ToolResult)) ([]openrouter.Input, error) {
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

	// Execute tools concurrently
	type toolOutput struct {
		index  int
		call   openrouter.OutputItem
		output string
	}

	ch := make(chan toolOutput, len(toolCalls))
	for i, call := range toolCalls {
		go func(i int, call openrouter.OutputItem) {
			internalName := DecodeToolName(call.Name)
			result, err := e.executeTool(ctx, internalName, json.RawMessage(call.Arguments))
			if err != nil {
				result = fmt.Sprintf("Error: %s", err.Error())
			}
			if result == "" {
				result = "(no output)"
			}
			ch <- toolOutput{index: i, call: call, output: result}
		}(i, call)
	}

	// Collect results in completion order, notifying caller as each completes
	outputInputs := make([]openrouter.Input, len(toolCalls))
	for range toolCalls {
		r := <-ch
		outputInputs[r.index] = openrouter.Input{
			Type:   "function_call_output",
			CallID: r.call.CallID,
			Output: r.output,
		}
		if onResult != nil {
			onResult(ToolResult{
				CallID:    r.call.CallID,
				ID:        r.call.ID,
				Name:      r.call.Name,
				Arguments: r.call.Arguments,
				Output:    r.output,
			})
		}
	}

	inputs = append(inputs, outputInputs...)
	return inputs, nil
}

// executeTool runs a tool, handling static registry tools, dynamic notification tools,
// and dynamic filesystem tools.
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

	// Handle dynamic filesystem tools
	if builder, ok := fsToolBuilders[name]; ok {
		toolRoots := GetFSToolRoots(ctx)
		roots := toolRoots[name]
		if len(roots) == 0 {
			return "", fmt.Errorf("no filesystem roots configured for tool %q", name)
		}
		tool := builder(roots)
		return tool.Handler(ctx, args)
	}

	// Handle static registry tools
	return e.registry.Execute(ctx, name, args)
}

// GetToolsForAgent returns tool definitions for enabled tools, notification channels,
// and filesystem roots. Returns a per-tool root mapping for context injection.
// Tool names are encoded for API compatibility (e.g. "notify:" becomes "notify__").
func (e *Executor) GetToolsForAgent(ctx context.Context, enabledTools []string, enabledNotificationChannels []string, fsRootConfigs []AgentFilesystemRootConfig) ([]map[string]any, map[string][]FilesystemRoot, error) {
	tools := e.registry.List(enabledTools)

	// Add dynamic notification channel tools
	if len(enabledNotificationChannels) > 0 && e.notificationLister != nil {
		channels, err := e.notificationLister.ListNotificationChannelsByIDs(ctx, enabledNotificationChannels)
		if err != nil {
			return nil, nil, fmt.Errorf("list notification channels: %w", err)
		}

		for _, channel := range channels {
			t := BuildNotificationTool(channel)
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        EncodeToolName(t.Name),
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
	}

	// Build per-tool filesystem root mapping
	fsToolRoots := make(map[string][]FilesystemRoot)
	if len(fsRootConfigs) > 0 && e.filesystemLister != nil {
		// Collect unique root IDs
		rootIDs := make([]string, 0, len(fsRootConfigs))
		for _, cfg := range fsRootConfigs {
			rootIDs = append(rootIDs, cfg.RootID)
		}

		allRoots, err := e.filesystemLister.ListFilesystemRootsByIDs(ctx, rootIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("list filesystem roots: %w", err)
		}

		// Index roots by ID for lookup
		rootsByID := make(map[string]FilesystemRoot, len(allRoots))
		for _, r := range allRoots {
			rootsByID[r.ID] = r
		}

		// For each root config, add the root to each enabled tool's list
		for _, cfg := range fsRootConfigs {
			root, ok := rootsByID[cfg.RootID]
			if !ok {
				continue
			}
			for _, toolName := range cfg.EnabledTools {
				if _, ok := fsToolBuilders[toolName]; ok {
					fsToolRoots[toolName] = append(fsToolRoots[toolName], root)
				}
			}
		}

		// Build tool definitions for each fs tool that has roots
		for toolName, roots := range fsToolRoots {
			builder := fsToolBuilders[toolName]
			t := builder(roots)
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
	}

	return tools, fsToolRoots, nil
}

// EncodeToolName converts internal tool names to API-safe names.
// "notify:foo" becomes "notify__foo".
func EncodeToolName(name string) string {
	return strings.ReplaceAll(name, ":", "__")
}

// DecodeToolName converts API-safe tool names back to internal names.
// "notify__foo" becomes "notify:foo".
func DecodeToolName(name string) string {
	return strings.Replace(name, "notify__", "notify:", 1)
}
