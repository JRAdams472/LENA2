package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// ErrUnknownRequest is returned for unrecognized prepared-request names.
var ErrUnknownRequest = errors.New("unknown assistant request")

// ErrBadParams wraps malformed or incomplete paramsJson payloads.
var ErrBadParams = errors.New("invalid request params")

// MaxPrepareParamsBytes bounds the paramsJson argument to
// PrepareRequest — it only ever carries an id or two plus a limit.
const MaxPrepareParamsBytes = 1024

// PreparedRequest is a fully assembled one-shot AI request. Client-side
// inference agents run Prompt+Context through a local engine and shape the
// reply to OutputSchema; the server path uses the same payload plus the
// typed validator that produced it (see prepared[T]).
type PreparedRequest struct {
	// Prompt is the system prompt for the request.
	Prompt string
	// Context is the assembled user message — household data gathered
	// through the read-only tool registry, as a JSON string.
	Context json.RawMessage
	// OutputSchema is a JSON Schema describing the required model output,
	// used by clients for constrained decoding and response validation.
	OutputSchema json.RawMessage
}

// prepared couples a serializable request with the typed validator the
// server path runs on model output.
type prepared[T any] struct {
	req      PreparedRequest
	validate func(content string) (T, error)
}

// runPrepared executes a prepared request against the provider with one
// malformed-output retry — the same contract every Suggest* feature uses.
func runPrepared[T any](ctx context.Context, prov llm.Provider, p *prepared[T]) (T, error) {
	var zero T
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: p.req.Prompt},
		{Role: llm.RoleUser, Content: string(p.req.Context)},
	}
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := prov.Chat(ctx, llm.Request{Messages: msgs, JSONMode: true})
		if err != nil {
			return zero, fmt.Errorf("suggest provider: %w", err)
		}
		out, err := p.validate(resp.Message.Content)
		if err != nil {
			// One retry, telling the model what was wrong.
			msgs = append(msgs,
				llm.Message{Role: llm.RoleAssistant, Content: resp.Message.Content},
				llm.Message{Role: llm.RoleUser, Content: "That was not valid JSON matching the required schema. Reply ONLY with the JSON object."},
			)
			continue
		}
		return out, nil
	}
	return zero, errors.New("assistant returned malformed suggestions")
}

// PrepareRequest assembles a one-shot AI request without running the
// model — the same context assembly and validation contract as the
// matching Suggest* method, for client-side inference. Returns (nil, nil)
// when nothing needs generating (no candidates, clean timeline, ...).
func (s *Service) PrepareRequest(ctx context.Context, userID, householdID int64, name string, params json.RawMessage) (*PreparedRequest, error) {
	if s == nil || s.reg == nil {
		return nil, ErrUnavailable
	}
	if len(params) > MaxPrepareParamsBytes {
		return nil, fmt.Errorf("%w: paramsJson exceeds %d bytes", ErrBadParams, MaxPrepareParamsBytes)
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}

	switch name {
	case "suggest-meals":
		var p struct {
			MealPlanID     int64 `json:"mealPlanId"`
			MaxSuggestions int   `json:"maxSuggestions"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if p.MealPlanID <= 0 {
			return nil, fmt.Errorf("%w: mealPlanId is required", ErrBadParams)
		}
		pr, err := s.prepareMeals(ctx, scope, p.MealPlanID, p.MaxSuggestions)
		return requestOf(pr, err)

	case "suggest-event-fixes":
		var p struct {
			FoodEventID    int64 `json:"foodEventId"`
			MaxSuggestions int   `json:"maxSuggestions"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if p.FoodEventID <= 0 {
			return nil, fmt.Errorf("%w: foodEventId is required", ErrBadParams)
		}
		pr, err := s.prepareEventFixes(ctx, scope, p.FoodEventID, p.MaxSuggestions)
		return requestOf(pr, err)

	case "suggest-pairings":
		var p struct {
			RecipeID       int64 `json:"recipeId"`
			MaxSuggestions int   `json:"maxSuggestions"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if p.RecipeID <= 0 {
			return nil, fmt.Errorf("%w: recipeId is required", ErrBadParams)
		}
		pr, err := s.preparePairings(ctx, scope, p.RecipeID, p.MaxSuggestions)
		return requestOf(pr, err)

	case "suggest-cocktails":
		var p struct {
			MaxSuggestions int  `json:"maxSuggestions"`
			InStockOnly    bool `json:"inStockOnly"`
		}
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		pr, err := s.prepareCocktails(ctx, scope, p.MaxSuggestions, p.InStockOnly)
		return requestOf(pr, err)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownRequest, name)
	}
}

// requestOf unwraps a prepare step: propagate errors, map a nil request
// (nothing to generate) to nil.
func requestOf[T any](pr *prepared[T], err error) (*PreparedRequest, error) {
	if err != nil || pr == nil {
		return nil, err
	}
	return &pr.req, nil
}

// decodeParams parses the paramsJson argument; empty input is an empty
// object so optional-parameter features need no args.
func decodeParams(params json.RawMessage, out any) error {
	if len(params) == 0 {
		return nil
	}
	if err := json.Unmarshal(params, out); err != nil {
		return fmt.Errorf("%w: %w", ErrBadParams, err)
	}
	return nil
}
