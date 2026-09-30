package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// PairingSuggestion is one sommelier pick for a recipe. When BottleID is
// set the pick is a bottle in the household cellar (InCellar mirrors that
// for the client); otherwise name is a style suggestion.
type PairingSuggestion struct {
	BottleID *int64 `json:"bottleId,omitempty"`
	Name     string `json:"name"`
	Reason   string `json:"reason"`
	InCellar bool   `json:"-"`
}

type pairingResponse struct {
	Pairings []PairingSuggestion `json:"pairings"`
}

// CocktailSuggestion is one bartender pick — a catalog Cocktail recipe.
// Missing lists ingredient names the model thinks the pantry lacks;
// display-only, advisory.
type CocktailSuggestion struct {
	RecipeID int64    `json:"recipeId"`
	Reason   string   `json:"reason"`
	Missing  []string `json:"missingIngredients,omitempty"`
}

type cocktailResponse struct {
	Suggestions []CocktailSuggestion `json:"suggestions"`
}

const pairingSystemPrompt = `You are LENA's sommelier. The user message contains a JSON object:
{"recipe":{...},"cellar":[{bottleId,vineyard,vintageYear,type,quantity,...}],"tastes":{...}}

Task: suggest wine pairings for the recipe. Prefer bottles already in the
household cellar (set bottleId); also offer general style picks (omit
bottleId, put a short style name like "off-dry Riesling" in "name").
Household tastes may inform the picks. Rules:
- bottleId must come from the provided cellar list — never invent one.
- "name" is the bottle's vineyard+vintage for cellar picks, or a style name.
- "reason" is one short phrase tying the pairing to the dish.
- Reply ONLY with {"pairings":[{...}]} — [] if the recipe defies pairing.`

const cocktailSystemPrompt = `You are LENA's bartender. The user message contains a JSON object:
{"cocktails":[{id,name,ingredients[]}],"pantry":[{name,...}],"tastes":{...}}

Task: suggest cocktails the household would enjoy. Judge what the pantry
can make: "missingIngredients" lists cocktail ingredient names not matched
by any pantry item (fuzzy matching is fine — "vodka" covers "vodka 80").
Household tastes may inform the picks. Rules:
- recipeId must come from the provided cocktail list — never invent one.
- "reason" is one short phrase ("uses your citrus", "household favorite").
- Reply ONLY with {"suggestions":[{...}]} — [] if nothing fits.`

// SuggestPairings recommends wines for a recipe: cellar bottles first,
// style suggestions second. Every bottleId is validated against the
// household cellar; unknown picks are dropped.
func (s *Service) SuggestPairings(ctx context.Context, userID, householdID, recipeID int64, maxSuggestions int) ([]PairingSuggestion, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	if maxSuggestions <= 0 {
		maxSuggestions = 4
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}

	detailAny, err := s.reg.Call(ctx, scope, "get_recipe_details", jsonArgs(map[string]any{"recipeIds": []int64{recipeID}}))
	if err != nil {
		return nil, err
	}
	details, ok := detailAny.([]tools.RecipeDetailRow)
	if !ok {
		return nil, fmt.Errorf("get_recipe_details returned %T", detailAny)
	}
	if len(details) == 0 {
		return nil, errors.New("recipe not found")
	}
	cellarAny, err := s.reg.Call(ctx, scope, "get_wine_cellar", jsonArgs(map[string]any{"limit": 200}))
	if err != nil {
		return nil, err
	}
	cellar, ok := cellarAny.([]tools.CellarBottleRow)
	if !ok {
		return nil, fmt.Errorf("get_wine_cellar returned %T", cellarAny)
	}
	tastesAny, err := s.reg.Call(ctx, scope, "get_household_tastes", jsonArgs(map[string]any{"limit": 10}))
	if err != nil {
		return nil, err
	}

	payload := struct {
		Recipe tools.RecipeDetailRow   `json:"recipe"`
		Cellar []tools.CellarBottleRow `json:"cellar"`
		Tastes any                     `json:"tastes"`
	}{Recipe: details[0], Cellar: cellar, Tastes: tastesAny}
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal pairing context: %w", err)
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: pairingSystemPrompt},
		{Role: llm.RoleUser, Content: string(reqBody)},
	}

	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.provider.Chat(ctx, llm.Request{Messages: msgs, JSONMode: true})
		if err != nil {
			return nil, fmt.Errorf("suggest provider: %w", err)
		}
		var parsed pairingResponse
		if err := json.Unmarshal([]byte(resp.Message.Content), &parsed); err != nil {
			msgs = append(msgs,
				llm.Message{Role: llm.RoleAssistant, Content: resp.Message.Content},
				llm.Message{Role: llm.RoleUser, Content: "That was not valid JSON matching the required schema. Reply ONLY with the JSON object."},
			)
			continue
		}
		return filterPairings(parsed.Pairings, cellar, maxSuggestions), nil
	}
	return nil, errors.New("assistant returned malformed suggestions")
}

// SuggestCocktails recommends Cocktail-category recipes the household can
// make. When inStockOnly is set, picks the model thinks need missing
// ingredients are dropped.
func (s *Service) SuggestCocktails(ctx context.Context, userID, householdID int64, maxSuggestions int, inStockOnly bool) ([]CocktailSuggestion, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	if maxSuggestions <= 0 {
		maxSuggestions = 6
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}

	cocktailsAny, err := s.reg.Call(ctx, scope, "list_cocktail_recipes", jsonArgs(map[string]any{"limit": 200}))
	if err != nil {
		return nil, err
	}
	cocktails, ok := cocktailsAny.([]tools.CocktailRow)
	if !ok {
		return nil, fmt.Errorf("list_cocktail_recipes returned %T", cocktailsAny)
	}
	if len(cocktails) == 0 {
		return []CocktailSuggestion{}, nil
	}
	pantryAny, err := s.reg.Call(ctx, scope, "get_pantry_inventory", jsonArgs(map[string]any{"limit": 200}))
	if err != nil {
		return nil, err
	}
	tastesAny, err := s.reg.Call(ctx, scope, "get_household_tastes", jsonArgs(map[string]any{"limit": 10}))
	if err != nil {
		return nil, err
	}

	payload := struct {
		Cocktails []tools.CocktailRow `json:"cocktails"`
		Pantry    any                 `json:"pantry"`
		Tastes    any                 `json:"tastes"`
	}{Cocktails: cocktails, Pantry: pantryAny, Tastes: tastesAny}
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal cocktail context: %w", err)
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: cocktailSystemPrompt},
		{Role: llm.RoleUser, Content: string(reqBody)},
	}

	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.provider.Chat(ctx, llm.Request{Messages: msgs, JSONMode: true})
		if err != nil {
			return nil, fmt.Errorf("suggest provider: %w", err)
		}
		var parsed cocktailResponse
		if err := json.Unmarshal([]byte(resp.Message.Content), &parsed); err != nil {
			msgs = append(msgs,
				llm.Message{Role: llm.RoleAssistant, Content: resp.Message.Content},
				llm.Message{Role: llm.RoleUser, Content: "That was not valid JSON matching the required schema. Reply ONLY with the JSON object."},
			)
			continue
		}
		return filterCocktails(parsed.Suggestions, cocktails, maxSuggestions, inStockOnly), nil
	}
	return nil, errors.New("assistant returned malformed suggestions")
}

// filterPairings drops picks with unknown bottleIds or bad fields, stamps
// the cellar identity on bottle picks, dedupes, and caps.
func filterPairings(in []PairingSuggestion, cellar []tools.CellarBottleRow, maxCount int) []PairingSuggestion {
	cellarByID := map[int64]tools.CellarBottleRow{}
	for _, b := range cellar {
		cellarByID[b.BottleID] = b
	}
	seen := map[string]bool{}
	out := make([]PairingSuggestion, 0, len(in))
	for _, p := range in {
		if p.BottleID != nil {
			b, ok := cellarByID[*p.BottleID]
			if !ok {
				continue
			}
			name := b.Vineyard
			if b.VintageYear > 0 {
				name = fmt.Sprintf("%s %d", b.Vineyard, b.VintageYear)
			}
			p.Name = name
			p.InCellar = true
		} else {
			p.Name = strings.TrimSpace(p.Name)
			if p.Name == "" || len(p.Name) > 80 {
				continue
			}
		}
		var bottleKey int64
		if p.BottleID != nil {
			bottleKey = *p.BottleID
		}
		key := fmt.Sprintf("%d|%s", bottleKey, strings.ToLower(p.Name))
		if seen[key] {
			continue
		}
		seen[key] = true
		p.Reason = truncate(strings.TrimSpace(p.Reason), 160)
		out = append(out, p)
	}
	if len(out) > maxCount {
		out = out[:maxCount]
	}
	return out
}

// filterCocktails drops picks for non-cocktail recipes, bounds strings,
// applies the in-stock toggle against the model's missing list, dedupes,
// and caps.
func filterCocktails(in []CocktailSuggestion, cocktails []tools.CocktailRow, maxCount int, inStockOnly bool) []CocktailSuggestion {
	valid := map[int64]bool{}
	for _, c := range cocktails {
		valid[c.ID] = true
	}
	seen := map[int64]bool{}
	out := make([]CocktailSuggestion, 0, len(in))
	for _, c := range in {
		if !valid[c.RecipeID] || seen[c.RecipeID] {
			continue
		}
		missing := make([]string, 0, len(c.Missing))
		for _, m := range c.Missing {
			m = strings.TrimSpace(m)
			if m != "" && len(m) <= 80 {
				missing = append(missing, m)
			}
		}
		c.Missing = missing
		if inStockOnly && len(missing) > 0 {
			continue
		}
		seen[c.RecipeID] = true
		c.Reason = truncate(strings.TrimSpace(c.Reason), 160)
		out = append(out, c)
	}
	if len(out) > maxCount {
		out = out[:maxCount]
	}
	return out
}
