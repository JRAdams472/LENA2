package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

func recipesFixture() *Registry {
	prep := int32(10)
	rc := &stubRecipes{
		recipes: map[int64]recipe.Recipe{
			1: {RecipeID: 1, Name: "Pancakes", PrepTimeMinutes: &prep},
			2: {RecipeID: 2, Name: "Pasta"},
		},
		cats: map[int64][]recipe.Category{
			1: {{CategoryID: 5, Name: "Breakfast"}},
		},
		items: []recipe.RecipeItem{
			{RecipeID: 1, ItemID: ptrInt64(10), Quantity: 2, UnitID: 1},
			{RecipeID: 1, ItemID: ptrInt64(11), Quantity: 1, UnitID: 2, IsOptional: true},
		},
	}
	items := &stubNamer{
		items: map[int64]inventory.Item{
			10: {ItemID: 10, Name: "Flour"},
			11: {ItemID: 11, Name: "Vanilla"},
		},
		units: map[int64]inventory.Unit{
			1: {UnitID: 1, Abbreviation: "c"},
			2: {UnitID: 2, Abbreviation: "tsp"},
		},
	}
	reg := New()
	RegisterRecipeTools(reg, rc, items)
	return reg
}

func TestListRecipes(t *testing.T) {
	reg := recipesFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "list_recipes", nil)
	require.NoError(t, err)
	rows, ok := out.([]RecipeRow)
	require.True(t, ok)
	assert.Len(t, rows, 2)
	for _, r := range rows {
		if r.ID == 1 {
			assert.Equal(t, "Pancakes", r.Name)
			assert.Equal(t, []string{"Breakfast"}, r.Categories)
		}
	}
}

func TestGetRecipeDetails(t *testing.T) {
	reg := recipesFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7},
		"get_recipe_details", json.RawMessage(`{"recipeIds":[1]}`))
	require.NoError(t, err)
	rows, ok := out.([]RecipeDetailRow)
	require.True(t, ok)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Ingredients, 2)
	assert.Equal(t, "Flour", rows[0].Ingredients[0].Name)
	assert.Equal(t, "c", rows[0].Ingredients[0].Unit)
	assert.True(t, rows[0].Ingredients[1].Optional)
}

func TestGetRecipeDetails_RequiresIDs(t *testing.T) {
	reg := recipesFixture()
	_, err := reg.Call(context.Background(), Scope{UserID: 7},
		"get_recipe_details", json.RawMessage(`{}`))
	require.Error(t, err)
}
