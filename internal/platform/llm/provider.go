// Package llm defines the provider-agnostic LLM interface used by the AI
// assistant features. Implementations (Ollama, mock, future commercial
// providers) adapt a vendor chat API to a common request/response shape so
// swapping providers is a config change, not a code change.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Role values for Message.Role.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is one turn of the conversation. Assistant messages may carry
// ToolCalls; tool-result messages use Role=tool with Name and ToolCallID set
// and Content holding the tool's output.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

// ToolCall is a model-requested tool invocation.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// ToolSpec describes a tool the model may invoke.
type ToolSpec struct {
	Name        string
	Description string
	// Parameters is a JSON Schema object describing the tool's arguments.
	Parameters map[string]any
}

// Request is one provider call.
type Request struct {
	Messages []Message
	// Tools advertised for this call. When non-empty the model may answer
	// with tool calls instead of content.
	Tools []ToolSpec
	// JSONMode asks the provider for strictly-JSON output. Providers that
	// cannot combine JSON mode with tools may ignore it when Tools is set.
	JSONMode bool
}

// Response is one provider reply: either assistant content, tool calls, or
// both (a model may narrate while calling tools).
type Response struct {
	Message Message
}

// Provider is the swappable LLM backend.
type Provider interface {
	Chat(ctx context.Context, req Request) (Response, error)
	// Name identifies the implementation ("ollama", "mock", ...).
	Name() string
}

// Params configures NewProvider.
type Params struct {
	// Provider selects the implementation: "ollama" or "mock".
	// Empty disables AI features (NewProvider returns nil, nil).
	Provider string
	// URL is the provider base URL ("ollama" → e.g. http://ollama:11434).
	URL string
	// Model is the model tag. Empty uses the implementation's default.
	Model string
	// Temperature is the sampling temperature.
	Temperature float64
	// NumCtx is the context window in tokens.
	NumCtx int
	// Timeout caps a single Chat call. Zero uses the implementation default.
	Timeout time.Duration
}

// NewProvider builds the configured Provider, or (nil, nil) when Params.
// Provider is empty — AI features are off by default. Unknown provider names
// are a config error.
func NewProvider(p Params) (Provider, error) {
	switch p.Provider {
	case "":
		return nil, nil
	case "ollama":
		return NewOllamaProvider(p), nil
	case "mock":
		return NewMockProvider(), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (want ollama|mock)", p.Provider)
	}
}
