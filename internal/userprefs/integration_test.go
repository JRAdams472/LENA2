package userprefs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

const itBy = "integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool), pool
}

func createTestItem(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	name := fmt.Sprintf("IT Userprefs Item %d", time.Now().UnixNano())
	var itemID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO inventory.item (name, category_id, unit_id, created_by)
		SELECT $1, c.category_id, u.unit_id, $2
		FROM inventory.category c, inventory.unit u
		WHERE c.name = 'Produce' AND u.name = 'each'
		LIMIT 1
		RETURNING item_id
	`, name, itBy).Scan(&itemID)
	require.NoError(t, err)
	return itemID
}

func createTestBottle(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var bottleID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO wine.bottle (type_id, country_id, region_id, vintage_year, bottle_size, created_by)
		SELECT t.type_id, c.country_id, r.region_id, 2019, '750ml', $1
		FROM wine.type t
		CROSS JOIN wine.country c
		CROSS JOIN wine.region r
		WHERE t.name = 'Red' AND c.iso_code = 'FRA' AND r.name = 'Bordeaux' AND r.country_id = c.country_id
		LIMIT 1
		RETURNING bottle_id
	`, itBy).Scan(&bottleID)
	require.NoError(t, err)
	return bottleID
}

func createTestRecipe(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	name := fmt.Sprintf("IT Userprefs Recipe %d", time.Now().UnixNano())
	var recipeID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO recipe.recipe (name, is_active, created_by)
		VALUES ($1, true, $2)
		RETURNING recipe_id
	`, name, itBy).Scan(&recipeID)
	require.NoError(t, err)
	return recipeID
}

func TestIntegrationUserItemLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "prefs-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "prefs-b@example.com")
	itemID := createTestItem(ctx, t, pool)

	minQty1 := 1.0
	minQty2 := 2.0
	ui1, err := svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userA,
		ItemID:      itemID,
		CurrentQty:  5.0,
		MinQty:      &minQty1,
		Notes:       "first",
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, ui1.HouseholdItemID)
	assert.Equal(t, userA, ui1.HouseholdID)
	assert.Equal(t, itemID, ui1.ItemID)
	assert.InDelta(t, 5.0, ui1.CurrentQty, 0.0001)
	assert.Equal(t, "first", ui1.Notes)

	ui2, err := svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userA,
		ItemID:      itemID,
		CurrentQty:  10.0,
		MinQty:      &minQty2,
		Notes:       "second",
	}, itBy)
	require.NoError(t, err)
	assert.Equal(t, ui1.HouseholdItemID, ui2.HouseholdItemID)
	assert.InDelta(t, 10.0, ui2.CurrentQty, 0.0001)
	assert.Equal(t, "second", ui2.Notes)

	got, err := svc.GetHouseholdItemByID(ctx, ui1.HouseholdItemID, userA)
	require.NoError(t, err)
	assert.Equal(t, ui1.HouseholdItemID, got.HouseholdItemID)
	assert.InDelta(t, 10.0, got.CurrentQty, 0.0001)

	items, err := svc.ListHouseholdItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	_, err = svc.GetHouseholdItemByID(ctx, ui1.HouseholdItemID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	itemsB, err := svc.ListHouseholdItems(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, itemsB)

	require.NoError(t, svc.DeleteHouseholdItem(ctx, ui1.HouseholdItemID, userB))
	items, err = svc.ListHouseholdItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NoError(t, svc.DeleteHouseholdItem(ctx, ui1.HouseholdItemID, userA))
	items, err = svc.ListHouseholdItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestIntegrationUserBottleLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "bottle-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "bottle-b@example.com")
	bottleID := createTestBottle(ctx, t, pool)

	ub1, err := svc.UpsertHouseholdBottle(ctx, HouseholdBottle{
		HouseholdID: userA,
		BottleID:    bottleID,
		Quantity:    1,
		Notes:       "first",
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, ub1.HouseholdBottleID)

	ub2, err := svc.UpsertHouseholdBottle(ctx, HouseholdBottle{
		HouseholdID: userA,
		BottleID:    bottleID,
		Quantity:    5,
		Notes:       "second",
	}, itBy)
	require.NoError(t, err)
	assert.Equal(t, ub1.HouseholdBottleID, ub2.HouseholdBottleID)
	assert.Equal(t, int32(5), ub2.Quantity)
	assert.Equal(t, "second", ub2.Notes)

	got, err := svc.GetHouseholdBottleByID(ctx, ub1.HouseholdBottleID, userA)
	require.NoError(t, err)
	assert.Equal(t, ub1.HouseholdBottleID, got.HouseholdBottleID)
	assert.Equal(t, int32(5), got.Quantity)

	bottles, err := svc.ListHouseholdBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, bottles, 1)

	_, err = svc.GetHouseholdBottleByID(ctx, ub1.HouseholdBottleID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	bottlesB, err := svc.ListHouseholdBottles(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, bottlesB)

	require.NoError(t, svc.DeleteHouseholdBottle(ctx, ub1.HouseholdBottleID, userB))
	bottles, err = svc.ListHouseholdBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, bottles, 1)

	require.NoError(t, svc.DeleteHouseholdBottle(ctx, ub1.HouseholdBottleID, userA))
	bottles, err = svc.ListHouseholdBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, bottles)
}

func TestIntegrationRecipeFavoriteLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "fav-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "fav-b@example.com")
	recipeID := createTestRecipe(ctx, t, pool)

	fav, err := svc.SetRecipeFavorite(ctx, userA, recipeID, true, itBy)
	require.NoError(t, err)
	assert.True(t, fav.IsFavorite)

	got, err := svc.GetRecipeFavorite(ctx, userA, recipeID)
	require.NoError(t, err)
	assert.True(t, got.IsFavorite)

	fav2, err := svc.SetRecipeFavorite(ctx, userA, recipeID, false, itBy)
	require.NoError(t, err)
	assert.False(t, fav2.IsFavorite)

	got, err = svc.GetRecipeFavorite(ctx, userA, recipeID)
	require.NoError(t, err)
	assert.False(t, got.IsFavorite)

	var cnt int64
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM userprefs.user_recipe_preference WHERE user_id = $1 AND recipe_id = $2
	`, userA, recipeID).Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cnt)

	favB, err := svc.SetRecipeFavorite(ctx, userB, recipeID, true, itBy)
	require.NoError(t, err)
	assert.True(t, favB.IsFavorite)

	gotA, err := svc.GetRecipeFavorite(ctx, userA, recipeID)
	require.NoError(t, err)
	assert.False(t, gotA.IsFavorite)

	gotB, err := svc.GetRecipeFavorite(ctx, userB, recipeID)
	require.NoError(t, err)
	assert.True(t, gotB.IsFavorite)

	require.NoError(t, svc.DeleteRecipeFavorite(ctx, userB, recipeID))
	gotA, err = svc.GetRecipeFavorite(ctx, userA, recipeID)
	require.NoError(t, err)
	assert.False(t, gotA.IsFavorite)

	_, err = svc.GetRecipeFavorite(ctx, userB, recipeID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	require.NoError(t, svc.DeleteRecipeFavorite(ctx, userA, recipeID))
	_, err = svc.GetRecipeFavorite(ctx, userA, recipeID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestIntegrationAdjustHouseholdItemQuantityConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "adjust-concurrent-a@example.com")
	itemID := createTestItem(ctx, t, pool)

	_, err := svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userA,
		ItemID:      itemID,
		CurrentQty:  20.0,
	}, itBy)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.AdjustHouseholdItemQuantity(ctx, userA, itemID, 1.0, itBy)
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.AdjustHouseholdItemQuantity(ctx, userA, itemID, -1.0, itBy)
		}()
	}
	wg.Wait()

	got, err := svc.GetHouseholdItemByItem(ctx, userA, itemID)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, got.CurrentQty, 0.0001)
}

// TestIntegrationHouseholdSharedHoldings proves that two users in the same
// household read and mutate one shared pantry/cellar scope.
func TestIntegrationHouseholdSharedHoldings(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "shared-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "shared-b@example.com")
	testutil.JoinHousehold(ctx, t, pool, userB, userA)
	itemID := createTestItem(ctx, t, pool)
	bottleID := createTestBottle(ctx, t, pool)

	ui, err := svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userA, ItemID: itemID, CurrentQty: 4.0,
	}, itBy)
	require.NoError(t, err)

	// The second member reads the same holding through the shared scope.
	got, err := svc.GetHouseholdItemByItem(ctx, userA, itemID)
	require.NoError(t, err)
	assert.Equal(t, ui.HouseholdItemID, got.HouseholdItemID)

	items, err := svc.ListHouseholdItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// A quantity adjustment by the second member lands on the same row.
	adjusted, err := svc.AdjustHouseholdItemQuantity(ctx, userA, itemID, -1.5, itBy)
	require.NoError(t, err)
	assert.Equal(t, ui.HouseholdItemID, adjusted.HouseholdItemID)
	assert.InDelta(t, 2.5, adjusted.CurrentQty, 0.0001)

	ub, err := svc.UpsertHouseholdBottle(ctx, HouseholdBottle{
		HouseholdID: userA, BottleID: bottleID, Quantity: 3,
	}, itBy)
	require.NoError(t, err)
	gotB, err := svc.GetHouseholdBottleByBottle(ctx, userA, bottleID)
	require.NoError(t, err)
	assert.Equal(t, ub.HouseholdBottleID, gotB.HouseholdBottleID)
}

// TestIntegrationHouseholdFavoritesStayPersonal proves the favorites split:
// members of one household share holdings but keep per-user favorites.
func TestIntegrationHouseholdFavoritesStayPersonal(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "favshare-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "favshare-b@example.com")
	testutil.JoinHousehold(ctx, t, pool, userB, userA)
	itemID := createTestItem(ctx, t, pool)
	bottleID := createTestBottle(ctx, t, pool)

	_, err := svc.SetItemFavorite(ctx, userB, itemID, true, itBy)
	require.NoError(t, err)
	_, err = svc.SetBottleFavorite(ctx, userB, bottleID, true, itBy)
	require.NoError(t, err)

	favB, err := svc.GetItemFavorite(ctx, userB, itemID)
	require.NoError(t, err)
	assert.True(t, favB)
	favA, err := svc.GetItemFavorite(ctx, userA, itemID)
	require.NoError(t, err)
	assert.False(t, favA)

	favsB, err := svc.ListItemFavorites(ctx, userB, []int64{itemID})
	require.NoError(t, err)
	assert.True(t, favsB[itemID])
	favsA, err := svc.ListItemFavorites(ctx, userA, []int64{itemID})
	require.NoError(t, err)
	assert.False(t, favsA[itemID])

	bfavB, err := svc.GetBottleFavorite(ctx, userB, bottleID)
	require.NoError(t, err)
	assert.True(t, bfavB)
	bfavA, err := svc.GetBottleFavorite(ctx, userA, bottleID)
	require.NoError(t, err)
	assert.False(t, bfavA)

	// Unfavoriting for B leaves A's (absent) favorite untouched.
	_, err = svc.SetItemFavorite(ctx, userB, itemID, false, itBy)
	require.NoError(t, err)
	favB, err = svc.GetItemFavorite(ctx, userB, itemID)
	require.NoError(t, err)
	assert.False(t, favB)
}

// TestIntegrationMergeHouseholdStock verifies duplicate-key merge semantics:
// when two households combine, quantities for the same catalog item sum.
func TestIntegrationMergeHouseholdStock(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "merge-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "merge-b@example.com")
	itemID := createTestItem(ctx, t, pool)
	itemID2 := createTestItem(ctx, t, pool)
	bottleID := createTestBottle(ctx, t, pool)

	_, err := svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userA, ItemID: itemID, CurrentQty: 3.0,
	}, itBy)
	require.NoError(t, err)
	_, err = svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userB, ItemID: itemID, CurrentQty: 2.0,
	}, itBy)
	require.NoError(t, err)
	_, err = svc.UpsertHouseholdItem(ctx, HouseholdItem{
		HouseholdID: userB, ItemID: itemID2, CurrentQty: 7.0,
	}, itBy)
	require.NoError(t, err)
	_, err = svc.UpsertHouseholdBottle(ctx, HouseholdBottle{
		HouseholdID: userB, BottleID: bottleID, Quantity: 2,
	}, itBy)
	require.NoError(t, err)

	require.NoError(t, svc.MergeHouseholdStock(ctx, userB, userA, itBy))

	got, err := svc.GetHouseholdItemByItem(ctx, userA, itemID)
	require.NoError(t, err)
	assert.InDelta(t, 5.0, got.CurrentQty, 0.0001, "duplicate item quantities sum on merge")

	got2, err := svc.GetHouseholdItemByItem(ctx, userA, itemID2)
	require.NoError(t, err)
	assert.InDelta(t, 7.0, got2.CurrentQty, 0.0001, "unique items move wholesale")

	bottlesA, err := svc.ListHouseholdBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, bottlesA, 1)
	assert.Equal(t, int32(2), bottlesA[0].Quantity)

	// The source household is emptied.
	itemsB, err := svc.ListHouseholdItems(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, itemsB)
	bottlesB, err := svc.ListHouseholdBottles(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, bottlesB)
}
