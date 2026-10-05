package recipe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

// TestIntegrationRecipeDeltaLifecycle exercises the full delta flow against
// a real database: write, read-back, apply, stale on canonical edit,
// acknowledge, anchor orphaning on base-line delete, and clear.
func TestIntegrationRecipeDeltaLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)
	svc := NewService(pool)
	invSvc := inventory.NewService(pool)
	householdID := testutil.MustHousehold(ctx, t, pool)

	cat, err := invSvc.CreateCategory(ctx, "IT Delta Category", "", false, itBy)
	require.NoError(t, err)
	flour, err := invSvc.CreateItem(ctx, inventory.Item{
		Name: "IT Delta Flour", CategoryID: cat.CategoryID, UnitID: itUnitID(t, ctx, invSvc, "g"),
	}, itBy)
	require.NoError(t, err)
	sugar, err := invSvc.CreateItem(ctx, inventory.Item{
		Name: "IT Delta Sugar", CategoryID: cat.CategoryID, UnitID: itUnitID(t, ctx, invSvc, "g"),
	}, itBy)
	require.NoError(t, err)
	cupID := itUnitID(t, ctx, invSvc, "cup")
	gID := itUnitID(t, ctx, invSvc, "g")

	rec, err := svc.CreateRecipe(ctx, Recipe{Name: "IT Delta Recipe", IsActive: true}, itBy)
	require.NoError(t, err)
	require.NoError(t, svc.AddRecipeItem(ctx, RecipeItem{
		RecipeID: rec.RecipeID, ItemID: i64(flour.ItemID), Quantity: 2, UnitID: cupID, DisplayOrder: 1,
	}))
	require.NoError(t, svc.AddRecipeItem(ctx, RecipeItem{
		RecipeID: rec.RecipeID, ItemID: i64(sugar.ItemID), Quantity: 1, UnitID: cupID, DisplayOrder: 2,
	}))
	_, err = svc.AddRecipeStep(ctx, rec.RecipeID, 1, "mix", itBy)
	require.NoError(t, err)
	_, err = svc.AddRecipeStep(ctx, rec.RecipeID, 2, "bake", itBy)
	require.NoError(t, err)
	items, err := svc.ListRecipeItems(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	steps, err := svc.ListRecipeSteps(ctx, rec.RecipeID)
	require.NoError(t, err)
	require.Len(t, steps, 2)

	// Write the household delta.
	_, err = svc.SetRecipeDelta(ctx, rec.RecipeID, householdID, []DeltaItem{
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(items[0].RecipeItemID), ItemID: i64(sugar.ItemID)},
		{Kind: DeltaItemAdjust, RecipeItemID: i64(items[1].RecipeItemID), Quantity: f2p(0.5)},
		{Kind: DeltaItemAdd, ItemID: i64(flour.ItemID), Quantity: f2p(0.25), UnitID: i64(gID), DisplayOrder: i32(3)},
	}, []DeltaStep{
		{Kind: DeltaStepReplace, StepID: i64(steps[0].StepID), Instruction: s2p("mix gently")},
		{Kind: DeltaStepAdd, StepNumber: i32(3), Instruction: s2p("cool")},
	}, itBy)
	require.NoError(t, err)

	delta, err := svc.GetRecipeDelta(ctx, rec.RecipeID, householdID)
	require.NoError(t, err)
	require.NotNil(t, delta)
	assert.Len(t, delta.Items, 3)
	assert.Len(t, delta.Steps, 2)
	assert.False(t, delta.Stale(rec.UpdatedAt))

	// Apply: effective view has substituted + adjusted + added lines and
	// replaced + added steps.
	canItems, _ := svc.ListRecipeItems(ctx, rec.RecipeID)
	canSteps, _ := svc.ListRecipeSteps(ctx, rec.RecipeID)
	eff := ApplyDelta(canItems, canSteps, delta)
	require.Len(t, eff.Items, 3)
	assert.Equal(t, sugar.ItemID, *eff.Items[0].ItemID)
	assert.InDelta(t, 0.5, eff.Items[1].Quantity, 0.0001)
	assert.Equal(t, "add", eff.Items[2].DeltaKind)
	require.Len(t, eff.Steps, 3)
	assert.Equal(t, "mix gently", eff.Steps[0].Instruction)
	assert.Equal(t, "cool", eff.Steps[2].Instruction)

	// Canonical edit → stale.
	time.Sleep(1100 * time.Millisecond) // base_updated_at resolution
	require.NoError(t, svc.UpdateRecipe(ctx, rec.RecipeID, Recipe{Name: "IT Delta Recipe v2", IsActive: true}, itBy))
	updated, err := svc.GetRecipeByID(ctx, rec.RecipeID)
	require.NoError(t, err)
	assert.True(t, delta.Stale(updated.UpdatedAt), "canonical edit should mark the delta stale")

	// Acknowledge clears it.
	require.NoError(t, svc.AcknowledgeRecipeDelta(ctx, rec.RecipeID, householdID, itBy))
	delta, err = svc.GetRecipeDelta(ctx, rec.RecipeID, householdID)
	require.NoError(t, err)
	assert.False(t, delta.Stale(updated.UpdatedAt))

	// Deleting the anchored base line orphans its delta row.
	require.NoError(t, svc.RemoveRecipeItem(ctx, items[0].RecipeItemID))
	delta, err = svc.GetRecipeDelta(ctx, rec.RecipeID, householdID)
	require.NoError(t, err)
	var orphan *DeltaItem
	for i := range delta.Items {
		// 'add' rows legitimately have no anchor — the orphan is a
		// non-add row whose anchor the base-line delete SET NULL'd.
		if delta.Items[i].RecipeItemID == nil && delta.Items[i].Kind != DeltaItemAdd {
			orphan = &delta.Items[i]
		}
	}
	require.NotNil(t, orphan, "deleted base line should leave an orphaned delta row")
	assert.Equal(t, DeltaItemSubstitute, orphan.Kind)
	canItems, _ = svc.ListRecipeItems(ctx, rec.RecipeID)
	eff = ApplyDelta(canItems, nil, delta)
	assert.Equal(t, 1, eff.OrphanedItems)

	// Clear removes the whole delta.
	require.NoError(t, svc.ClearRecipeDelta(ctx, rec.RecipeID, householdID))
	delta, err = svc.GetRecipeDelta(ctx, rec.RecipeID, householdID)
	require.NoError(t, err)
	assert.Nil(t, delta)
	err = svc.ClearRecipeDelta(ctx, rec.RecipeID, householdID)
	require.ErrorIs(t, err, domainerr.ErrNotFound)
}
