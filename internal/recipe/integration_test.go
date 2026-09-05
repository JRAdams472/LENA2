package recipe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

const itBy = "integration-test"

func TestIntegrationRecipeCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	svc := NewService(pool)

	active, err := svc.CreateRecipe(ctx, Recipe{
		Name:            "IT Recipe Active",
		Description:     "test recipe",
		Servings:        4,
		PrepTimeMinutes: 10,
		CookTimeMinutes: 20,
		IsActive:        true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, active.RecipeID)
	assert.Equal(t, "IT Recipe Active", active.Name)
	assert.Equal(t, int32(4), active.Servings)
	assert.True(t, active.IsActive)

	inactive, err := svc.CreateRecipe(ctx, Recipe{
		Name:     "IT Recipe Inactive",
		IsActive: false,
	}, itBy)
	require.NoError(t, err)

	got, err := svc.GetRecipeByID(ctx, active.RecipeID)
	require.NoError(t, err)
	assert.Equal(t, "IT Recipe Active", got.Name)
	assert.Equal(t, "test recipe", got.Description)
	assert.Equal(t, int32(10), got.PrepTimeMinutes)
	assert.Equal(t, int32(20), got.CookTimeMinutes)

	// Active/inactive list filtering.
	actives, err := svc.ListRecipes(ctx, true, 100, 0)
	require.NoError(t, err)
	var activeFound, inactiveInActive bool
	for _, r := range actives {
		if r.RecipeID == active.RecipeID {
			activeFound = true
		}
		if r.RecipeID == inactive.RecipeID {
			inactiveInActive = true
		}
	}
	assert.True(t, activeFound)
	assert.False(t, inactiveInActive, "inactive recipe should not appear in active list")

	inactives, err := svc.ListRecipes(ctx, false, 100, 0)
	require.NoError(t, err)
	var inactiveFound bool
	for _, r := range inactives {
		if r.RecipeID == inactive.RecipeID {
			inactiveFound = true
		}
	}
	assert.True(t, inactiveFound)

	require.NoError(t, svc.UpdateRecipe(ctx, active.RecipeID, Recipe{
		Name:            "IT Recipe Updated",
		Description:     "updated",
		Servings:        2,
		PrepTimeMinutes: 5,
		CookTimeMinutes: 15,
		IsActive:        false,
	}, itBy))
	got, err = svc.GetRecipeByID(ctx, active.RecipeID)
	require.NoError(t, err)
	assert.Equal(t, "IT Recipe Updated", got.Name)
	assert.Equal(t, int32(2), got.Servings)
	assert.False(t, got.IsActive)

	// Unique constraint on recipe name.
	_, err = svc.CreateRecipe(ctx, Recipe{Name: "IT Recipe Updated", IsActive: true}, itBy)
	assert.Error(t, err, "duplicate recipe name should violate unique constraint")

	require.NoError(t, svc.DeleteRecipe(ctx, inactive.RecipeID))
	_, err = svc.GetRecipeByID(ctx, inactive.RecipeID)
	assert.Error(t, err)
}

func TestIntegrationRecipeItemsAndSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	svc := NewService(pool)

	// Create an inventory item for recipe_item FK.
	invSvc := inventory.NewService(pool)
	cat, err := invSvc.CreateCategory(ctx, "IT Recipe Category", "", itBy)
	require.NoError(t, err)
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Recipe Item",
		CategoryID: cat.CategoryID,
		Unit:       "g",
	}, itBy)
	require.NoError(t, err)

	rec, err := svc.CreateRecipe(ctx, Recipe{
		Name:     "IT Recipe Full",
		IsActive: true,
	}, itBy)
	require.NoError(t, err)

	// Recipe items.
	require.NoError(t, svc.AddRecipeItem(ctx, RecipeItem{
		RecipeID:   rec.RecipeID,
		ItemID:     item.ItemID,
		Quantity:   2.5,
		Unit:       "cup",
		Notes:      "chopped",
		IsOptional: true,
	}))
	items, err := svc.ListRecipeItems(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, item.ItemID, items[0].ItemID)
	assert.InDelta(t, 2.5, items[0].Quantity, 0.0001)
	assert.Equal(t, "cup", items[0].Unit)
	assert.Equal(t, "chopped", items[0].Notes)
	assert.True(t, items[0].IsOptional)

	// FK violation: recipe item referencing a non-existent inventory item.
	err = svc.AddRecipeItem(ctx, RecipeItem{
		RecipeID: rec.RecipeID,
		ItemID:   99999999,
		Quantity: 1,
		Unit:     "g",
	})
	assert.Error(t, err, "recipe item with non-existent item_id should fail")

	require.NoError(t, svc.RemoveRecipeItem(ctx, rec.RecipeID, item.ItemID))
	items, err = svc.ListRecipeItems(ctx, rec.RecipeID)
	require.NoError(t, err)
	assert.Empty(t, items)

	// Recipe steps: added out of order, listed in step_number order.
	step2, err := svc.AddRecipeStep(ctx, rec.RecipeID, 2, "second step", itBy)
	require.NoError(t, err)
	step1, err := svc.AddRecipeStep(ctx, rec.RecipeID, 1, "first step", itBy)
	require.NoError(t, err)
	assert.Equal(t, rec.RecipeID, step1.RecipeID)

	steps, err := svc.ListRecipeSteps(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, steps, 2)
	assert.Equal(t, int32(1), steps[0].StepNumber)
	assert.Equal(t, "first step", steps[0].Instruction)
	assert.Equal(t, int32(2), steps[1].StepNumber)

	// Unique constraint on (recipe_id, step_number).
	_, err = svc.AddRecipeStep(ctx, rec.RecipeID, 1, "dup step", itBy)
	assert.Error(t, err, "duplicate step number should violate unique constraint")

	require.NoError(t, svc.UpdateRecipeStep(ctx, step2.StepID, 3, "renumbered step", itBy))
	steps, err = svc.ListRecipeSteps(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, steps, 2)
	assert.Equal(t, int32(3), steps[1].StepNumber)
	assert.Equal(t, "renumbered step", steps[1].Instruction)

	require.NoError(t, svc.DeleteRecipeStep(ctx, step1.StepID))
	steps, err = svc.ListRecipeSteps(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, step2.StepID, steps[0].StepID)

	// DeleteRecipe cascades to remaining steps.
	require.NoError(t, svc.DeleteRecipe(ctx, rec.RecipeID))
	_, err = svc.GetRecipeByID(ctx, rec.RecipeID)
	assert.Error(t, err)
}
