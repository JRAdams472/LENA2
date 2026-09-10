// Package ollamaclient is a thin HTTP client for the local Ollama API used
// during recipe OCR import.
package ollamaclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the Ollama API.
type Client struct {
	baseURL string
	client  *http.Client
	model   string
	temp    float64
	numCtx  int
}

// New creates a Client. baseURL is the root of the Ollama API, e.g.
// "http://ollama:11434".
func New(baseURL, model string, temp float64, numCtx int) *Client {
	if baseURL == "" {
		baseURL = "http://ollama:11434"
	}
	if model == "" {
		model = "qwen2.5:7b-instruct"
	}
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Minute},
		model:   model,
		temp:    temp,
		numCtx:  numCtx,
	}
}

// NewWithTimeout creates a Client with an explicit HTTP timeout.
func NewWithTimeout(baseURL, model string, temp float64, numCtx int, timeout time.Duration) *Client {
	c := New(baseURL, model, temp, numCtx)
	c.client.Timeout = timeout
	return c
}

// Message is one turn in an Ollama chat.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the payload for POST /api/chat.
type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Format   string                 `json:"format"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Stream   bool                   `json:"stream"`
}

// ChatResponse is the non-streaming response from /api/chat.
type ChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// Chat sends a single-turn chat and returns the assistant's content.
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	reqBody := ChatRequest{
		Model:  c.model,
		Format: "json",
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Options: map[string]interface{}{
			"temperature": c.temp,
			"num_ctx":     c.numCtx,
		},
		Stream: false,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	return chatResp.Message.Content, nil
}
