package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dstotijn/blippy/internal/store"
)

const memoryPathPrefix = "memories/"

// FileStore is the interface for agent file persistence.
type FileStore interface {
	UpsertAgentFile(ctx context.Context, arg store.UpsertAgentFileParams) (store.AgentFile, error)
	GetAgentFile(ctx context.Context, arg store.GetAgentFileParams) (store.AgentFile, error)
	ListAgentFiles(ctx context.Context, arg store.ListAgentFilesParams) ([]store.ListAgentFilesRow, error)
	DeleteAgentFile(ctx context.Context, arg store.DeleteAgentFileParams) error
}

func memoryPath(path string) string {
	return memoryPathPrefix + strings.TrimLeft(path, "/")
}

func stripMemoryPrefix(path string) string {
	return strings.TrimPrefix(path, memoryPathPrefix)
}

// NewMemoryViewTool creates a tool for viewing memory files or listing memory directory contents.
func NewMemoryViewTool(fs FileStore) *Tool {
	return &Tool{
		Name:        "memory_view",
		Description: "View your memory files. Without a path (or with a directory path ending in /), lists all files. With a file path, returns the file content.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path to view, or directory path (ending in /) to list. Omit to list all memory files."
				}
			}
		}`),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			agentID := GetAgentID(ctx)
			if agentID == "" {
				return "", fmt.Errorf("no current agent in context")
			}

			// List mode: no path, empty path, or path ending in /
			if args.Path == "" || strings.HasSuffix(args.Path, "/") {
				prefix := memoryPath(args.Path) + "%"
				files, err := fs.ListAgentFiles(ctx, store.ListAgentFilesParams{
					AgentID: agentID,
					Path:    prefix,
				})
				if err != nil {
					return "", fmt.Errorf("list files: %w", err)
				}
				if len(files) == 0 {
					return "No memory files found.", nil
				}
				var sb strings.Builder
				for _, f := range files {
					sb.WriteString(fmt.Sprintf("- %s (updated: %s)\n", stripMemoryPrefix(f.Path), f.UpdatedAt))
				}
				return sb.String(), nil
			}

			// View mode: specific file
			file, err := fs.GetAgentFile(ctx, store.GetAgentFileParams{
				AgentID: agentID,
				Path:    memoryPath(args.Path),
			})
			if err != nil {
				return "", fmt.Errorf("file not found: %s", args.Path)
			}
			return file.Content, nil
		},
	}
}

// NewMemoryCreateTool creates a tool for creating or overwriting memory files.
func NewMemoryCreateTool(fs FileStore) *Tool {
	return &Tool{
		Name:        "memory_create",
		Description: "Create or overwrite a memory file. Use this to save information for future reference across conversations. Always update MEMORY.md to reference any new files you create.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path (e.g. \"MEMORY.md\", \"projects/acme.md\")"
				},
				"content": {
					"type": "string",
					"description": "The file content to write"
				}
			},
			"required": ["path", "content"]
		}`),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			var args struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}
			if args.Path == "" {
				return "", fmt.Errorf("path is required")
			}
			if args.Content == "" {
				return "", fmt.Errorf("content is required")
			}

			agentID := GetAgentID(ctx)
			if agentID == "" {
				return "", fmt.Errorf("no current agent in context")
			}

			now := time.Now().UTC().Format(time.RFC3339)
			_, err := fs.UpsertAgentFile(ctx, store.UpsertAgentFileParams{
				AgentID:   agentID,
				Path:      memoryPath(args.Path),
				Content:   args.Content,
				CreatedAt: now,
				UpdatedAt: now,
			})
			if err != nil {
				return "", fmt.Errorf("create file: %w", err)
			}
			return fmt.Sprintf("File %s saved.", args.Path), nil
		},
	}
}

// NewMemoryEditTool creates a tool for editing memory files via string replacement.
func NewMemoryEditTool(fs FileStore) *Tool {
	return &Tool{
		Name:        "memory_edit",
		Description: "Edit a memory file by replacing a specific string. The old_str must match exactly once in the file.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path to edit"
				},
				"old_str": {
					"type": "string",
					"description": "The exact string to find and replace"
				},
				"new_str": {
					"type": "string",
					"description": "The replacement string"
				}
			},
			"required": ["path", "old_str", "new_str"]
		}`),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			var args struct {
				Path   string `json:"path"`
				OldStr string `json:"old_str"`
				NewStr string `json:"new_str"`
			}
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}
			if args.Path == "" || args.OldStr == "" {
				return "", fmt.Errorf("path and old_str are required")
			}

			agentID := GetAgentID(ctx)
			if agentID == "" {
				return "", fmt.Errorf("no current agent in context")
			}

			fullPath := memoryPath(args.Path)
			file, err := fs.GetAgentFile(ctx, store.GetAgentFileParams{
				AgentID: agentID,
				Path:    fullPath,
			})
			if err != nil {
				return "", fmt.Errorf("file not found: %s", args.Path)
			}

			count := strings.Count(file.Content, args.OldStr)
			if count == 0 {
				return "", fmt.Errorf("old_str not found in %s", args.Path)
			}
			if count > 1 {
				return "", fmt.Errorf("old_str matches %d times in %s (must match exactly once)", count, args.Path)
			}

			newContent := strings.Replace(file.Content, args.OldStr, args.NewStr, 1)
			now := time.Now().UTC().Format(time.RFC3339)
			_, err = fs.UpsertAgentFile(ctx, store.UpsertAgentFileParams{
				AgentID:   agentID,
				Path:      fullPath,
				Content:   newContent,
				CreatedAt: file.CreatedAt,
				UpdatedAt: now,
			})
			if err != nil {
				return "", fmt.Errorf("update file: %w", err)
			}
			return fmt.Sprintf("File %s updated.", args.Path), nil
		},
	}
}

// NewMemoryDeleteTool creates a tool for deleting memory files.
func NewMemoryDeleteTool(fs FileStore) *Tool {
	return &Tool{
		Name:        "memory_delete",
		Description: "Delete a memory file.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path to delete"
				}
			},
			"required": ["path"]
		}`),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}
			if args.Path == "" {
				return "", fmt.Errorf("path is required")
			}

			agentID := GetAgentID(ctx)
			if agentID == "" {
				return "", fmt.Errorf("no current agent in context")
			}

			err := fs.DeleteAgentFile(ctx, store.DeleteAgentFileParams{
				AgentID: agentID,
				Path:    memoryPath(args.Path),
			})
			if err != nil {
				return "", fmt.Errorf("delete file: %w", err)
			}
			return fmt.Sprintf("File %s deleted.", args.Path), nil
		},
	}
}
