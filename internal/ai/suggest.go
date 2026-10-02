package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
)

// MealSuggestion is one reviewable meal-plan suggestion: a recipe for an
// open day/meal-type cell, with the model's reason and the expiring pantry
// items it would consume. Applying it always goes through addMealSlot.
type MealSuggestion struct {
	RecipeID  int64    `json:"recipeId"`
	DayOfWeek int16    `json:"dayOfWeek"`
	MealType  string   `json:"mealType"`
	Reason    string   `json:"reason"`
	Expiring  []string `json:"usesExpiring,omitempty"`
}

const suggestSystemPrompt = `You are LENA's meal-planning assistant. The user message contains a meal plan,
the household pantry (with expiration dates), the recipe catalog, and taste signals.

Task: propose recipes for the plan's EMPTY dayOfWeek/mealType cells.

Rules:
- Only use recipeId values from the "candidates" list — never invent IDs.
- Only target cells not already in "occupied" — never replace a planned meal.
- Prefer recipes whose ingredients appear in the pantry; prioritise items in
  "expiring" so food is eaten before it spoils.
- Prefer recipes the household or user picks often; vary cuisine/meal types.
- "reason" must be one short phrase (under 15 words), e.g. "uses expiring milk".
- "usesExpiring" lists pantry item names from "expiring" the recipe would use; [] if none.
- dayOfWeek is 0 (Sunday) through 6 (Saturday); mealType is the household's
  usual type name (e.g. "Breakfast", "Lunch", "Dinner").
- Reply ONLY with a JSON object: {"suggestions":[{recipeId,dayOfWeek,mealType,reason,usesExpiring}]}`

// suggestRequest carries the assembled household context to the model.
type suggestRequest struct {
	Plan           tools.MealPlanOut `json:"plan"`
	Occupied       []string          `json:"occupied"`
	Candidates     []tools.RecipeRow `json:"candidates"`
	Pantry         json.RawMessage   `json:"pantry"`
	Expiring       json.RawMessage   `json:"expiring"`
	Tastes         json.RawMessage   `json:"tastes"`
	MaxSuggestions int               `json:"maxSuggestions"`
}

type suggestResponse struct {
	Suggestions []MealSuggestion `json:"suggestions"`
}

// mealSuggestionsSchema constrains structured output for suggest-meals —
// served in PreparedRequest.OutputSchema for client-side generation.
const mealSuggestionsSchema = `{"type":"object","required":["suggestions"],` +
	`"properties":{"suggestions":{"type":"array","items":{"type":"object",` +
	`"required":["recipeId","dayOfWeek","mealType","reason"],` +
	`"properties":{"recipeId":{"type":"integer"},"dayOfWeek":{"type":"integer"},` +
	`"mealType":{"type":"string"},"reason":{"type":"string"},` +
	`"usesExpiring":{"type":"array","items":{"type":"string"}}}}},"additionalProperties":false}`

// SuggestMeals runs the server-inference path: assemble context through
// the read-only tool registry, generate with the provider, validate every
// pick — only real recipe IDs and truly open cells survive.
func (s *Service) SuggestMeals(ctx context.Context, userID, householdID, mealPlanID int64, maxSuggestions int) ([]MealSuggestion, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}
	p, err := s.prepareMeals(ctx, scope, mealPlanID, maxSuggestions)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return []MealSuggestion{}, nil
	}
	return runPrepared(ctx, s.provider, p)
}

// prepareMeals assembles plan/pantry/expiry/taste context through the
// read-only tool registry into a prepared request usable by either the
// server provider or a client-side engine. Returns (nil, nil) when there
// are no recipe candidates to pick from.
func (s *Service) prepareMeals(ctx context.Context, scope tools.Scope, mealPlanID int64, maxSuggestions int) (*prepared[[]MealSuggestion], error) {
	if maxSuggestions <= 0 {
		maxSuggestions = 6
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}

	planAny, err := s.reg.Call(ctx, scope, "get_meal_plan", jsonArgs(map[string]any{"mealPlanId": mealPlanID}))
	if err != nil {
		return nil, err
	}
	plan, ok := planAny.(tools.MealPlanOut)
	if !ok {
		return nil, fmt.Errorf("get_meal_plan returned %T", planAny)
	}
	if plan.MealPlanID == 0 {
		return nil, errors.New("meal plan not found")
	}

	candAny, err := s.reg.Call(ctx, scope, "list_recipes", jsonArgs(map[string]any{"limit": 200}))
	if err != nil {
		return nil, err
	}
	candidates, ok := candAny.([]tools.RecipeRow)
	if !ok {
		return nil, fmt.Errorf("list_recipes returned %T", candAny)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	expiring, err := s.callJSON(ctx, scope, "get_expiring_items", map[string]any{"days": 14})
	if err != nil {
		return nil, err
	}
	pantry, err := s.callJSON(ctx, scope, "get_pantry_inventory", map[string]any{"limit": 200})
	if err != nil {
		return nil, err
	}
	tastes, err := s.callJSON(ctx, scope, "get_household_tastes", map[string]any{"limit": 10})
	if err != nil {
		return nil, err
	}

	occupied := map[string]bool{}
	occupiedList := make([]string, 0, len(plan.Slots))
	for _, slot := range plan.Slots {
		if slot.RecipeID == nil {
			continue
		}
		key := cellKey(slot.DayOfWeek, slot.MealType)
		occupied[key] = true
		occupiedList = append(occupiedList, key)
	}

	reqBody, err := json.Marshal(suggestRequest{
		Plan:           plan,
		Occupied:       occupiedList,
		Candidates:     candidates,
		Pantry:         pantry,
		Expiring:       expiring,
		Tastes:         tastes,
		MaxSuggestions: maxSuggestions,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal suggest context: %w", err)
	}

	valid := map[int64]bool{}
	for _, c := range candidates {
		valid[c.ID] = true
	}
	limit := maxSuggestions

	return &prepared[[]MealSuggestion]{
		req: PreparedRequest{
			Prompt:       suggestSystemPrompt,
			Context:      reqBody,
			OutputSchema: json.RawMessage(mealSuggestionsSchema),
		},
		validate: func(content string) ([]MealSuggestion, error) {
			var parsed suggestResponse
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				return nil, err
			}
			return filterSuggestions(parsed.Suggestions, valid, occupied, limit), nil
		},
	}, nil
}

// filterSuggestions drops model output that violates the contract:
// unknown recipe IDs, occupied cells, out-of-range days, empty types,
// duplicates — sorted by day then meal type.
func filterSuggestions(in []MealSuggestion, valid map[int64]bool, occupied map[string]bool, maxCount int) []MealSuggestion {
	seen := map[string]bool{}
	seenRecipes := map[int64]bool{}
	out := make([]MealSuggestion, 0, len(in))
	for _, sg := range in {
		if !valid[sg.RecipeID] || sg.DayOfWeek < 0 || sg.DayOfWeek > 6 {
			continue
		}
		mt := strings.TrimSpace(sg.MealType)
		if mt == "" || len(mt) > 40 || seenRecipes[sg.RecipeID] {
			continue
		}
		key := cellKey(sg.DayOfWeek, mt)
		if occupied[key] || seen[key] {
			continue
		}
		seen[key] = true
		seenRecipes[sg.RecipeID] = true
		sg.MealType = mt
		sg.Reason = truncate(strings.TrimSpace(sg.Reason), 120)
		out = append(out, sg)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DayOfWeek != out[j].DayOfWeek {
			return out[i].DayOfWeek < out[j].DayOfWeek
		}
		return out[i].MealType < out[j].MealType
	})
	if len(out) > maxCount {
		out = out[:maxCount]
	}
	return out
}

func cellKey(day int16, mealType string) string {
	return fmt.Sprintf("%d/%s", day, strings.ToLower(strings.TrimSpace(mealType)))
}

func jsonArgs(v map[string]any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// callJSON invokes a registry tool and returns its result marshalled to
// JSON for prompt embedding.
func (s *Service) callJSON(ctx context.Context, scope tools.Scope, name string, args map[string]any) (json.RawMessage, error) {
	res, err := s.reg.Call(ctx, scope, name, jsonArgs(args))
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("marshal %s result: %w", name, err)
	}
	return b, nil
}
