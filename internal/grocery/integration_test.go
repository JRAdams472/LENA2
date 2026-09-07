package grocery

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
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

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool), pool
}

func TestIntegrationGroceryLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testenv.MustUser(ctx, t, pool, "grocery-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "grocery-b@example.com")

	invSvc := inventory.NewService(pool)
	brand, err := invSvc.CreateBrand(ctx, "IT Grocery Brand")
	require.NoError(t, err)
	cat, err := invSvc.CreateCategory(ctx, "IT Grocery Category", "", itBy)
	require.NoError(t, err)
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Grocery Item",
		BrandID:    &brand.BrandID,
		CategoryID: cat.CategoryID,
		UnitID:     itUnitID(t, ctx, invSvc, "each"),
	}, itBy)
	require.NoError(t, err)

	mpSvc := mealplan.NewService(pool)
	week := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	plan, err := mpSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		UserID:             userA,
		Name:               "Grocery Week",
		WeekStartDate:      week,
		WeekStartDayOfWeek: 1,
		IsActive:           true,
	}, itBy)
	require.NoError(t, err)

	list, err := svc.CreateGroceryList(ctx, userA, nil, itBy)
	require.NoError(t, err)
	require.NotZero(t, list.GroceryListID)
	assert.Equal(t, userA, list.UserID)
	assert.Nil(t, list.MealPlanID)

	gotList, err := svc.GetGroceryListByID(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	assert.Equal(t, list.GroceryListID, gotList.GroceryListID)

	_, err = svc.GetGroceryListByID(ctx, list.GroceryListID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	listsA, err := svc.ListGroceryLists(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, listsA, 1)

	listsB, err := svc.ListGroceryLists(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, listsB)

	lbID := itUnitID(t, ctx, invSvc, "lb")
	manualItem, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ManualItemName: "apples",
		QuantityNeeded: 2.0,
		UnitID:         &lbID,
		Source:         "manual",
		IsChecked:      false,
	}, userA, itBy)
	require.NoError(t, err)
	require.NotZero(t, manualItem.GroceryListItemID)

	itemID := item.ItemID
	canID := itUnitID(t, ctx, invSvc, "can")
	catalogItem, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ItemID:         &itemID,
		QuantityNeeded: 1.0,
		UnitID:         &canID,
		Source:         "pantry",
		IsChecked:      false,
	}, userA, itBy)
	require.NoError(t, err)
	require.NotZero(t, catalogItem.GroceryListItemID)

	items, err := svc.ListGroceryListItems(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	require.Len(t, items, 2)

	gotItem, err := svc.GetGroceryListItemByID(ctx, manualItem.GroceryListItemID, userA)
	require.NoError(t, err)
	assert.Equal(t, "apples", gotItem.ManualItemName)
	assert.False(t, gotItem.IsChecked)

	require.NoError(t, svc.UpdateGroceryListItem(ctx, manualItem.GroceryListItemID, userA, GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ManualItemName: "green apples",
		QuantityNeeded: 3.0,
		UnitID:         &lbID,
		Source:         "manual",
		IsChecked:      true,
	}, itBy))
	updatedItem, err := svc.GetGroceryListItemByID(ctx, manualItem.GroceryListItemID, userA)
	require.NoError(t, err)
	assert.Equal(t, "green apples", updatedItem.ManualItemName)
	assert.InDelta(t, 3.0, updatedItem.QuantityNeeded, 0.0001)
	assert.True(t, updatedItem.IsChecked)

	require.NoError(t, svc.DeleteGroceryListItem(ctx, catalogItem.GroceryListItemID, userA))
	items, err = svc.ListGroceryListItems(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, manualItem.GroceryListItemID, items[0].GroceryListItemID)

	gen, err := svc.Generate(ctx, userA, plan.MealPlanID, itBy)
	require.NoError(t, err)
	require.NotZero(t, gen.GroceryListID)
	require.NotNil(t, gen.MealPlanID)
	assert.Equal(t, plan.MealPlanID, *gen.MealPlanID)
	assert.Equal(t, userA, gen.UserID)

	listsA, err = svc.ListGroceryLists(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, listsA, 2)

	_, err = svc.GetGroceryListByID(ctx, gen.GroceryListID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	require.NoError(t, svc.DeleteGroceryList(ctx, gen.GroceryListID, userB))
	_, err = svc.GetGroceryListByID(ctx, gen.GroceryListID, userA)
	require.NoError(t, err, "generated list should still exist after wrong-user delete")

	require.NoError(t, svc.DeleteGroceryList(ctx, gen.GroceryListID, userA))
	_, err = svc.GetGroceryListByID(ctx, gen.GroceryListID, userA)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	require.NoError(t, svc.DeleteGroceryList(ctx, list.GroceryListID, userA))
	_, err = svc.GetGroceryListByID(ctx, list.GroceryListID, userA)
	require.Error(t, err)
	items, err = svc.ListGroceryListItems(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// TestIntegrationGroceryCrossUserDenied verifies that every list-item
// operation is scoped to the owning user (LENA-001).
func TestIntegrationGroceryCrossUserDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testenv.MustUser(ctx, t, pool, "grocery-xu-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "grocery-xu-b@example.com")

	list, err := svc.CreateGroceryList(ctx, userA, nil, itBy)
	require.NoError(t, err)

	gli, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID: list.GroceryListID, ManualItemName: "milk",
		QuantityNeeded: 1, Source: "manual",
	}, userA, itBy)
	require.NoError(t, err)

	// Reads and writes as userB must see or affect nothing.
	_, err = svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID: list.GroceryListID, ManualItemName: "eggs",
		QuantityNeeded: 1, Source: "manual",
	}, userB, itBy)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	_, err = svc.GetGroceryListItemByID(ctx, gli.GroceryListItemID, userB)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	items, err := svc.ListGroceryListItems(ctx, list.GroceryListID, userB)
	require.NoError(t, err)
	assert.Empty(t, items)

	items, err = svc.ListGroceryListItemsByLists(ctx, []int64{list.GroceryListID}, userB)
	require.NoError(t, err)
	assert.Empty(t, items)

	require.NoError(t, svc.UpdateGroceryListItem(ctx, gli.GroceryListItemID, userB, GroceryListItem{
		ManualItemName: "tampered", QuantityNeeded: 99, Source: "manual", IsChecked: true,
	}, itBy))
	got, err := svc.GetGroceryListItemByID(ctx, gli.GroceryListItemID, userA)
	require.NoError(t, err)
	assert.Equal(t, "milk", got.ManualItemName, "wrong-user update must not mutate the item")

	require.NoError(t, svc.DeleteGroceryListItem(ctx, gli.GroceryListItemID, userB))
	items, err = svc.ListGroceryListItems(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	assert.Len(t, items, 1, "wrong-user delete must not remove the item")
}
