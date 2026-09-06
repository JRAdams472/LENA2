package userprefs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	userA := testenv.MustUser(ctx, t, pool, "prefs-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "prefs-b@example.com")
	itemID := createTestItem(ctx, t, pool)

	minQty1 := 1.0
	minQty2 := 2.0
	ui1, err := svc.UpsertUserItem(ctx, UserItem{
		UserID:     userA,
		ItemID:     itemID,
		CurrentQty: 5.0,
		MinQty:     &minQty1,
		Notes:      "first",
		IsFavorite: false,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, ui1.UserItemID)
	assert.Equal(t, userA, ui1.UserID)
	assert.Equal(t, itemID, ui1.ItemID)
	assert.InDelta(t, 5.0, ui1.CurrentQty, 0.0001)
	assert.Equal(t, "first", ui1.Notes)

	ui2, err := svc.UpsertUserItem(ctx, UserItem{
		UserID:     userA,
		ItemID:     itemID,
		CurrentQty: 10.0,
		MinQty:     &minQty2,
		Notes:      "second",
		IsFavorite: true,
	}, itBy)
	require.NoError(t, err)
	assert.Equal(t, ui1.UserItemID, ui2.UserItemID)
	assert.InDelta(t, 10.0, ui2.CurrentQty, 0.0001)
	assert.Equal(t, "second", ui2.Notes)
	assert.True(t, ui2.IsFavorite)

	got, err := svc.GetUserItemByID(ctx, ui1.UserItemID, userA)
	require.NoError(t, err)
	assert.Equal(t, ui1.UserItemID, got.UserItemID)
	assert.InDelta(t, 10.0, got.CurrentQty, 0.0001)

	items, err := svc.ListUserItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	_, err = svc.GetUserItemByID(ctx, ui1.UserItemID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	itemsB, err := svc.ListUserItems(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, itemsB)

	require.NoError(t, svc.DeleteUserItem(ctx, ui1.UserItemID, userB))
	items, err = svc.ListUserItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NoError(t, svc.DeleteUserItem(ctx, ui1.UserItemID, userA))
	items, err = svc.ListUserItems(ctx, userA, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestIntegrationUserBottleLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testenv.MustUser(ctx, t, pool, "bottle-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "bottle-b@example.com")
	bottleID := createTestBottle(ctx, t, pool)

	ub1, err := svc.UpsertUserBottle(ctx, UserBottle{
		UserID:     userA,
		BottleID:   bottleID,
		Quantity:   1,
		Notes:      "first",
		IsFavorite: false,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, ub1.UserBottleID)

	ub2, err := svc.UpsertUserBottle(ctx, UserBottle{
		UserID:     userA,
		BottleID:   bottleID,
		Quantity:   5,
		Notes:      "second",
		IsFavorite: true,
	}, itBy)
	require.NoError(t, err)
	assert.Equal(t, ub1.UserBottleID, ub2.UserBottleID)
	assert.Equal(t, int32(5), ub2.Quantity)
	assert.Equal(t, "second", ub2.Notes)
	assert.True(t, ub2.IsFavorite)

	got, err := svc.GetUserBottleByID(ctx, ub1.UserBottleID, userA)
	require.NoError(t, err)
	assert.Equal(t, ub1.UserBottleID, got.UserBottleID)
	assert.Equal(t, int32(5), got.Quantity)

	bottles, err := svc.ListUserBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, bottles, 1)

	_, err = svc.GetUserBottleByID(ctx, ub1.UserBottleID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	bottlesB, err := svc.ListUserBottles(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, bottlesB)

	require.NoError(t, svc.DeleteUserBottle(ctx, ub1.UserBottleID, userB))
	bottles, err = svc.ListUserBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, bottles, 1)

	require.NoError(t, svc.DeleteUserBottle(ctx, ub1.UserBottleID, userA))
	bottles, err = svc.ListUserBottles(ctx, userA, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, bottles)
}

func TestIntegrationRecipeFavoriteLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testenv.MustUser(ctx, t, pool, "fav-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "fav-b@example.com")
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
		SELECT count(*) FROM recipe.user_recipe_preference WHERE user_id = $1 AND recipe_id = $2
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
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	require.NoError(t, svc.DeleteRecipeFavorite(ctx, userA, recipeID))
	_, err = svc.GetRecipeFavorite(ctx, userA, recipeID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}
