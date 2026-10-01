package grocery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

// TestIntegrationStoreRouting walks the whole routing path against real
// Postgres: store + aisle setup, assignments, check-off learning, manual
// reorder priority, store inheritance, and cross-user denial.
func TestIntegrationStoreRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "routing-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "routing-b@example.com")

	// Store and aisle layout.
	store, err := svc.CreateStore(ctx, userA, "IT Grocery", itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Grocery", store.Name)

	produce, err := svc.CreateAisle(ctx, store.StoreID, userA, "Produce", 0, itBy)
	require.NoError(t, err)
	dairy, err := svc.CreateAisle(ctx, store.StoreID, userA, "Dairy", 1, itBy)
	require.NoError(t, err)

	aisles, err := svc.ListAisles(ctx, store.StoreID, userA)
	require.NoError(t, err)
	require.Len(t, aisles, 2)
	assert.Equal(t, "Produce", aisles[0].Name)

	// Aisle reorder persists positions.
	require.NoError(t, svc.ReorderAisles(ctx, store.StoreID, userA, []int64{dairy.AisleID, produce.AisleID}, itBy))
	aisles, err = svc.ListAisles(ctx, store.StoreID, userA)
	require.NoError(t, err)
	assert.Equal(t, "Dairy", aisles[0].Name)
	assert.Equal(t, int32(0), aisles[0].Position)

	// Manual-name identity assignment normalizes case/space.
	require.NoError(t, svc.AssignToAisle(ctx, store.StoreID, userA,
		RouteIdentity{ManualName: "  Bananas "}, &produce.AisleID, itBy))
	assignments, err := svc.ListAssignments(ctx, store.StoreID, userA)
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, "bananas", assignments[0].Identity.ManualName)

	// A list with two manual items.
	list, err := svc.CreateGroceryList(ctx, userA, nil, itBy)
	require.NoError(t, err)
	require.NoError(t, svc.SetGroceryListStore(ctx, list.GroceryListID, userA, &store.StoreID, itBy))

	bananas, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID: list.GroceryListID, ManualItemName: "bananas",
		QuantityNeeded: 2, Source: "manual",
	}, userA, itBy)
	require.NoError(t, err)
	milk, err := svc.AddGroceryListItem(ctx, GroceryListItem{
		GroceryListID: list.GroceryListID, ManualItemName: "milk",
		QuantityNeeded: 1, Source: "manual",
	}, userA, itBy)
	require.NoError(t, err)

	// Assignment maps "bananas" to its aisle; "milk" is unassigned and has
	// no learned data, so it lands in the trailing bucket.
	groups, err := svc.RouteGroups(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "Produce", groups[0].Aisle.Name)
	assert.Equal(t, bananas.GroceryListItemID, groups[0].Items[0].Item.GroceryListItemID)
	assert.Nil(t, groups[1].Aisle)
	assert.Equal(t, milk.GroceryListItemID, groups[1].Items[0].Item.GroceryListItemID)

	// Check-offs learn normalized positions. bananas first (seq 1/2 = .5).
	got, err := svc.ToggleGroceryListItemChecked(ctx, bananas.GroceryListItemID, userA, itBy)
	require.NoError(t, err)
	require.True(t, got.IsChecked)
	got, err = svc.ToggleGroceryListItemChecked(ctx, milk.GroceryListItemID, userA, itBy)
	require.NoError(t, err)
	require.True(t, got.IsChecked)

	routes, err := svc.ListItemRoutes(ctx, store.StoreID, userA)
	require.NoError(t, err)
	byManual := make(map[string]ItemRoute)
	for _, rt := range routes {
		byManual[rt.Identity.ManualName] = rt
	}
	require.Len(t, byManual, 2)
	assert.InDelta(t, 0.5, byManual["bananas"].LearnedMean, 1e-9)
	assert.InDelta(t, 1.0, byManual["milk"].LearnedMean, 1e-9)
	assert.Equal(t, int32(1), byManual["milk"].LearnedCount)

	// Manual reorder overrides learned order: milk before bananas.
	require.NoError(t, svc.ReorderListItems(ctx, list.GroceryListID, userA, []ReorderEntry{
		{GroceryListItemID: milk.GroceryListItemID, AisleID: &dairy.AisleID},
		{GroceryListItemID: bananas.GroceryListItemID},
	}, itBy))

	groups, err = svc.RouteGroups(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "Dairy", groups[0].Aisle.Name, "aisle reorder: dairy first")
	assert.Equal(t, milk.GroceryListItemID, groups[0].Items[0].Item.GroceryListItemID)
	assert.Equal(t, "Produce", groups[1].Aisle.Name)
	assert.Equal(t, bananas.GroceryListItemID, groups[1].Items[0].Item.GroceryListItemID)

	// A new list inherits the household's most recently used store.
	list2, err := svc.CreateGroceryList(ctx, userA, nil, itBy)
	require.NoError(t, err)
	require.NotNil(t, list2.StoreID)
	assert.Equal(t, store.StoreID, *list2.StoreID)

	// Cross-user denial on every new surface.
	_, err = svc.GetStoreByID(ctx, store.StoreID, userB)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
	stores, err := svc.ListStores(ctx, userB)
	require.NoError(t, err)
	assert.Empty(t, stores)
	_, err = svc.CreateAisle(ctx, store.StoreID, userB, "Intruder", 0, itBy)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
	err = svc.AssignToAisle(ctx, store.StoreID, userB, RouteIdentity{ManualName: "x"}, &produce.AisleID, itBy)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
	_, err = svc.RouteGroups(ctx, list.GroceryListID, userB)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
	err = svc.ReorderListItems(ctx, list.GroceryListID, userB, []ReorderEntry{
		{GroceryListItemID: milk.GroceryListItemID},
	}, itBy)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	// Deleting the store cascades aisles/assignments/routes and detaches
	// the list via SET NULL.
	require.NoError(t, svc.DeleteStore(ctx, store.StoreID, userA))
	got2, err := svc.GetGroceryListByID(ctx, list.GroceryListID, userA)
	require.NoError(t, err)
	assert.Nil(t, got2.StoreID)
}
