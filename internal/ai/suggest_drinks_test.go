package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

func int64ptr(v int64) *int64 { return &v }

type sugCellar struct{}

func (sugCellar) ListHouseholdBottles(_ context.Context, hh int64, _, _ int32) ([]userprefs.HouseholdBottle, error) {
	if hh != 9 {
		return nil, nil
	}
	return []userprefs.HouseholdBottle{
		{HouseholdBottleID: 1, HouseholdID: 9, BottleID: 501, Quantity: 2},
		{HouseholdBottleID: 2, HouseholdID: 9, BottleID: 502, Quantity: 1},
	}, nil
}

type sugBottles struct{}

func (sugBottles) GetBottlesByIDs(_ context.Context, ids []int64) ([]wine.Bottle, error) {
	var out []wine.Bottle
	for _, id := range ids {
		out = append(out, wine.Bottle{
			BottleID:    id,
			Vineyard:    fmt.Sprintf("Estate %d", id),
			VintageYear: 2021,
			TypeID:      1,
		})
	}
	return out, nil
}

func (sugBottles) ListTypes(context.Context) ([]wine.Type, error) {
	return []wine.Type{{TypeID: 1, Name: "Red"}}, nil
}

// sugCocktailRecipes serves two Cocktail-category recipes plus plain ones.
type sugCocktailRecipes struct{}

func (sugCocktailRecipes) GetRecipesByIDs(_ context.Context, ids []int64) ([]recipe.Recipe, error) {
	known := map[int64]string{1: "Margarita", 2: "Old Fashioned", 3: "Lasagna"}
	var out []recipe.Recipe
	for _, id := range ids {
		if n, ok := known[id]; ok {
			out = append(out, recipe.Recipe{RecipeID: id, Name: n})
		}
	}
	return out, nil
}

func (sugCocktailRecipes) ListRecipes(context.Context, bool, int32, int32) ([]recipe.Recipe, error) {
	return []recipe.Recipe{
		{RecipeID: 1, Name: "Margarita"},
		{RecipeID: 2, Name: "Old Fashioned"},
		{RecipeID: 3, Name: "Lasagna"},
	}, nil
}

func (sugCocktailRecipes) ListCategoriesForRecipes(context.Context, []int64) (map[int64][]recipe.Category, error) {
	return map[int64][]recipe.Category{
		1: {{CategoryID: 90, Name: "Cocktail", GroupName: "Dish Type"}},
		2: {{CategoryID: 90, Name: "Cocktail", GroupName: "Dish Type"}},
		3: {{CategoryID: 1, Name: "Dinner", GroupName: "Dish Type"}},
	}, nil
}

func (sugCocktailRecipes) ListRecipeItemsByRecipes(_ context.Context, ids []int64) ([]recipe.RecipeItem, error) {
	var out []recipe.RecipeItem
	for _, id := range ids {
		out = append(out, recipe.RecipeItem{RecipeItemID: id, RecipeID: id, ItemID: int64ptr(id * 100)})
	}
	return out, nil
}

func drinkService(t *testing.T, p llm.Provider, rc tools.RecipeCatalog) *Service {
	t.Helper()
	reg := tools.New()
	tools.RegisterPantryTools(reg, sugPantry{}, sugNamer{})
	tools.RegisterRecipeTools(reg, rc, sugNamer{})
	tools.RegisterTasteTools(reg, sugTastes{}, rc)
	tools.RegisterCellarTools(reg, sugCellar{}, sugBottles{}, rc, sugNamer{})
	return NewService(p, reg, Config{})
}

func enqueuePairings(p *llm.MockProvider, pairings ...PairingSuggestion) {
	b, _ := json.Marshal(pairingResponse{Pairings: pairings})
	p.EnqueueText(string(b))
}

func TestSuggestPairings_Happy(t *testing.T) {
	p := llm.NewMockProvider()
	bottle := int64(501)
	enqueuePairings(p,
		PairingSuggestion{BottleID: &bottle, Name: "ignored", Reason: "tannins cut the fat"},
		PairingSuggestion{Name: "off-dry Riesling", Reason: "acidity lifts the sauce"},
	)
	svc := drinkService(t, p, sugCocktailRecipes{})
	got, err := svc.SuggestPairings(context.Background(), 7, 9, 1, 4)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Cellar pick: name is forced to the bottle's identity, flagged in-cellar.
	assert.True(t, got[0].InCellar)
	assert.Equal(t, "Estate 501 2021", got[0].Name)
	// Style pick: model name kept, no bottle.
	assert.False(t, got[1].InCellar)
	assert.Nil(t, got[1].BottleID)
	assert.Equal(t, "off-dry Riesling", got[1].Name)
	assert.True(t, p.Requests[0].JSONMode)
	assert.Contains(t, p.Requests[0].Messages[1].Content, "Margarita")
	assert.Contains(t, p.Requests[0].Messages[1].Content, "Estate 501")
}

func TestSuggestPairings_FiltersInvalid(t *testing.T) {
	p := llm.NewMockProvider()
	bad := int64(999)
	bottle := int64(501)
	enqueuePairings(p,
		PairingSuggestion{BottleID: &bad, Name: "ghost bottle", Reason: "not in cellar"},
		PairingSuggestion{Name: "", Reason: "empty name"},
		PairingSuggestion{BottleID: &bottle, Reason: "good"},
		PairingSuggestion{BottleID: &bottle, Reason: "dup"},
	)
	svc := drinkService(t, p, sugCocktailRecipes{})
	got, err := svc.SuggestPairings(context.Background(), 7, 9, 1, 4)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "good", got[0].Reason)
}

func TestSuggestPairings_RecipeNotFound(t *testing.T) {
	p := llm.NewMockProvider()
	svc := drinkService(t, p, sugCocktailRecipes{})
	_, err := svc.SuggestPairings(context.Background(), 7, 9, 42, 4)
	require.Error(t, err)
	assert.Equal(t, 0, p.CallCount())
}

func TestSuggestPairings_Unavailable(t *testing.T) {
	svc := drinkService(t, nil, sugCocktailRecipes{})
	_, err := svc.SuggestPairings(context.Background(), 7, 9, 1, 4)
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestSuggestPairings_RetriesMalformed(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("as an AI I cannot recommend alcohol")
	bottle := int64(502)
	enqueuePairings(p, PairingSuggestion{BottleID: &bottle, Reason: "retry"})
	svc := drinkService(t, p, sugCocktailRecipes{})
	got, err := svc.SuggestPairings(context.Background(), 7, 9, 1, 4)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, p.CallCount())
}

func enqueueCocktails(p *llm.MockProvider, suggs ...CocktailSuggestion) {
	b, _ := json.Marshal(cocktailResponse{Suggestions: suggs})
	p.EnqueueText(string(b))
}

func TestSuggestCocktails_Happy(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueCocktails(p,
		CocktailSuggestion{RecipeID: 1, Reason: "citrus on hand"},
		CocktailSuggestion{RecipeID: 2, Reason: "needs bitters", Missing: []string{"bitters"}},
	)
	svc := drinkService(t, p, sugCocktailRecipes{})
	got, err := svc.SuggestCocktails(context.Background(), 7, 9, 6, false)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].RecipeID)
	assert.Equal(t, []string{"bitters"}, got[1].Missing)
	assert.Contains(t, p.Requests[0].Messages[1].Content, "Margarita")
}

func TestSuggestCocktails_InStockOnlyFiltersMissing(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueCocktails(p,
		CocktailSuggestion{RecipeID: 1, Reason: "makeable"},
		CocktailSuggestion{RecipeID: 2, Reason: "missing", Missing: []string{"bitters"}},
	)
	svc := drinkService(t, p, sugCocktailRecipes{})
	got, err := svc.SuggestCocktails(context.Background(), 7, 9, 6, true)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].RecipeID)
}

func TestSuggestCocktails_FiltersNonCocktail(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueCocktails(p,
		CocktailSuggestion{RecipeID: 3, Reason: "lasagna is not a cocktail"},
		CocktailSuggestion{RecipeID: 99, Reason: "invented"},
		CocktailSuggestion{RecipeID: 1, Reason: "real"},
		CocktailSuggestion{RecipeID: 1, Reason: "dup"},
	)
	svc := drinkService(t, p, sugCocktailRecipes{})
	got, err := svc.SuggestCocktails(context.Background(), 7, 9, 6, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "real", got[0].Reason)
}

func TestSuggestCocktails_Unavailable(t *testing.T) {
	svc := drinkService(t, nil, sugCocktailRecipes{})
	_, err := svc.SuggestCocktails(context.Background(), 7, 9, 6, false)
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestSuggestCocktails_NoCocktailsSkipsModel(t *testing.T) {
	p := llm.NewMockProvider()
	svc := drinkService(t, p, sugRecipes{}) // no Cocktail-category recipes
	got, err := svc.SuggestCocktails(context.Background(), 7, 9, 6, false)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, 0, p.CallCount())
}
