package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/ollamaclient"
)

func ollamaTestServer(t *testing.T, handler func(req ollamaclient.ChatRequest) ollamaclient.ChatResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req ollamaclient.ChatRequest
		require.NoError(t, json.Unmarshal(body, &req))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(handler(req)))
	}))
}

func TestOllamaProviderJSONMode(t *testing.T) {
	server := ollamaTestServer(t, func(req ollamaclient.ChatRequest) ollamaclient.ChatResponse {
		assert.Equal(t, "json", req.Format)
		assert.Empty(t, req.Tools)
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		return ollamaclient.ChatResponse{
			Done:    true,
			Message: ollamaclient.Message{Role: "assistant", Content: `{"ok":true}`},
		}
	})
	defer server.Close()

	p := NewOllamaProvider(Params{URL: server.URL, Model: "m", Temperature: 0.2, NumCtx: 4096})
	resp, err := p.Chat(context.Background(), Request{
		JSONMode: true,
		Messages: []Message{
			{Role: RoleSystem, Content: "sys"},
			{Role: RoleUser, Content: "usr"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, resp.Message.Content)
	assert.Equal(t, "ollama", p.Name())
}

func TestOllamaProviderJSONSchema(t *testing.T) {
	schema := map[string]any{"type": "object", "required": []string{"a"}}
	server := ollamaTestServer(t, func(req ollamaclient.ChatRequest) ollamaclient.ChatResponse {
		assert.Equal(t, map[string]any{"type": "object", "required": []any{"a"}}, req.Format)
		return ollamaclient.ChatResponse{
			Done:    true,
			Message: ollamaclient.Message{Role: "assistant", Content: `{"a":1}`},
		}
	})
	defer server.Close()

	p := NewOllamaProvider(Params{URL: server.URL, Model: "m"})
	_, err := p.Chat(context.Background(), Request{
		JSONMode:   true,
		JSONSchema: schema,
		Messages:   []Message{{Role: RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
}

func TestOllamaProviderToolCalling(t *testing.T) {
	server := ollamaTestServer(t, func(req ollamaclient.ChatRequest) ollamaclient.ChatResponse {
		// format must not force plain JSON while tools are advertised.
		assert.Empty(t, req.Format)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_pantry", req.Tools[0].Function.Name)
		return ollamaclient.ChatResponse{
			Done: true,
			Message: ollamaclient.Message{
				Role: "assistant",
				ToolCalls: []ollamaclient.ToolCall{
					{Function: ollamaclient.ToolCallFunction{
						Name:      "get_pantry",
						Arguments: json.RawMessage(`{"limit":5}`),
					}},
				},
			},
		}
	})
	defer server.Close()

	p := NewOllamaProvider(Params{URL: server.URL, Model: "m"})
	resp, err := p.Chat(context.Background(), Request{
		JSONMode: true, // ignored while tools are advertised
		Messages: []Message{{Role: RoleUser, Content: "what's in stock"}},
		Tools: []ToolSpec{{
			Name:        "get_pantry",
			Description: "List pantry items",
			Parameters:  map[string]any{"type": "object"},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Message.ToolCalls, 1)
	call := resp.Message.ToolCalls[0]
	assert.Equal(t, "get_pantry", call.Name)
	assert.Equal(t, "call-0", call.ID)
	assert.JSONEq(t, `{"limit":5}`, string(call.Arguments))
}

func TestOllamaProviderToolResultRoundTrip(t *testing.T) {
	server := ollamaTestServer(t, func(req ollamaclient.ChatRequest) ollamaclient.ChatResponse {
		require.Len(t, req.Messages, 3)
		toolMsg := req.Messages[2]
		assert.Equal(t, "tool", toolMsg.Role)
		assert.Equal(t, "get_pantry", toolMsg.Name)
		assert.Equal(t, `[{"name":"flour"}]`, toolMsg.Content)
		return ollamaclient.ChatResponse{
			Done:    true,
			Message: ollamaclient.Message{Role: "assistant", Content: `{"answer":"flour"}`},
		}
	})
	defer server.Close()

	p := NewOllamaProvider(Params{URL: server.URL, Model: "m"})
	call := ToolCall{ID: "call-0", Name: "get_pantry", Arguments: json.RawMessage(`{}`)}
	resp, err := p.Chat(context.Background(), Request{
		Messages: []Message{
			{Role: RoleUser, Content: "stock?"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{call}},
			ToolResult(call, `[{"name":"flour"}]`),
		},
		Tools: []ToolSpec{{Name: "get_pantry", Parameters: map[string]any{}}},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"answer":"flour"}`, resp.Message.Content)
}

func TestOllamaProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()
	p := NewOllamaProvider(Params{URL: server.URL})
	_, err := p.Chat(context.Background(), Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestNewProvider(t *testing.T) {
	p, err := NewProvider(Params{})
	require.NoError(t, err)
	assert.Nil(t, p)

	p, err = NewProvider(Params{Provider: "mock"})
	require.NoError(t, err)
	assert.Equal(t, "mock", p.Name())

	p, err = NewProvider(Params{Provider: "ollama", URL: "http://localhost:1"})
	require.NoError(t, err)
	assert.Equal(t, "ollama", p.Name())

	_, err = NewProvider(Params{Provider: "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestMockProviderScriptedSequence(t *testing.T) {
	m := NewMockProvider()
	m.EnqueueToolCalls(ToolCall{ID: "call-0", Name: "get_pantry", Arguments: json.RawMessage(`{}`)})
	m.EnqueueText(`{"answer":"done"}`)

	r1, err := m.Chat(context.Background(), Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	require.NoError(t, err)
	require.Len(t, r1.Message.ToolCalls, 1)
	assert.Equal(t, "get_pantry", r1.Message.ToolCalls[0].Name)

	r2, err := m.Chat(context.Background(), Request{})
	require.NoError(t, err)
	assert.Equal(t, `{"answer":"done"}`, r2.Message.Content)

	assert.Equal(t, 2, m.CallCount())
	assert.Len(t, m.Requests, 2)

	_, err = m.Chat(context.Background(), Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no scripted response")
}

func TestMockProviderErrorAndHandler(t *testing.T) {
	m := NewMockProvider()
	m.EnqueueError(errors.New("provider down"))
	_, err := m.Chat(context.Background(), Request{})
	require.Error(t, err)

	m.Handler = func(req Request) (Response, error) {
		return Response{Message: Message{Role: RoleAssistant, Content: "handled " + req.Messages[0].Content}}, nil
	}
	resp, err := m.Chat(context.Background(), Request{Messages: []Message{{Role: RoleUser, Content: "q"}}})
	require.NoError(t, err)
	assert.Equal(t, "handled q", resp.Message.Content)
}
