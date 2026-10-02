package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// promptCatalog is the single source of truth for assistant system
// prompts. Client-side inference agents fetch them through
// assistantPrompt so local and server runs stay identical. Names are a
// closed set — the resolver rejects anything unlisted.
var promptCatalog = map[string]string{
	"ask":                 assistantSystemPrompt,
	"suggest-meals":       suggestSystemPrompt,
	"suggest-event-fixes": eventFixSystemPrompt,
	"suggest-pairings":    pairingSystemPrompt,
	"suggest-cocktails":   cocktailSystemPrompt,
}

// ToolSpecs returns the registered read-only tool catalog for client-side
// agents. Available even when no LLM provider is configured — the tools
// themselves are just household-scoped reads.
func (s *Service) ToolSpecs() []llm.ToolSpec {
	if s == nil || s.reg == nil {
		return nil
	}
	return s.reg.Specs()
}

// CallTool executes one read-only tool under the caller's household scope
// for a client-side inference agent. Results are JSON strings truncated to
// the configured tool-result cap. Every failure — unknown tool, invalid
// arguments, handler error — comes back as an {"error":...} payload so the
// client agent can feed it to the model exactly like the server loop does;
// only marshalling the result itself can produce a Go error.
func (s *Service) CallTool(ctx context.Context, userID, householdID int64, name string, args json.RawMessage) (string, error) {
	if s == nil || s.reg == nil {
		return "", ErrUnavailable
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}
	result, err := s.reg.Call(ctx, scope, name, args)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal tool result: %w", err)
	}
	return truncate(string(data), s.cfg.ToolResultMaxBytes), nil
}

// Prompt returns the system prompt for a named assistant flow.
// The second return reports whether the name exists in the catalog.
func (s *Service) Prompt(name string) (string, bool) {
	p, ok := promptCatalog[name]
	return p, ok
}
