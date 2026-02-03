package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	sprites "github.com/superfly/sprites-go"
)

// BashArgs defines the arguments for the bash tool
type BashArgs struct {
	Command string `json:"command"`
}

// NewBashTool creates the bash tool with a Sprites client
func NewBashTool(apiKey string) *Tool {
	client := sprites.New(apiKey)

	// Track which sprites we've already created
	var (
		createdSprites = make(map[string]bool)
		mu             sync.Mutex
	)

	return &Tool{
		Name:        "bash",
		Description: "Run a bash command in a sandboxed environment. Use for file operations, system commands, installing packages, running Python (python3), JavaScript (node), and general shell tasks.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The bash command to run"
				}
			},
			"required": ["command"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var a BashArgs
			if err := json.Unmarshal(args, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			if a.Command == "" {
				return "", fmt.Errorf("command is required")
			}

			// Get agent ID from context for sprite naming (one sprite per agent)
			agentID := GetAgentID(ctx)
			if agentID == "" {
				return "", fmt.Errorf("agent ID not found in context")
			}

			spriteName := "blippy-" + agentID

			// Ensure sprite exists (create if needed)
			mu.Lock()
			needsCreate := !createdSprites[spriteName]
			mu.Unlock()

			if needsCreate {
				_, err := client.GetSprite(ctx, spriteName)
				if err != nil {
					_, err = client.CreateSprite(ctx, spriteName, nil)
					if err != nil && !strings.Contains(err.Error(), "already exists") {
						return "", fmt.Errorf("create sprite: %w", err)
					}
				}
				mu.Lock()
				createdSprites[spriteName] = true
				mu.Unlock()
			}

			// Execute command
			sprite := client.Sprite(spriteName)
			cmd := sprite.CommandContext(ctx, "bash", "-c", a.Command)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*sprites.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					log.Printf("Bash execution failed: %v", err)
					return "", fmt.Errorf("execution failed: %w", err)
				}
			}

			// Format output
			var out strings.Builder
			if stdout.Len() > 0 {
				out.WriteString(stdout.String())
				if !strings.HasSuffix(stdout.String(), "\n") {
					out.WriteString("\n")
				}
			}
			if stderr.Len() > 0 {
				out.WriteString("stderr:\n")
				out.WriteString(stderr.String())
				if !strings.HasSuffix(stderr.String(), "\n") {
					out.WriteString("\n")
				}
			}
			if exitCode != 0 {
				out.WriteString(fmt.Sprintf("exit_code: %d", exitCode))
			}

			return strings.TrimSpace(out.String()), nil
		},
	}
}
