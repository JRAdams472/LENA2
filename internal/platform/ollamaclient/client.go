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
	// ToolCalls is populated on assistant messages when the model requests
	// tool execution. Tool-result turns carry Name and use Content for the
	// tool's output.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Name      string     `json:"name,omitempty"`
}

// ToolCall is a model-requested function invocation on an assistant message.
type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction carries the invoked tool's name and parsed arguments.
type ToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolSpec describes a tool the model may call.
type ToolSpec struct {
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction holds the tool name, description, and JSON Schema params.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest is the payload for POST /api/chat.
type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Format   interface{}            `json:"format,omitempty"`
	Tools    []ToolSpec             `json:"tools,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Stream   bool                   `json:"stream"`
}

// ChatResponse is the non-streaming response from /api/chat.
type ChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// EmbedRequest is the payload for POST /api/embed.
type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbedResponse is the response from /api/embed — one embedding per input.
type EmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

// Embed embeds a batch of texts with the given model (embedding models are
// separate from the chat model, so it is a parameter rather than a client
// field).
func (c *Client) Embed(ctx context.Context, model string, inputs []string) ([][]float64, error) {
	data, err := json.Marshal(EmbedRequest{Model: model, Input: inputs})
	if err != nil {
		return nil, fmt.Errorf("marshal ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed returned status %d: %s", resp.StatusCode, string(body))
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}
	if len(embedResp.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("ollama embed returned %d embeddings for %d inputs", len(embedResp.Embeddings), len(inputs))
	}
	return embedResp.Embeddings, nil
}

// Chat sends a single-turn chat and returns the assistant's content.
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	resp, err := c.ChatRaw(ctx, ChatRequest{
		Format: "json",
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

// ChatRaw sends a fully-specified chat request (multi-turn, tools, optional
// JSON mode) and returns the raw response so callers can inspect tool calls.
func (c *Client) ChatRaw(ctx context.Context, reqBody ChatRequest) (ChatResponse, error) {
	reqBody.Model = c.model
	reqBody.Stream = false
	if reqBody.Options == nil {
		reqBody.Options = map[string]interface{}{}
	}
	if _, ok := reqBody.Options["temperature"]; !ok {
		reqBody.Options["temperature"] = c.temp
	}
	if _, ok := reqBody.Options["num_ctx"]; !ok {
		reqBody.Options["num_ctx"] = c.numCtx
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("ollama request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return ChatResponse{}, fmt.Errorf("decode ollama response: %w", err)
	}
	return chatResp, nil
}
