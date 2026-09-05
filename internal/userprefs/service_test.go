package userprefs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/userprefs/sqlc"
	"github.com/JRAdams472/LENA2/internal/userprefs/sqlc/mock"
)

var errDB = errors.New("db error")

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq}, mq
}

func numericVal(t *testing.T, n pgtype.Numeric) float64 {
	t.Helper()
	f8, err := n.Float64Value()
	require.NoError(t, err)
	return f8.Float64
}

func TestUpsertUserItem(t *testing.T) {
	ctx := context.Background()
	minQty := 1.5
	purchaseAt := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)
	in := UserItem{
		UserID:     10,
		ItemID:     20,
		CurrentQty: 3.25,
		MinQty:     &minQty,
		PurchaseAt: &purchaseAt,
		ExpiresAt:  &expiresAt,
		Notes:      "keep stocked",
		IsFavorite: true,
	}

	t.Run("success passes params and maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUserItem(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserItemParams) (sqlc.InventoryUserItem, error) {
				assert.Equal(t, int64(10), arg.UserID)
				assert.Equal(t, int64(20), arg.ItemID)
				assert.InDelta(t, 3.25, numericVal(t, arg.CurrentQty), 1e-9)
				assert.InDelta(t, 1.5, numericVal(t, arg.MinQty), 1e-9)
				assert.Equal(t, purchaseAt, arg.PurchaseAt.Time)
				assert.Equal(t, expiresAt, arg.ExpiresAt.Time)
				assert.Equal(t, pgtype.Text{String: "keep stocked", Valid: true}, arg.Notes)
				assert.True(t, arg.IsFavorite)
				assert.Equal(t, "tester", arg.CreatedBy)
				assert.Equal(t, pgtype.Text{String: "tester", Valid: true}, arg.UpdatedBy)
				return sqlc.InventoryUserItem{
					UserItemID: 7,
					UserID:     arg.UserID,
					ItemID:     arg.ItemID,
					CurrentQty: arg.CurrentQty,
					MinQty:     arg.MinQty,
					PurchaseAt: arg.PurchaseAt,
					ExpiresAt:  arg.ExpiresAt,
					Notes:      arg.Notes,
					IsFavorite: arg.IsFavorite,
				}, nil
			})

		got, err := svc.UpsertUserItem(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.UserItemID)
		assert.Equal(t, int64(10), got.UserID)
		assert.Equal(t, int64(20), got.ItemID)
		assert.InDelta(t, 3.25, got.CurrentQty, 1e-9)
		require.NotNil(t, got.MinQty)
		assert.InDelta(t, 1.5, *got.MinQty, 1e-9)
		require.NotNil(t, got.PurchaseAt)
		assert.Equal(t, purchaseAt, *got.PurchaseAt)
		require.NotNil(t, got.ExpiresAt)
		assert.Equal(t, expiresAt, *got.ExpiresAt)
		assert.Equal(t, "keep stocked", got.Notes)
		assert.True(t, got.IsFavorite)
	})

	t.Run("optional fields empty become invalid", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUserItem(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserItemParams) (sqlc.InventoryUserItem, error) {
				assert.False(t, arg.MinQty.Valid)
				assert.False(t, arg.PurchaseAt.Valid)
				assert.False(t, arg.ExpiresAt.Valid)
				assert.False(t, arg.Notes.Valid)
				assert.False(t, arg.UpdatedBy.Valid)
				return sqlc.InventoryUserItem{UserItemID: 8, UserID: arg.UserID, ItemID: arg.ItemID}, nil
			})

		got, err := svc.UpsertUserItem(ctx, UserItem{UserID: 10, ItemID: 20}, "")
		require.NoError(t, err)
		assert.Equal(t, int64(8), got.UserItemID)
		assert.Nil(t, got.MinQty)
		assert.Nil(t, got.PurchaseAt)
		assert.Nil(t, got.ExpiresAt)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUserItem(ctx, gomock.Any()).Return(sqlc.InventoryUserItem{}, errDB)

		_, err := svc.UpsertUserItem(ctx, in, "tester")
		require.Error(t, err)
		assert.ErrorContains(t, err, "upsert user item")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetUserItemByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success scopes to user", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserItemByID(ctx, sqlc.GetUserItemByIDParams{UserItemID: 7, UserID: 10}).
			Return(sqlc.InventoryUserItem{
				UserItemID: 7, UserID: 10, ItemID: 20,
				Notes:      pgtype.Text{String: "n", Valid: true},
				IsFavorite: true,
			}, nil)

		got, err := svc.GetUserItemByID(ctx, 7, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.UserItemID)
		assert.Equal(t, int64(10), got.UserID)
		assert.Equal(t, int64(20), got.ItemID)
		assert.Equal(t, "n", got.Notes)
		assert.True(t, got.IsFavorite)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserItemByID(ctx, sqlc.GetUserItemByIDParams{UserItemID: 7, UserID: 10}).
			Return(sqlc.InventoryUserItem{}, errDB)

		_, err := svc.GetUserItemByID(ctx, 7, 10)
		require.Error(t, err)
		assert.ErrorContains(t, err, "get user item")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListUserItems(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListUserItems(ctx, sqlc.ListUserItemsParams{UserID: 10, Limit: 50, Offset: 5}).
			Return([]sqlc.InventoryUserItem{
				{UserItemID: 1, UserID: 10, ItemID: 20},
				{UserItemID: 2, UserID: 10, ItemID: 21, IsFavorite: true},
			}, nil)

		got, err := svc.ListUserItems(ctx, 10, 50, 5)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(1), got[0].UserItemID)
		assert.Equal(t, int64(20), got[0].ItemID)
		assert.Equal(t, int64(2), got[1].UserItemID)
		assert.True(t, got[1].IsFavorite)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListUserItems(ctx, sqlc.ListUserItemsParams{UserID: 10, Limit: 50, Offset: 0}).
			Return(nil, errDB)

		_, err := svc.ListUserItems(ctx, 10, 50, 0)
		require.Error(t, err)
		assert.ErrorContains(t, err, "list user items")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestDeleteUserItem(t *testing.T) {
	ctx := context.Background()

	t.Run("success scopes to user", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteUserItem(ctx, sqlc.DeleteUserItemParams{UserItemID: 7, UserID: 10}).Return(nil)

		require.NoError(t, svc.DeleteUserItem(ctx, 7, 10))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteUserItem(ctx, sqlc.DeleteUserItemParams{UserItemID: 7, UserID: 10}).Return(errDB)

		err := svc.DeleteUserItem(ctx, 7, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUpsertUserBottle(t *testing.T) {
	ctx := context.Background()
	bottleNumber := int32(3)
	price := 42.50
	temp := 12.5
	purchaseAt := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	in := UserBottle{
		UserID:        10,
		BottleID:      30,
		BottleNumber:  &bottleNumber,
		Quantity:      2,
		PurchaseAt:    &purchaseAt,
		PurchasePrice: &price,
		StorageTemp:   &temp,
		Location:      "cellar",
		Notes:         "gift",
		IsFavorite:    true,
	}

	t.Run("success passes params and maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUserBottle(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserBottleParams) (sqlc.WineUserBottle, error) {
				assert.Equal(t, int64(10), arg.UserID)
				assert.Equal(t, int64(30), arg.BottleID)
				assert.Equal(t, pgtype.Int4{Int32: 3, Valid: true}, arg.BottleNumber)
				assert.Equal(t, int32(2), arg.Quantity)
				assert.Equal(t, purchaseAt, arg.PurchaseAt.Time)
				assert.InDelta(t, 42.50, numericVal(t, arg.PurchasePrice), 1e-9)
				assert.InDelta(t, 12.5, numericVal(t, arg.StorageTemp), 1e-9)
				assert.Equal(t, pgtype.Text{String: "cellar", Valid: true}, arg.Location)
				assert.Equal(t, pgtype.Text{String: "gift", Valid: true}, arg.Notes)
				assert.True(t, arg.IsFavorite)
				assert.Equal(t, "tester", arg.CreatedBy)
				assert.Equal(t, pgtype.Text{String: "tester", Valid: true}, arg.UpdatedBy)
				return sqlc.WineUserBottle{
					UserBottleID:  9,
					UserID:        arg.UserID,
					BottleID:      arg.BottleID,
					BottleNumber:  arg.BottleNumber,
					Quantity:      arg.Quantity,
					PurchaseAt:    arg.PurchaseAt,
					PurchasePrice: arg.PurchasePrice,
					StorageTemp:   arg.StorageTemp,
					Location:      arg.Location,
					Notes:         arg.Notes,
					IsFavorite:    arg.IsFavorite,
				}, nil
			})

		got, err := svc.UpsertUserBottle(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(9), got.UserBottleID)
		assert.Equal(t, int64(10), got.UserID)
		assert.Equal(t, int64(30), got.BottleID)
		require.NotNil(t, got.BottleNumber)
		assert.Equal(t, int32(3), *got.BottleNumber)
		assert.Equal(t, int32(2), got.Quantity)
		require.NotNil(t, got.PurchasePrice)
		assert.InDelta(t, 42.50, *got.PurchasePrice, 1e-9)
		require.NotNil(t, got.StorageTemp)
		assert.InDelta(t, 12.5, *got.StorageTemp, 1e-9)
		assert.Equal(t, "cellar", got.Location)
		assert.Equal(t, "gift", got.Notes)
		assert.True(t, got.IsFavorite)
	})

	t.Run("optional fields empty become invalid", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUserBottle(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserBottleParams) (sqlc.WineUserBottle, error) {
				assert.False(t, arg.BottleNumber.Valid)
				assert.False(t, arg.PurchaseAt.Valid)
				assert.False(t, arg.PurchasePrice.Valid)
				assert.False(t, arg.StorageTemp.Valid)
				assert.False(t, arg.Location.Valid)
				assert.False(t, arg.Notes.Valid)
				return sqlc.WineUserBottle{UserBottleID: 11, UserID: arg.UserID, BottleID: arg.BottleID}, nil
			})

		got, err := svc.UpsertUserBottle(ctx, UserBottle{UserID: 10, BottleID: 30, Quantity: 1}, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(11), got.UserBottleID)
		assert.Nil(t, got.BottleNumber)
		assert.Nil(t, got.PurchasePrice)
		assert.Nil(t, got.StorageTemp)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUserBottle(ctx, gomock.Any()).Return(sqlc.WineUserBottle{}, errDB)

		_, err := svc.UpsertUserBottle(ctx, in, "tester")
		require.Error(t, err)
		assert.ErrorContains(t, err, "upsert user bottle")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetUserBottleByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success scopes to user", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserBottleByID(ctx, sqlc.GetUserBottleByIDParams{UserBottleID: 9, UserID: 10}).
			Return(sqlc.WineUserBottle{
				UserBottleID: 9, UserID: 10, BottleID: 30, Quantity: 4,
				Location: pgtype.Text{String: "rack", Valid: true},
			}, nil)

		got, err := svc.GetUserBottleByID(ctx, 9, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(9), got.UserBottleID)
		assert.Equal(t, int64(10), got.UserID)
		assert.Equal(t, int64(30), got.BottleID)
		assert.Equal(t, int32(4), got.Quantity)
		assert.Equal(t, "rack", got.Location)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserBottleByID(ctx, sqlc.GetUserBottleByIDParams{UserBottleID: 9, UserID: 10}).
			Return(sqlc.WineUserBottle{}, errDB)

		_, err := svc.GetUserBottleByID(ctx, 9, 10)
		require.Error(t, err)
		assert.ErrorContains(t, err, "get user bottle")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListUserBottles(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListUserBottles(ctx, sqlc.ListUserBottlesParams{UserID: 10, Limit: 25, Offset: 0}).
			Return([]sqlc.WineUserBottle{
				{UserBottleID: 1, UserID: 10, BottleID: 30, Quantity: 1},
				{UserBottleID: 2, UserID: 10, BottleID: 31, Quantity: 6},
			}, nil)

		got, err := svc.ListUserBottles(ctx, 10, 25, 0)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(1), got[0].UserBottleID)
		assert.Equal(t, int64(30), got[0].BottleID)
		assert.Equal(t, int32(6), got[1].Quantity)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListUserBottles(ctx, sqlc.ListUserBottlesParams{UserID: 10, Limit: 25, Offset: 0}).
			Return(nil, errDB)

		_, err := svc.ListUserBottles(ctx, 10, 25, 0)
		require.Error(t, err)
		assert.ErrorContains(t, err, "list user bottles")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestDeleteUserBottle(t *testing.T) {
	ctx := context.Background()

	t.Run("success scopes to user", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteUserBottle(ctx, sqlc.DeleteUserBottleParams{UserBottleID: 9, UserID: 10}).Return(nil)

		require.NoError(t, svc.DeleteUserBottle(ctx, 9, 10))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteUserBottle(ctx, sqlc.DeleteUserBottleParams{UserBottleID: 9, UserID: 10}).Return(errDB)

		err := svc.DeleteUserBottle(ctx, 9, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestSetRecipeFavorite(t *testing.T) {
	ctx := context.Background()

	t.Run("success passes params and maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertRecipeFavorite(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertRecipeFavoriteParams) (sqlc.RecipeUserRecipePreference, error) {
				assert.Equal(t, int64(10), arg.UserID)
				assert.Equal(t, int64(40), arg.RecipeID)
				assert.True(t, arg.IsFavorite)
				assert.Equal(t, "tester", arg.CreatedBy)
				assert.Equal(t, pgtype.Text{String: "tester", Valid: true}, arg.UpdatedBy)
				return sqlc.RecipeUserRecipePreference{UserID: arg.UserID, RecipeID: arg.RecipeID, IsFavorite: arg.IsFavorite}, nil
			})

		got, err := svc.SetRecipeFavorite(ctx, 10, 40, true, "tester")
		require.NoError(t, err)
		assert.Equal(t, RecipeFavorite{UserID: 10, RecipeID: 40, IsFavorite: true}, got)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertRecipeFavorite(ctx, gomock.Any()).Return(sqlc.RecipeUserRecipePreference{}, errDB)

		_, err := svc.SetRecipeFavorite(ctx, 10, 40, true, "tester")
		require.Error(t, err)
		assert.ErrorContains(t, err, "set recipe favorite")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetRecipeFavorite(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipeFavorite(ctx, sqlc.GetRecipeFavoriteParams{UserID: 10, RecipeID: 40}).
			Return(sqlc.RecipeUserRecipePreference{UserID: 10, RecipeID: 40, IsFavorite: true}, nil)

		got, err := svc.GetRecipeFavorite(ctx, 10, 40)
		require.NoError(t, err)
		assert.Equal(t, RecipeFavorite{UserID: 10, RecipeID: 40, IsFavorite: true}, got)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipeFavorite(ctx, sqlc.GetRecipeFavoriteParams{UserID: 10, RecipeID: 40}).
			Return(sqlc.RecipeUserRecipePreference{}, errDB)

		_, err := svc.GetRecipeFavorite(ctx, 10, 40)
		require.Error(t, err)
		assert.ErrorContains(t, err, "get recipe favorite")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestDeleteRecipeFavorite(t *testing.T) {
	ctx := context.Background()

	t.Run("success scopes to user", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRecipeFavorite(ctx, sqlc.DeleteRecipeFavoriteParams{UserID: 10, RecipeID: 40}).Return(nil)

		require.NoError(t, svc.DeleteRecipeFavorite(ctx, 10, 40))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRecipeFavorite(ctx, sqlc.DeleteRecipeFavoriteParams{UserID: 10, RecipeID: 40}).Return(errDB)

		err := svc.DeleteRecipeFavorite(ctx, 10, 40)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
	})
}
