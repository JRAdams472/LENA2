package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
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

// pairingSuggestionsSchema constrains structured output for
// suggest-pairings — served in PreparedRequest.OutputSchema.
const pairingSuggestionsSchema = `{"type":"object","required":["pairings"],` +
	`"properties":{"pairings":{"type":"array","items":{"type":"object",` +
	`"required":["name","reason"],` +
	`"properties":{"bottleId":{"type":"integer"},"name":{"type":"string"},` +
	`"reason":{"type":"string"}}}},"additionalProperties":false}`

// cocktailSuggestionsSchema constrains structured output for
// suggest-cocktails — served in PreparedRequest.OutputSchema.
const cocktailSuggestionsSchema = `{"type":"object","required":["suggestions"],` +
	`"properties":{"suggestions":{"type":"array","items":{"type":"object",` +
	`"required":["recipeId","reason"],` +
	`"properties":{"recipeId":{"type":"integer"},"reason":{"type":"string"},` +
	`"missingIngredients":{"type":"array","items":{"type":"string"}}}}},"additionalProperties":false}`

// SuggestPairings runs the server-inference path for wine pairings.
func (s *Service) SuggestPairings(ctx context.Context, userID, householdID, recipeID int64, maxSuggestions int) ([]PairingSuggestion, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}
	p, err := s.preparePairings(ctx, scope, recipeID, maxSuggestions)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return []PairingSuggestion{}, nil
	}
	return runPrepared(ctx, s.provider, p)
}

// preparePairings assembles recipe/cellar/taste context for pairing picks.
// Every bottleId is validated against the household cellar; unknown picks
// are dropped by the returned validator.
func (s *Service) preparePairings(ctx context.Context, scope tools.Scope, recipeID int64, maxSuggestions int) (*prepared[[]PairingSuggestion], error) {
	if maxSuggestions <= 0 {
		maxSuggestions = 4
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}

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
	limit := maxSuggestions

	return &prepared[[]PairingSuggestion]{
		req: PreparedRequest{
			Prompt:       pairingSystemPrompt,
			Context:      reqBody,
			OutputSchema: json.RawMessage(pairingSuggestionsSchema),
		},
		validate: func(content string) ([]PairingSuggestion, error) {
			var parsed pairingResponse
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				return nil, err
			}
			return filterPairings(parsed.Pairings, cellar, limit), nil
		},
	}, nil
}

// SuggestCocktails runs the server-inference path for cocktail picks.
func (s *Service) SuggestCocktails(ctx context.Context, userID, householdID int64, maxSuggestions int, inStockOnly bool) ([]CocktailSuggestion, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}
	p, err := s.prepareCocktails(ctx, scope, maxSuggestions, inStockOnly)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return []CocktailSuggestion{}, nil
	}
	return runPrepared(ctx, s.provider, p)
}

// prepareCocktails assembles cocktail-catalog/pantry/taste context for
// bartender picks. Returns (nil, nil) when the catalog has no cocktails.
func (s *Service) prepareCocktails(ctx context.Context, scope tools.Scope, maxSuggestions int, inStockOnly bool) (*prepared[[]CocktailSuggestion], error) {
	if maxSuggestions <= 0 {
		maxSuggestions = 6
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}

	cocktailsAny, err := s.reg.Call(ctx, scope, "list_cocktail_recipes", jsonArgs(map[string]any{"limit": 200}))
	if err != nil {
		return nil, err
	}
	cocktails, ok := cocktailsAny.([]tools.CocktailRow)
	if !ok {
		return nil, fmt.Errorf("list_cocktail_recipes returned %T", cocktailsAny)
	}
	if len(cocktails) == 0 {
		return nil, nil
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
	limit := maxSuggestions

	return &prepared[[]CocktailSuggestion]{
		req: PreparedRequest{
			Prompt:       cocktailSystemPrompt,
			Context:      reqBody,
			OutputSchema: json.RawMessage(cocktailSuggestionsSchema),
		},
		validate: func(content string) ([]CocktailSuggestion, error) {
			var parsed cocktailResponse
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				return nil, err
			}
			return filterCocktails(parsed.Suggestions, cocktails, limit, inStockOnly), nil
		},
	}, nil
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
		if !resolvePairingName(&p, cellarByID) {
			continue
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

// resolvePairingName stamps the display name for a pairing pick: cellar
// bottles get the "Vineyard Year" identity and InCellar; free-text picks
// are trimmed and length-bounded. Returns false for unknown bottles and
// empty/oversized names.
func resolvePairingName(p *PairingSuggestion, cellarByID map[int64]tools.CellarBottleRow) bool {
	if p.BottleID == nil {
		p.Name = strings.TrimSpace(p.Name)
		return p.Name != "" && len(p.Name) <= 80
	}
	b, ok := cellarByID[*p.BottleID]
	if !ok {
		return false
	}
	p.Name = b.Vineyard
	if b.VintageYear > 0 {
		p.Name = fmt.Sprintf("%s %d", b.Vineyard, b.VintageYear)
	}
	p.InCellar = true
	return true
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
