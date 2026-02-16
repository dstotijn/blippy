package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://openrouter.ai/api/v1"

type Client struct {
	apiKey     string
	httpClient *http.Client

	modelsMu      sync.Mutex
	modelsCache   []Model
	modelsFetched time.Time
}

// Model represents an available model from OpenRouter.
type Model struct {
	ID                string
	Name              string
	PromptPricing     string
	CompletionPricing string
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

type ResponseRequest struct {
	Model              string           `json:"model"`
	Input              []Input          `json:"input"`
	Instructions       string           `json:"instructions,omitempty"`
	PreviousResponseID string           `json:"previous_response_id,omitempty"`
	Stream             bool             `json:"stream,omitempty"`
	Tools              []map[string]any `json:"tools,omitempty"`
}

type Input struct {
	Type        string        `json:"type,omitempty"`        // "message", "function_call", "function_call_output"
	Role        string        `json:"role,omitempty"`        // for message type
	Content     []ContentPart `json:"content,omitempty"`     // for message type (structured)
	ID          string        `json:"id,omitempty"`          // for assistant messages and function_call_output
	Status      string        `json:"status,omitempty"`      // for assistant messages ("completed")
	Annotations []any         `json:"annotations,omitempty"` // for assistant messages (empty array)
	CallID      string        `json:"call_id,omitempty"`     // for function_call and function_call_output
	Name        string        `json:"name,omitempty"`        // for function_call
	Arguments   string        `json:"arguments,omitempty"`   // for function_call
	Output      string        `json:"output,omitempty"`      // for function_call_output
}

// ContentPart represents a content element in a message
type ContentPart struct {
	Type string `json:"type"` // "input_text" or "output_text"
	Text string `json:"text"`
}

type Response struct {
	ID     string         `json:"id"`
	Output []OutputItem   `json:"output"`
	Error  *ResponseError `json:"error,omitempty"`
}

type OutputItem struct {
	Type      string        `json:"type"`                // "message", "function_call"
	Content   []ContentPart `json:"content,omitempty"`   // for message type
	ID        string        `json:"id,omitempty"`        // for function_call
	CallID    string        `json:"call_id,omitempty"`   // for function_call
	Name      string        `json:"name,omitempty"`      // function name
	Arguments string        `json:"arguments,omitempty"` // function args as JSON string
}

type ResponseError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type StreamEvent struct {
	Type           string    `json:"type"`
	Delta          string    `json:"delta,omitempty"`
	Response       *Response `json:"response,omitempty"`
	ItemType       string    `json:"item_type,omitempty"`       // "function_call" for tool calls
	Name           string    `json:"name,omitempty"`            // function name
	CallID         string    `json:"call_id,omitempty"`         // function call ID
	ArgumentsDelta string    `json:"arguments_delta,omitempty"` // streaming args
}

func (c *Client) CreateResponse(ctx context.Context, req *ResponseRequest) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &response, nil
}

func (c *Client) CreateResponseStream(ctx context.Context, req *ResponseRequest) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent)
	errs := make(chan error, 1)

	req.Stream = true

	go func() {
		defer close(events)
		defer close(errs)

		body, err := json.Marshal(req)
		if err != nil {
			errs <- fmt.Errorf("marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/responses", bytes.NewReader(body))
		if err != nil {
			errs <- fmt.Errorf("create request: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errs <- fmt.Errorf("do request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errs <- fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var event StreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue // skip malformed events
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("scan: %w", err)
		}
	}()

	return events, errs
}

// ListModels returns the list of available models from OpenRouter, cached for 1 hour.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	c.modelsMu.Lock()
	defer c.modelsMu.Unlock()

	if c.modelsCache != nil && time.Since(c.modelsFetched) < time.Hour {
		return c.modelsCache, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]Model, len(result.Data))
	for i, m := range result.Data {
		models[i] = Model{
			ID:                m.ID,
			Name:              m.Name,
			PromptPricing:     m.Pricing.Prompt,
			CompletionPricing: m.Pricing.Completion,
		}
	}

	c.modelsCache = models
	c.modelsFetched = time.Now()

	return models, nil
}

// GenerateTitle generates a brief conversation title from the first exchange.
func (c *Client) GenerateTitle(ctx context.Context, model, userMessage, assistantResponse string) (string, error) {
	prompt := fmt.Sprintf(`Generate a brief title (3-6 words) for this conversation:

User: %s
Assistant: %s

Reply with only the title, no quotes or explanation.`, userMessage, assistantResponse)

	req := &ResponseRequest{
		Model: model,
		Input: []Input{
			{
				Type: "message",
				Role: "user",
				Content: []ContentPart{
					{Type: "input_text", Text: prompt},
				},
			},
		},
	}

	resp, err := c.CreateResponse(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create response: %w", err)
	}

	for _, item := range resp.Output {
		if item.Type == "message" && len(item.Content) > 0 {
			return strings.TrimSpace(item.Content[0].Text), nil
		}
	}

	return "", fmt.Errorf("no title in response")
}
