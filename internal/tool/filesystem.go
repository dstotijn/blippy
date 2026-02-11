package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePath securely resolves a relative path within a root directory.
// Rejects absolute paths, ".." components, and symlink escapes.
func resolvePath(rootPath, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	cleaned := filepath.Clean(relativePath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}

	joined := filepath.Join(rootPath, cleaned)
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	absRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}

	if !strings.HasPrefix(resolved, absRoot+string(filepath.Separator)) && resolved != absRoot {
		return "", fmt.Errorf("path escapes root directory")
	}

	return resolved, nil
}

// resolvePathForCreate resolves a path for file creation. The parent directory
// must exist, but the file itself may not.
func resolvePathForCreate(rootPath, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	cleaned := filepath.Clean(relativePath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}

	absRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}

	// Walk up from the target's parent to find the nearest existing ancestor,
	// then verify that ancestor is within the root. This allows fs_create to
	// make intermediate directories without requiring them to already exist.
	targetPath := filepath.Join(absRoot, cleaned)
	ancestor := filepath.Dir(targetPath)
	for ancestor != absRoot {
		if _, err := os.Stat(ancestor); err == nil {
			break
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			// Reached filesystem root without finding an existing dir
			break
		}
		ancestor = parent
	}

	resolvedAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", fmt.Errorf("resolve ancestor: %w", err)
	}

	if !strings.HasPrefix(resolvedAncestor, absRoot+string(filepath.Separator)) && resolvedAncestor != absRoot {
		return "", fmt.Errorf("path escapes root directory")
	}

	return targetPath, nil
}

// findRoot looks up a filesystem root by name.
func findRoot(roots []FilesystemRoot, rootName string) (*FilesystemRoot, error) {
	for _, r := range roots {
		if r.Name == rootName {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("filesystem root %q not found", rootName)
}

func rootEnum(roots []FilesystemRoot) []string {
	names := make([]string, len(roots))
	for i, r := range roots {
		names[i] = r.Name
	}
	return names
}

func rootDescriptions(roots []FilesystemRoot) string {
	var parts []string
	for _, r := range roots {
		desc := r.Name
		if r.Description != "" {
			desc += ": " + r.Description
		}
		parts = append(parts, desc)
	}
	return strings.Join(parts, "; ")
}

// BuildFSViewTool creates the fs_view tool for the given roots.
func BuildFSViewTool(roots []FilesystemRoot) *Tool {
	enumJSON, _ := json.Marshal(rootEnum(roots))
	params := fmt.Sprintf(`{
  "type": "object",
  "properties": {
    "root": {"type": "string", "enum": %s, "description": "Filesystem root name"},
    "path": {"type": "string", "description": "Relative path within the root"},
    "view_range": {
      "type": "array",
      "items": {"type": "integer"},
      "minItems": 2,
      "maxItems": 2,
      "description": "Optional [start_line, end_line] range (1-indexed)"
    }
  },
  "required": ["root", "path"],
  "additionalProperties": false
}`, string(enumJSON))

	return &Tool{
		Name:        "fs_view",
		Description: fmt.Sprintf("View file contents or list directory entries. Available roots: %s", rootDescriptions(roots)),
		Parameters:  json.RawMessage(params),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Root      string `json:"root"`
				Path      string `json:"path"`
				ViewRange []int  `json:"view_range"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			root, err := findRoot(roots, p.Root)
			if err != nil {
				return "", err
			}

			resolved, err := resolvePath(root.Path, p.Path)
			if err != nil {
				return "", err
			}

			info, err := os.Stat(resolved)
			if err != nil {
				return "", fmt.Errorf("stat: %w", err)
			}

			if info.IsDir() {
				entries, err := os.ReadDir(resolved)
				if err != nil {
					return "", fmt.Errorf("read dir: %w", err)
				}
				var lines []string
				for _, e := range entries {
					name := e.Name()
					if e.IsDir() {
						name += "/"
					}
					lines = append(lines, name)
				}
				return strings.Join(lines, "\n"), nil
			}

			// File: check size limit (500KB)
			if info.Size() > 500*1024 {
				return "", fmt.Errorf("file too large (%d bytes, max 512000)", info.Size())
			}

			data, err := os.ReadFile(resolved)
			if err != nil {
				return "", fmt.Errorf("read file: %w", err)
			}

			lines := strings.Split(string(data), "\n")

			// Apply view_range if specified
			if len(p.ViewRange) == 2 {
				start := p.ViewRange[0]
				end := p.ViewRange[1]
				if start < 1 {
					start = 1
				}
				if end > len(lines) {
					end = len(lines)
				}
				if start > len(lines) {
					return "", fmt.Errorf("start line %d exceeds file length %d", start, len(lines))
				}
				lines = lines[start-1 : end]
				// Number lines from start
				var numbered []string
				for i, line := range lines {
					numbered = append(numbered, fmt.Sprintf("%6d\t%s", start+i, line))
				}
				return strings.Join(numbered, "\n"), nil
			}

			// Return all lines with line numbers
			var numbered []string
			for i, line := range lines {
				numbered = append(numbered, fmt.Sprintf("%6d\t%s", i+1, line))
			}
			return strings.Join(numbered, "\n"), nil
		},
	}
}

// BuildFSStrReplaceTool creates the fs_str_replace tool for the given roots.
func BuildFSStrReplaceTool(roots []FilesystemRoot) *Tool {
	enumJSON, _ := json.Marshal(rootEnum(roots))
	params := fmt.Sprintf(`{
  "type": "object",
  "properties": {
    "root": {"type": "string", "enum": %s, "description": "Filesystem root name"},
    "path": {"type": "string", "description": "Relative path within the root"},
    "old_str": {"type": "string", "description": "Exact string to find (must match exactly once)"},
    "new_str": {"type": "string", "description": "Replacement string"}
  },
  "required": ["root", "path", "old_str", "new_str"],
  "additionalProperties": false
}`, string(enumJSON))

	return &Tool{
		Name:        "fs_str_replace",
		Description: fmt.Sprintf("Replace an exact string occurrence in a file. The old_str must appear exactly once. Available roots: %s", rootDescriptions(roots)),
		Parameters:  json.RawMessage(params),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Root   string `json:"root"`
				Path   string `json:"path"`
				OldStr string `json:"old_str"`
				NewStr string `json:"new_str"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			root, err := findRoot(roots, p.Root)
			if err != nil {
				return "", err
			}

			resolved, err := resolvePath(root.Path, p.Path)
			if err != nil {
				return "", err
			}

			data, err := os.ReadFile(resolved)
			if err != nil {
				return "", fmt.Errorf("read file: %w", err)
			}

			content := string(data)
			count := strings.Count(content, p.OldStr)
			if count == 0 {
				return "", fmt.Errorf("old_str not found in file")
			}
			if count > 1 {
				return "", fmt.Errorf("old_str appears %d times, must be unique", count)
			}

			newContent := strings.Replace(content, p.OldStr, p.NewStr, 1)
			if err := os.WriteFile(resolved, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("write file: %w", err)
			}

			return "File updated successfully.", nil
		},
	}
}

// BuildFSCreateTool creates the fs_create tool for the given roots.
func BuildFSCreateTool(roots []FilesystemRoot) *Tool {
	enumJSON, _ := json.Marshal(rootEnum(roots))
	params := fmt.Sprintf(`{
  "type": "object",
  "properties": {
    "root": {"type": "string", "enum": %s, "description": "Filesystem root name"},
    "path": {"type": "string", "description": "Relative path for the new file"},
    "file_text": {"type": "string", "description": "Content of the new file"}
  },
  "required": ["root", "path", "file_text"],
  "additionalProperties": false
}`, string(enumJSON))

	return &Tool{
		Name:        "fs_create",
		Description: fmt.Sprintf("Create a new file. Fails if the file already exists. Available roots: %s", rootDescriptions(roots)),
		Parameters:  json.RawMessage(params),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Root     string `json:"root"`
				Path     string `json:"path"`
				FileText string `json:"file_text"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			root, err := findRoot(roots, p.Root)
			if err != nil {
				return "", err
			}

			resolved, err := resolvePathForCreate(root.Path, p.Path)
			if err != nil {
				return "", err
			}

			// Check if file already exists
			if _, err := os.Stat(resolved); err == nil {
				return "", fmt.Errorf("file already exists: %s", p.Path)
			}

			// Create parent directories if needed
			dir := filepath.Dir(resolved)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("create directories: %w", err)
			}

			if err := os.WriteFile(resolved, []byte(p.FileText), 0644); err != nil {
				return "", fmt.Errorf("write file: %w", err)
			}

			return "File created successfully.", nil
		},
	}
}

// BuildFSInsertTool creates the fs_insert tool for the given roots.
func BuildFSInsertTool(roots []FilesystemRoot) *Tool {
	enumJSON, _ := json.Marshal(rootEnum(roots))
	params := fmt.Sprintf(`{
  "type": "object",
  "properties": {
    "root": {"type": "string", "enum": %s, "description": "Filesystem root name"},
    "path": {"type": "string", "description": "Relative path within the root"},
    "insert_line": {"type": "integer", "description": "Line number to insert after (0 = beginning of file)"},
    "new_str": {"type": "string", "description": "Text to insert"}
  },
  "required": ["root", "path", "insert_line", "new_str"],
  "additionalProperties": false
}`, string(enumJSON))

	return &Tool{
		Name:        "fs_insert",
		Description: fmt.Sprintf("Insert text after a specific line in a file. Use insert_line=0 to insert at the beginning. Available roots: %s", rootDescriptions(roots)),
		Parameters:  json.RawMessage(params),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Root       string `json:"root"`
				Path       string `json:"path"`
				InsertLine int    `json:"insert_line"`
				NewStr     string `json:"new_str"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			root, err := findRoot(roots, p.Root)
			if err != nil {
				return "", err
			}

			resolved, err := resolvePath(root.Path, p.Path)
			if err != nil {
				return "", err
			}

			data, err := os.ReadFile(resolved)
			if err != nil {
				return "", fmt.Errorf("read file: %w", err)
			}

			lines := strings.Split(string(data), "\n")
			if p.InsertLine < 0 || p.InsertLine > len(lines) {
				return "", fmt.Errorf("insert_line %d out of range (0..%d)", p.InsertLine, len(lines))
			}

			newLines := strings.Split(p.NewStr, "\n")
			result := make([]string, 0, len(lines)+len(newLines))
			result = append(result, lines[:p.InsertLine]...)
			result = append(result, newLines...)
			result = append(result, lines[p.InsertLine:]...)

			if err := os.WriteFile(resolved, []byte(strings.Join(result, "\n")), 0644); err != nil {
				return "", fmt.Errorf("write file: %w", err)
			}

			return "Text inserted successfully.", nil
		},
	}
}

// fsToolBuilders maps fs tool names to their builder functions.
var fsToolBuilders = map[string]func([]FilesystemRoot) *Tool{
	"fs_view":        BuildFSViewTool,
	"fs_str_replace": BuildFSStrReplaceTool,
	"fs_create":      BuildFSCreateTool,
	"fs_insert":      BuildFSInsertTool,
}
