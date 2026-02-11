package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFSCreateNestedDirs(t *testing.T) {
	dir := t.TempDir()
	root := FilesystemRoot{Name: "test", Path: dir, Description: "test"}
	roots := []FilesystemRoot{root}
	ctx := context.Background()

	tool := BuildFSCreateTool(roots)
	args, _ := json.Marshal(map[string]string{
		"root":      "test",
		"path":      "subdir/nested/hello.txt",
		"file_text": "hello world",
	})
	result, err := tool.Handler(ctx, args)
	if err != nil {
		t.Fatalf("fs_create failed: %v", err)
	}
	if result != "File created successfully." {
		t.Fatalf("unexpected result: %s", result)
	}

	data, err := os.ReadFile(filepath.Join(dir, "subdir/nested/hello.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestFSCreateTraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	root := FilesystemRoot{Name: "test", Path: dir, Description: "test"}
	roots := []FilesystemRoot{root}
	ctx := context.Background()

	tool := BuildFSCreateTool(roots)
	args, _ := json.Marshal(map[string]string{
		"root":      "test",
		"path":      "../../../etc/evil.txt",
		"file_text": "nope",
	})
	_, err := tool.Handler(ctx, args)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestFSCreateExistingFileBlocked(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("already here"), 0644)

	root := FilesystemRoot{Name: "test", Path: dir, Description: "test"}
	roots := []FilesystemRoot{root}
	ctx := context.Background()

	tool := BuildFSCreateTool(roots)
	args, _ := json.Marshal(map[string]string{
		"root":      "test",
		"path":      "exists.txt",
		"file_text": "overwrite",
	})
	_, err := tool.Handler(ctx, args)
	if err == nil {
		t.Fatal("expected error for existing file, got nil")
	}
}

func TestFSViewDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	root := FilesystemRoot{Name: "test", Path: dir, Description: "test"}
	roots := []FilesystemRoot{root}
	ctx := context.Background()

	tool := BuildFSViewTool(roots)
	args, _ := json.Marshal(map[string]string{
		"root": "test",
		"path": ".",
	})
	result, err := tool.Handler(ctx, args)
	if err != nil {
		t.Fatalf("fs_view failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestFSStrReplace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world"), 0644)

	root := FilesystemRoot{Name: "test", Path: dir, Description: "test"}
	roots := []FilesystemRoot{root}
	ctx := context.Background()

	tool := BuildFSStrReplaceTool(roots)
	args, _ := json.Marshal(map[string]string{
		"root":    "test",
		"path":    "test.txt",
		"old_str": "world",
		"new_str": "there",
	})
	_, err := tool.Handler(ctx, args)
	if err != nil {
		t.Fatalf("fs_str_replace failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "test.txt"))
	if string(data) != "hello there" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestFSInsert(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("line1\nline2\nline3"), 0644)

	root := FilesystemRoot{Name: "test", Path: dir, Description: "test"}
	roots := []FilesystemRoot{root}
	ctx := context.Background()

	tool := BuildFSInsertTool(roots)
	args, _ := json.Marshal(map[string]any{
		"root":        "test",
		"path":        "test.txt",
		"insert_line": 1,
		"new_str":     "inserted",
	})
	_, err := tool.Handler(ctx, args)
	if err != nil {
		t.Fatalf("fs_insert failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "test.txt"))
	expected := "line1\ninserted\nline2\nline3"
	if string(data) != expected {
		t.Fatalf("unexpected content: %q, want %q", string(data), expected)
	}
}
