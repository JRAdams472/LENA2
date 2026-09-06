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

// itUnitID returns the id of a seeded unit by name or abbreviation.
func itUnitID(t *testing.T, ctx context.Context, invSvc *inventory.Service, name string) int64 {
	t.Helper()
	u, err := invSvc.GetUnitByName(ctx, name)
	require.NoError(t, err, "unit %q should be seeded by migration 0012", name)
	return u.UnitID
}

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
		Servings:        i32(4),
		PrepTimeMinutes: i32(10),
		CookTimeMinutes: i32(20),
		IsActive:        true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, active.RecipeID)
	assert.Equal(t, "IT Recipe Active", active.Name)
	require.NotNil(t, active.Servings)
	assert.Equal(t, int32(4), *active.Servings)
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
	require.NotNil(t, got.PrepTimeMinutes)
	assert.Equal(t, int32(10), *got.PrepTimeMinutes)
	require.NotNil(t, got.CookTimeMinutes)
	assert.Equal(t, int32(20), *got.CookTimeMinutes)

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
		Servings:        i32(2),
		PrepTimeMinutes: i32(5),
		CookTimeMinutes: i32(15),
		IsActive:        false,
	}, itBy))
	got, err = svc.GetRecipeByID(ctx, active.RecipeID)
	require.NoError(t, err)
	assert.Equal(t, "IT Recipe Updated", got.Name)
	require.NotNil(t, got.Servings)
	assert.Equal(t, int32(2), *got.Servings)
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
		UnitID:     itUnitID(t, ctx, invSvc, "g"),
	}, itBy)
	require.NoError(t, err)

	rec, err := svc.CreateRecipe(ctx, Recipe{
		Name:     "IT Recipe Full",
		IsActive: true,
	}, itBy)
	require.NoError(t, err)

	// Recipe items.
	cupID := itUnitID(t, ctx, invSvc, "cup")
	require.NoError(t, svc.AddRecipeItem(ctx, RecipeItem{
		RecipeID:     rec.RecipeID,
		ItemID:       item.ItemID,
		Quantity:     2.5,
		UnitID:       cupID,
		SectionName:  "filling",
		DisplayOrder: 1,
		Notes:        "chopped",
		IsOptional:   true,
	}))
	items, err := svc.ListRecipeItems(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, item.ItemID, items[0].ItemID)
	assert.NotZero(t, items[0].RecipeItemID)
	assert.InDelta(t, 2.5, items[0].Quantity, 0.0001)
	assert.Equal(t, cupID, items[0].UnitID)
	assert.Equal(t, "filling", items[0].SectionName)
	assert.Equal(t, int32(1), items[0].DisplayOrder)
	assert.Equal(t, "chopped", items[0].Notes)
	assert.True(t, items[0].IsOptional)

	// FK violation: recipe item referencing a non-existent inventory item.
	err = svc.AddRecipeItem(ctx, RecipeItem{
		RecipeID: rec.RecipeID,
		ItemID:   99999999,
		Quantity: 1,
		UnitID:   itUnitID(t, ctx, invSvc, "g"),
	})
	assert.Error(t, err, "recipe item with non-existent item_id should fail")

	require.NoError(t, svc.RemoveRecipeItem(ctx, items[0].RecipeItemID))
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

func TestIntegrationCreateRecipeWithChildrenRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	svc := NewService(pool)

	invSvc := inventory.NewService(pool)
	cat, err := invSvc.CreateCategory(ctx, "IT Children Category", "", itBy)
	require.NoError(t, err)
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Children Item",
		CategoryID: cat.CategoryID,
		UnitID:     itUnitID(t, ctx, invSvc, "g"),
	}, itBy)
	require.NoError(t, err)
	cupID := itUnitID(t, ctx, invSvc, "cup")
	gID := itUnitID(t, ctx, invSvc, "g")

	t.Run("success creates recipe with item and step", func(t *testing.T) {
		rec, err := svc.CreateRecipeWithChildren(ctx, Recipe{
			Name:            "Children Recipe",
			Description:     "with children",
			Servings:        i32(4),
			PrepTimeMinutes: i32(10),
			CookTimeMinutes: i32(20),
			IsActive:        true,
		}, []RecipeItem{
			{ItemID: item.ItemID, Quantity: 2.5, UnitID: cupID, Notes: "chopped", IsOptional: true},
		}, []RecipeStep{
			{StepNumber: 1, Instruction: "Mix"},
		}, itBy)
		require.NoError(t, err)
		require.NotZero(t, rec.RecipeID)

		items, err := svc.ListRecipeItems(ctx, rec.RecipeID)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, item.ItemID, items[0].ItemID)

		steps, err := svc.ListRecipeSteps(ctx, rec.RecipeID)
		require.NoError(t, err)
		require.Len(t, steps, 1)
		assert.Equal(t, "Mix", steps[0].Instruction)
	})

	t.Run("bad item id rolls back recipe creation", func(t *testing.T) {
		_, err := svc.CreateRecipeWithChildren(ctx, Recipe{
			Name:        "Bad Children Recipe",
			Description: "should not persist",
			IsActive:    true,
		}, []RecipeItem{
			{ItemID: 99999999, Quantity: 1, UnitID: gID},
		}, nil, itBy)
		require.Error(t, err)

		// Confirm no recipe was persisted: the unique name query should find none.
		lists, err := svc.ListRecipes(ctx, true, 100, 0)
		require.NoError(t, err)
		for _, r := range lists {
			assert.NotEqual(t, "Bad Children Recipe", r.Name)
		}
	})
}

func TestIntegrationUpdateRecipeWithChildrenRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	svc := NewService(pool)

	invSvc := inventory.NewService(pool)
	cat, err := invSvc.CreateCategory(ctx, "IT Update Children Category", "", itBy)
	require.NoError(t, err)
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Update Children Item",
		CategoryID: cat.CategoryID,
		UnitID:     itUnitID(t, ctx, invSvc, "g"),
	}, itBy)
	require.NoError(t, err)
	cupID := itUnitID(t, ctx, invSvc, "cup")
	gID := itUnitID(t, ctx, invSvc, "g")

	rec, err := svc.CreateRecipe(ctx, Recipe{Name: "Update Children Recipe", IsActive: true}, itBy)
	require.NoError(t, err)
	require.NoError(t, svc.AddRecipeItem(ctx, RecipeItem{RecipeID: rec.RecipeID, ItemID: item.ItemID, Quantity: 1, UnitID: cupID}))
	_, err = svc.AddRecipeStep(ctx, rec.RecipeID, 1, "First", itBy)
	require.NoError(t, err)

	// Updating with a non-existent item should fail and leave prior children intact.
	err = svc.UpdateRecipeWithChildren(ctx, rec.RecipeID, Recipe{Name: "Update Children Recipe", IsActive: true}, []RecipeItem{
		{ItemID: 99999999, Quantity: 1, UnitID: gID},
	}, []RecipeStep{{StepNumber: 2, Instruction: "Second"}}, itBy)
	require.Error(t, err)

	items, err := svc.ListRecipeItems(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, item.ItemID, items[0].ItemID)

	steps, err := svc.ListRecipeSteps(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "First", steps[0].Instruction)
}
