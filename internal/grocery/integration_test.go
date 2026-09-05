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
		Unit:       "each",
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

	manualItem, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ManualItemName: "apples",
		QuantityNeeded: 2.0,
		UnitOfMeasure:  "lb",
		Source:         "manual",
		IsChecked:      false,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, manualItem.GroceryListItemID)

	itemID := item.ItemID
	catalogItem, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ItemID:         &itemID,
		QuantityNeeded: 1.0,
		UnitOfMeasure:  "bottle",
		Source:         "pantry",
		IsChecked:      false,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, catalogItem.GroceryListItemID)

	items, err := svc.ListGroceryListItems(ctx, list.GroceryListID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	gotItem, err := svc.GetGroceryListItemByID(ctx, manualItem.GroceryListItemID)
	require.NoError(t, err)
	assert.Equal(t, "apples", gotItem.ManualItemName)
	assert.False(t, gotItem.IsChecked)

	require.NoError(t, svc.UpdateGroceryListItem(ctx, manualItem.GroceryListItemID, GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ManualItemName: "green apples",
		QuantityNeeded: 3.0,
		UnitOfMeasure:  "lb",
		Source:         "manual",
		IsChecked:      true,
	}, itBy))
	updatedItem, err := svc.GetGroceryListItemByID(ctx, manualItem.GroceryListItemID)
	require.NoError(t, err)
	assert.Equal(t, "green apples", updatedItem.ManualItemName)
	assert.InDelta(t, 3.0, updatedItem.QuantityNeeded, 0.0001)
	assert.True(t, updatedItem.IsChecked)

	require.NoError(t, svc.DeleteGroceryListItem(ctx, catalogItem.GroceryListItemID))
	items, err = svc.ListGroceryListItems(ctx, list.GroceryListID)
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
	items, err = svc.ListGroceryListItems(ctx, list.GroceryListID)
	require.NoError(t, err)
	assert.Empty(t, items)
}
