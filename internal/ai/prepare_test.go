package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/recipe"
)

func TestPrepareRequest_Meals(t *testing.T) {
	svc := suggestService(t, nil)
	p, err := svc.PrepareRequest(context.Background(), 7, 9, "suggest-meals",
		json.RawMessage(`{"mealPlanId":10,"maxSuggestions":4}`))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Contains(t, p.Prompt, "meal-planning assistant")
	assert.Contains(t, string(p.OutputSchema), "suggestions")

	// The assembled context carries the plan, open cells and candidates —
	// everything a client-side engine needs to generate identical output.
	var ctx map[string]any
	require.NoError(t, json.Unmarshal(p.Context, &ctx))
	assert.Contains(t, ctx, "candidates")
	assert.Contains(t, ctx, "occupied")
	assert.EqualValues(t, 4, ctx["maxSuggestions"])
}

func TestPrepareRequest_MealsForeignPlan(t *testing.T) {
	svc := suggestService(t, nil)
	// Plan 10 belongs to household 9 — household 42 gets nothing.
	_, err := svc.PrepareRequest(context.Background(), 7, 42, "suggest-meals",
		json.RawMessage(`{"mealPlanId":10}`))
	require.Error(t, err)
}

func TestPrepareRequest_BadParams(t *testing.T) {
	svc := suggestService(t, nil)

	_, err := svc.PrepareRequest(context.Background(), 7, 9, "suggest-meals",
		json.RawMessage(`{}`))
	assert.ErrorIs(t, err, ErrBadParams)

	_, err = svc.PrepareRequest(context.Background(), 7, 9, "suggest-meals",
		json.RawMessage(`{malformed`))
	assert.ErrorIs(t, err, ErrBadParams)
}

func TestPrepareRequest_UnknownName(t *testing.T) {
	svc := suggestService(t, nil)
	_, err := svc.PrepareRequest(context.Background(), 7, 9, "exfiltrate",
		json.RawMessage(`{}`))
	assert.ErrorIs(t, err, ErrUnknownRequest)
}

func TestPrepareRequest_NilService(t *testing.T) {
	var svc *Service
	_, err := svc.PrepareRequest(context.Background(), 7, 9, "suggest-meals", nil)
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestPrepareRequest_CocktailsEmptyCatalog(t *testing.T) {
	// drinkService registers the cocktail tool; a catalog with no Cocktail
	// dishes → nil request (nothing to generate), matching SuggestCocktails.
	svc := drinkService(t, nil, emptyCocktailRecipes{})
	p, err := svc.PrepareRequest(context.Background(), 7, 9, "suggest-cocktails",
		json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Nil(t, p)
}

// emptyCocktailRecipes satisfies tools.RecipeCatalog with no Cocktail
// dish type — prepareCocktails early-returns nil.
type emptyCocktailRecipes struct{ sugCocktailRecipes }

func (emptyCocktailRecipes) ListCategoriesForRecipes(context.Context, []int64) (map[int64][]recipe.Category, error) {
	return map[int64][]recipe.Category{}, nil
}
