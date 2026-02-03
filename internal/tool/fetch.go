package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchArgs defines the arguments for the fetch tool
type FetchArgs struct {
	URL string `json:"url"`
}

// NewFetchTool creates the URL fetch tool
func NewFetchTool() *Tool {
	return &Tool{
		Name:        "fetch_url",
		Description: "Fetch the content of a URL. Returns the text content of the page. Use this to read web pages, documentation, or API responses.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "The URL to fetch"
				}
			},
			"required": ["url"]
		}`),
		Handler: fetchHandler,
	}
}

func fetchHandler(ctx context.Context, args json.RawMessage) (string, error) {
	var a FetchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if a.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	// Validate URL scheme
	if !strings.HasPrefix(a.URL, "http://") && !strings.HasPrefix(a.URL, "https://") {
		return "", fmt.Errorf("url must start with http:// or https://")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Blippy/1.0")
	req.Header.Set("Accept", "text/html,text/plain,application/json,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit response size to 500KB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return string(body), nil
}
