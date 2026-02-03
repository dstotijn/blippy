package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NotificationChannel represents a notification channel for tool building.
type NotificationChannel struct {
	ID          string
	Name        string
	Description string
	JSONSchema  string
	Type        string
	Config      string
}

// BuildNotificationTool creates a tool definition for a notification channel.
func BuildNotificationTool(channel NotificationChannel) *Tool {
	// Use provided schema or default to accepting any JSON
	schema := channel.JSONSchema
	if schema == "" {
		schema = `{"type": "object", "additionalProperties": true}`
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Send a notification to the %s channel", channel.Name)
	}

	return &Tool{
		Name:        "notify:" + channel.Name,
		Description: description,
		Parameters:  json.RawMessage(schema),
		Handler: func(ctx context.Context, argsJSON json.RawMessage) (string, error) {
			switch channel.Type {
			case "http_request":
				return executeNotificationHTTPRequest(ctx, channel.Config, argsJSON)
			default:
				return fmt.Sprintf("Unknown channel type: %s", channel.Type), nil
			}
		},
	}
}

func executeNotificationHTTPRequest(ctx context.Context, configJSON string, payload json.RawMessage) (string, error) {
	var cfg struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}

	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to send: %s", err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("Failed with status %d: %s", resp.StatusCode, string(body)), nil
	}

	return "Notification sent successfully", nil
}
