package ollamaclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req ChatRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "qwen2.5:7b-instruct", req.Model)
		assert.Equal(t, "json", req.Format)

		resp := ChatResponse{
			Model:   req.Model,
			Done:    true,
			Message: Message{Role: "assistant", Content: `{"name":"Pancakes"}`},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client := New(server.URL, "qwen2.5:7b-instruct", 0.1, 8192)
	content, err := client.Chat(context.Background(), "system prompt", "user prompt")
	require.NoError(t, err)
	assert.Equal(t, `{"name":"Pancakes"}`, content)
}

func TestChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := New(server.URL, "missing", 0.1, 8192)
	_, err := client.Chat(context.Background(), "system prompt", "user prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}
