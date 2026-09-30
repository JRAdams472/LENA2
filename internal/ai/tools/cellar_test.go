package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

type stubCellar struct {
	bottles []userprefs.HouseholdBottle
}

func (s *stubCellar) ListHouseholdBottles(_ context.Context, hh int64, _, _ int32) ([]userprefs.HouseholdBottle, error) {
	if hh != 9 {
		return nil, nil
	}
	return s.bottles, nil
}

type stubBottles struct {
	bottles map[int64]wine.Bottle
	types   []wine.Type
}

func (s *stubBottles) GetBottlesByIDs(_ context.Context, ids []int64) ([]wine.Bottle, error) {
	var out []wine.Bottle
	for _, id := range ids {
		if b, ok := s.bottles[id]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *stubBottles) ListTypes(context.Context) ([]wine.Type, error) {
	return s.types, nil
}

func cellarFixture() *Registry {
	cellar := &stubCellar{bottles: []userprefs.HouseholdBottle{
		{HouseholdBottleID: 1, HouseholdID: 9, BottleID: 501, Quantity: 3, Location: "rack"},
	}}
	bottles := &stubBottles{
		bottles: map[int64]wine.Bottle{
			501: {BottleID: 501, Vineyard: "Ridge", VintageYear: 2019, TypeID: 1},
		},
		types: []wine.Type{{TypeID: 1, Name: "Red"}},
	}
	rc := &stubRecipes{recipes: map[int64]recipe.Recipe{
		1: {RecipeID: 1, Name: "Margarita"},
		2: {RecipeID: 2, Name: "Lasagna"},
	}, cats: map[int64][]recipe.Category{
		1: {{CategoryID: 90, Name: "Cocktail", GroupName: "Dish Type"}},
		2: {{CategoryID: 1, Name: "Dinner", GroupName: "Dish Type"}},
	}, items: []recipe.RecipeItem{
		{RecipeItemID: 1, RecipeID: 1, ItemID: 100},
	}}
	namer := &stubNamer{items: map[int64]inventory.Item{100: {ItemID: 100, Name: "Tequila"}}}
	reg := New()
	RegisterCellarTools(reg, cellar, bottles, rc, namer)
	return reg
}

func TestGetWineCellar_Happy(t *testing.T) {
	reg := cellarFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_wine_cellar", nil)
	require.NoError(t, err)
	rows, ok := out.([]CellarBottleRow)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(501), rows[0].BottleID)
	assert.Equal(t, "Ridge", rows[0].Vineyard)
	assert.Equal(t, int32(2019), rows[0].VintageYear)
	assert.Equal(t, "Red", rows[0].Type)
	assert.Equal(t, int32(3), rows[0].Quantity)
	assert.Equal(t, "rack", rows[0].Location)
}

func TestGetWineCellar_EmptyHousehold(t *testing.T) {
	reg := cellarFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 0}, "get_wine_cellar", nil)
	require.NoError(t, err)
	rows, ok := out.([]CellarBottleRow)
	require.True(t, ok)
	assert.Empty(t, rows)
}

func TestGetWineCellar_ForeignHouseholdEmpty(t *testing.T) {
	reg := cellarFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 99}, "get_wine_cellar", nil)
	require.NoError(t, err)
	rows, ok := out.([]CellarBottleRow)
	require.True(t, ok)
	assert.Empty(t, rows)
}

func TestListCocktailRecipes_OnlyCocktails(t *testing.T) {
	reg := cellarFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "list_cocktail_recipes", nil)
	require.NoError(t, err)
	rows, ok := out.([]CocktailRow)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1), rows[0].ID)
	assert.Equal(t, "Margarita", rows[0].Name)
	assert.Equal(t, []string{"Tequila"}, rows[0].Ingredients)
}

func TestListCocktailRecipes_BadArgs(t *testing.T) {
	reg := cellarFixture()
	_, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9},
		"list_cocktail_recipes", json.RawMessage(`{"limit":"abc"}`))
	require.Error(t, err)
}
