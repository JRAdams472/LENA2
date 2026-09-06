package grocery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/grocery/sqlc"
	"github.com/JRAdams472/LENA2/internal/grocery/sqlc/mock"
)

var errDB = errors.New("db error")

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq}, mq
}

func TestCreateGroceryList(t *testing.T) {
	ctx := context.Background()
	mealPlanID := int64(7)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		want := sqlc.CreateGroceryListParams{
			UserID:     42,
			MealPlanID: pgtype.Int8{Int64: 7, Valid: true},
			CreatedBy:  "tester",
			UpdatedBy:  pgtype.Text{String: "tester", Valid: true},
		}
		mq.EXPECT().CreateGroceryList(ctx, want).Return(sqlc.GroceryGroceryList{
			GroceryListID: 3,
			UserID:        42,
			MealPlanID:    pgtype.Int8{Int64: 7, Valid: true},
			GeneratedAt:   now,
		}, nil)

		got, err := s.CreateGroceryList(ctx, 42, &mealPlanID, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(3), got.GroceryListID)
		assert.Equal(t, int64(42), got.UserID)
		require.NotNil(t, got.MealPlanID)
		assert.Equal(t, int64(7), *got.MealPlanID)
		assert.Equal(t, now, got.GeneratedAt)
	})

	t.Run("nil mealPlanID becomes null", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateGroceryList(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, arg sqlc.CreateGroceryListParams) (sqlc.GroceryGroceryList, error) {
				assert.False(t, arg.MealPlanID.Valid)
				return sqlc.GroceryGroceryList{GroceryListID: 4, UserID: arg.UserID}, nil
			})

		got, err := s.CreateGroceryList(ctx, 42, nil, "tester")
		require.NoError(t, err)
		assert.Nil(t, got.MealPlanID)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateGroceryList(ctx, gomock.Any()).Return(sqlc.GroceryGroceryList{}, errDB)
		_, err := s.CreateGroceryList(ctx, 42, &mealPlanID, "tester")
		assert.ErrorContains(t, err, "create grocery list")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetGroceryListByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success threads userID", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: 3, UserID: 42}).
			Return(sqlc.GroceryGroceryList{GroceryListID: 3, UserID: 42}, nil)

		got, err := s.GetGroceryListByID(ctx, 3, 42)
		require.NoError(t, err)
		assert.Equal(t, int64(3), got.GroceryListID)
		assert.Equal(t, int64(42), got.UserID)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetGroceryListByID(ctx, gomock.Any()).Return(sqlc.GroceryGroceryList{}, errDB)
		_, err := s.GetGroceryListByID(ctx, 3, 42)
		assert.ErrorContains(t, err, "get grocery list")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListGroceryLists(t *testing.T) {
	ctx := context.Background()

	t.Run("success threads userID and paging", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListGroceryLists(ctx, sqlc.ListGroceryListsParams{UserID: 42, Limit: 10, Offset: 5}).
			Return([]sqlc.GroceryGroceryList{
				{GroceryListID: 3, UserID: 42},
				{GroceryListID: 4, UserID: 42},
			}, nil)

		got, err := s.ListGroceryLists(ctx, 42, 10, 5)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(3), got[0].GroceryListID)
		assert.Equal(t, int64(4), got[1].GroceryListID)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListGroceryLists(ctx, gomock.Any()).Return(nil, errDB)
		_, err := s.ListGroceryLists(ctx, 42, 10, 0)
		assert.ErrorContains(t, err, "list grocery lists")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestDeleteGroceryList(t *testing.T) {
	ctx := context.Background()

	t.Run("success threads userID", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteGroceryList(ctx, sqlc.DeleteGroceryListParams{GroceryListID: 3, UserID: 42}).Return(nil)
		require.NoError(t, s.DeleteGroceryList(ctx, 3, 42))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteGroceryList(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.DeleteGroceryList(ctx, 3, 42), errDB)
	})
}

func TestAddGroceryListItem(t *testing.T) {
	ctx := context.Background()
	itemID := int64(123)
	in := GroceryListItem{
		GroceryListID:  3,
		ItemID:         &itemID,
		ManualItemName: "milk",
		QuantityNeeded: 2.5,
		UnitOfMeasure:  "liters",
		Source:         "manual",
		IsChecked:      false,
	}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().AddGroceryListItem(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, arg sqlc.AddGroceryListItemParams) (sqlc.GroceryGroceryListItem, error) {
				assert.Equal(t, int64(3), arg.GroceryListID)
				assert.Equal(t, pgtype.Int8{Int64: 123, Valid: true}, arg.ItemID)
				assert.Equal(t, pgtype.Text{String: "milk", Valid: true}, arg.ManualItemName)
				f8, err := arg.QuantityNeeded.Float64Value()
				require.NoError(t, err)
				assert.InDelta(t, 2.5, f8.Float64, 1e-9)
				assert.Equal(t, pgtype.Text{String: "liters", Valid: true}, arg.UnitOfMeasure)
				assert.Equal(t, "manual", arg.Source)
				assert.False(t, arg.IsChecked)
				assert.Equal(t, "tester", arg.CreatedBy)
				return sqlc.GroceryGroceryListItem{
					GroceryListItemID: 900,
					GroceryListID:     arg.GroceryListID,
					ItemID:            arg.ItemID,
					ManualItemName:    arg.ManualItemName,
					QuantityNeeded:    arg.QuantityNeeded,
					UnitOfMeasure:     arg.UnitOfMeasure,
					Source:            arg.Source,
				}, nil
			})

		got, err := s.AddGroceryListItem(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(900), got.GroceryListItemID)
		assert.Equal(t, int64(3), got.GroceryListID)
		require.NotNil(t, got.ItemID)
		assert.Equal(t, int64(123), *got.ItemID)
		assert.Equal(t, "milk", got.ManualItemName)
		assert.InDelta(t, 2.5, got.QuantityNeeded, 1e-9)
		assert.Equal(t, "liters", got.UnitOfMeasure)
		assert.Equal(t, "manual", got.Source)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().AddGroceryListItem(ctx, gomock.Any()).Return(sqlc.GroceryGroceryListItem{}, errDB)
		_, err := s.AddGroceryListItem(ctx, in, "tester")
		assert.ErrorContains(t, err, "add grocery list item")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListGroceryListItems(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListGroceryListItems(ctx, int64(3)).
			Return([]sqlc.GroceryGroceryListItem{
				{GroceryListItemID: 900, GroceryListID: 3, Source: "manual"},
				{GroceryListItemID: 901, GroceryListID: 3, Source: "recipe", IsChecked: true},
			}, nil)

		got, err := s.ListGroceryListItems(ctx, 3)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "manual", got[0].Source)
		assert.True(t, got[1].IsChecked)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListGroceryListItems(ctx, gomock.Any()).Return(nil, errDB)
		_, err := s.ListGroceryListItems(ctx, 3)
		assert.ErrorContains(t, err, "list grocery list items")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetGroceryListItemByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetGroceryListItemByID(ctx, int64(900)).
			Return(sqlc.GroceryGroceryListItem{
				GroceryListItemID: 900,
				GroceryListID:     3,
				ManualItemName:    pgtype.Text{String: "milk", Valid: true},
				IsChecked:         true,
			}, nil)

		got, err := s.GetGroceryListItemByID(ctx, 900)
		require.NoError(t, err)
		assert.Equal(t, int64(900), got.GroceryListItemID)
		assert.Equal(t, "milk", got.ManualItemName)
		assert.True(t, got.IsChecked)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetGroceryListItemByID(ctx, gomock.Any()).Return(sqlc.GroceryGroceryListItem{}, errDB)
		_, err := s.GetGroceryListItemByID(ctx, 900)
		assert.ErrorContains(t, err, "get grocery list item")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUpdateGroceryListItem(t *testing.T) {
	ctx := context.Background()
	itemID := int64(123)
	in := GroceryListItem{
		ItemID:         &itemID,
		ManualItemName: "eggs",
		QuantityNeeded: 12,
		UnitOfMeasure:  "pcs",
		Source:         "manual",
		IsChecked:      true,
	}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpdateGroceryListItem(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, arg sqlc.UpdateGroceryListItemParams) error {
				assert.Equal(t, int64(900), arg.GroceryListItemID)
				assert.Equal(t, pgtype.Int8{Int64: 123, Valid: true}, arg.ItemID)
				assert.Equal(t, pgtype.Text{String: "eggs", Valid: true}, arg.ManualItemName)
				f8, err := arg.QuantityNeeded.Float64Value()
				require.NoError(t, err)
				assert.InDelta(t, 12.0, f8.Float64, 1e-9)
				assert.True(t, arg.IsChecked)
				assert.Equal(t, pgtype.Text{String: "tester", Valid: true}, arg.UpdatedBy)
				return nil
			})

		require.NoError(t, s.UpdateGroceryListItem(ctx, 900, in, "tester"))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpdateGroceryListItem(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.UpdateGroceryListItem(ctx, 900, in, "tester"), errDB)
	})
}

func TestDeleteGroceryListItem(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteGroceryListItem(ctx, int64(900)).Return(nil)
		require.NoError(t, s.DeleteGroceryListItem(ctx, 900))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteGroceryListItem(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.DeleteGroceryListItem(ctx, 900), errDB)
	})
}

func TestGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("creates list linked to meal plan", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateGroceryList(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, arg sqlc.CreateGroceryListParams) (sqlc.GroceryGroceryList, error) {
				assert.Equal(t, int64(42), arg.UserID)
				assert.Equal(t, pgtype.Int8{Int64: 7, Valid: true}, arg.MealPlanID)
				assert.Equal(t, "tester", arg.CreatedBy)
				return sqlc.GroceryGroceryList{
					GroceryListID: 3,
					UserID:        arg.UserID,
					MealPlanID:    arg.MealPlanID,
				}, nil
			})

		got, err := s.Generate(ctx, 42, 7, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(3), got.GroceryListID)
		require.NotNil(t, got.MealPlanID)
		assert.Equal(t, int64(7), *got.MealPlanID)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateGroceryList(ctx, gomock.Any()).Return(sqlc.GroceryGroceryList{}, errDB)
		_, err := s.Generate(ctx, 42, 7, "tester")
		assert.ErrorContains(t, err, "create grocery list")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestCountGroceryLists(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CountGroceryLists(ctx, int64(42)).Return(int64(5), nil)

		got, err := s.CountGroceryLists(ctx, 42)
		require.NoError(t, err)
		assert.Equal(t, int64(5), got)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CountGroceryLists(ctx, int64(42)).Return(int64(0), errDB)

		_, err := s.CountGroceryLists(ctx, 42)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "count grocery lists")
	})
}
