package tool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
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

			// Forward host environment variables configured for this agent.
			if names := GetHostEnvVars(ctx); len(names) > 0 {
				for _, name := range names {
					if val, ok := os.LookupEnv(name); ok {
						cmd.Env = append(cmd.Env, name+"="+val)
					}
				}
			}

			onOutput := GetOutputCallback(ctx)

			// Use streaming if an output callback is available.
			if onOutput != nil {
				return bashRunStreaming(cmd, onOutput)
			}
			return bashRunBuffered(cmd)
		},
	}
}

// bashRunBuffered runs the command and returns the full output at once.
func bashRunBuffered(cmd *sprites.Cmd) (string, error) {
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

	return formatBashOutput(stdout.String(), stderr.String(), exitCode), nil
}

// bashRunStreaming runs the command, streaming stdout line-by-line via the callback,
// and returns the complete output when done.
func bashRunStreaming(cmd *sprites.Cmd, onOutput OutputCallback) (string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Bash execution failed: %v", err)
		return "", fmt.Errorf("execution failed: %w", err)
	}

	// Read stdout and stderr concurrently.
	var stdoutBuf, stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			stdoutBuf.WriteString(line)
			onOutput(line)
		}
	}()

	go func() {
		defer wg.Done()
		// Buffer stderr without streaming (it's included in final output).
		_, _ = io.Copy(&stderrBuf, stderrPipe)
	}()

	// Wait for command to finish first — this closes the pipe write ends,
	// which causes the reader goroutines above to see EOF and exit.
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*sprites.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			log.Printf("Bash execution failed: %v", err)
			return "", fmt.Errorf("execution failed: %w", err)
		}
	}

	// Now wait for goroutines to finish draining any remaining pipe data.
	wg.Wait()

	return formatBashOutput(stdoutBuf.String(), stderrBuf.String(), exitCode), nil
}

// formatBashOutput combines stdout, stderr, and exit code into the final tool output string.
func formatBashOutput(stdout, stderr string, exitCode int) string {
	var out strings.Builder
	if stdout != "" {
		out.WriteString(stdout)
		if !strings.HasSuffix(stdout, "\n") {
			out.WriteString("\n")
		}
	}
	if stderr != "" {
		out.WriteString("stderr:\n")
		out.WriteString(stderr)
		if !strings.HasSuffix(stderr, "\n") {
			out.WriteString("\n")
		}
	}
	if exitCode != 0 {
		out.WriteString(fmt.Sprintf("exit_code: %d", exitCode))
	}
	return strings.TrimSpace(out.String())
}
