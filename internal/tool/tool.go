package tool

import (
	"context"
	"encoding/json"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// ConversationIDKey is the context key for the conversation ID
const ConversationIDKey contextKey = "conversation_id"

// AgentIDKey is the context key for the current agent ID
const AgentIDKey contextKey = "agent_id"

// WithConversationID returns a context with the conversation ID set
func WithConversationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ConversationIDKey, id)
}

// WithAgentID returns a context with the agent ID set
func WithAgentID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, AgentIDKey, id)
}

// GetAgentID retrieves the agent ID from context
func GetAgentID(ctx context.Context) string {
	if id, ok := ctx.Value(AgentIDKey).(string); ok {
		return id
	}
	return ""
}

// FilesystemRoot represents a configured filesystem root for tools.
type FilesystemRoot struct {
	ID, Name, Path, Description string
}

// AgentFilesystemRootConfig maps a root ID to its per-agent tool permissions.
type AgentFilesystemRootConfig struct {
	RootID       string
	EnabledTools []string
}

type hostEnvVarsKey struct{}

// WithHostEnvVars returns a context with the forwarded host env var names.
func WithHostEnvVars(ctx context.Context, names []string) context.Context {
	return context.WithValue(ctx, hostEnvVarsKey{}, names)
}

// GetHostEnvVars retrieves the forwarded host env var names from context.
func GetHostEnvVars(ctx context.Context) []string {
	names, _ := ctx.Value(hostEnvVarsKey{}).([]string)
	return names
}

type fsToolRootsKey struct{}

// WithFSToolRoots returns a context with per-tool filesystem root mappings.
func WithFSToolRoots(ctx context.Context, m map[string][]FilesystemRoot) context.Context {
	return context.WithValue(ctx, fsToolRootsKey{}, m)
}

// GetFSToolRoots retrieves per-tool filesystem root mappings from context.
func GetFSToolRoots(ctx context.Context) map[string][]FilesystemRoot {
	m, _ := ctx.Value(fsToolRootsKey{}).(map[string][]FilesystemRoot)
	return m
}

// Tool defines a callable tool for an agent
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
	Handler     Handler         `json:"-"`
}

// Handler executes a tool with given arguments
type Handler func(ctx context.Context, args json.RawMessage) (string, error)

// Registry holds available tools
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry creates an empty tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(t *Tool) {
	r.tools[t.Name] = t
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all tools as OpenResponses-compatible definitions
func (r *Registry) List(enabledTools []string) []map[string]any {
	var result []map[string]any
	for _, name := range enabledTools {
		t, ok := r.tools[name]
		if !ok {
			continue
		}
		result = append(result, map[string]any{
			"type":        "function",
			"name":        t.Name,
			"description": t.Description,
			"parameters":  json.RawMessage(t.Parameters),
		})
	}
	return result
}

// Execute runs a tool by name with given arguments
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", &ErrToolNotFound{Name: name}
	}
	return t.Handler(ctx, args)
}

// ErrToolNotFound is returned when a tool is not in the registry
type ErrToolNotFound struct {
	Name string
}

func (e *ErrToolNotFound) Error() string {
	return "tool not found: " + e.Name
}
