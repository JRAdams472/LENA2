// Package tools is the MCP-shaped tool layer for the AI assistant: a
// registry of named, JSON-schema-described, read-only data functions the
// model invokes through the provider's tool-calling protocol. Dispatch is
// in-process — the registry is structured so a real MCP transport could
// front the same entries later.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// Scope is the authenticated caller's data boundary. It is built from the
// request context by the assistant service — never from tool arguments, so
// a prompt or compromised model cannot redirect a tool at another
// household's data.
type Scope struct {
	UserID      int64
	HouseholdID int64
}

// Handler executes one tool for the given scope. The returned value is
// marshaled to JSON and fed back to the model as the tool result.
type Handler func(ctx context.Context, scope Scope, args json.RawMessage) (any, error)

// ErrUnknownTool is returned when the model invokes an unregistered name.
var ErrUnknownTool = errors.New("unknown tool")

type entry struct {
	spec    llm.ToolSpec
	handler Handler
}

// Registry is the tool catalog. Tools are registered at startup; lookups
// and calls are safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]entry
	order   []string
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{entries: map[string]entry{}}
}

// Register adds a tool. Duplicate names panic — registration is startup
// wiring, and a collision is a programmer error, not runtime data.
func (r *Registry) Register(spec llm.ToolSpec, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.entries[spec.Name]; dup {
		panic("tools: duplicate registration " + spec.Name)
	}
	r.entries[spec.Name] = entry{spec: spec, handler: h}
	r.order = append(r.order, spec.Name)
}

// Specs returns every registered tool in registration order for
// advertising to the provider.
func (r *Registry) Specs() []llm.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]llm.ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.entries[name].spec)
	}
	return out
}

// Call dispatches one model-requested tool call. Unknown names fail with
// ErrUnknownTool so the loop can tell the model the tool doesn't exist
// rather than crashing the request.
func (r *Registry) Call(ctx context.Context, scope Scope, name string, args json.RawMessage) (any, error) {
	r.mu.RLock()
	e, ok := r.entries[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	return e.handler(ctx, scope, args)
}
