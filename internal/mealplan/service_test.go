package mealplan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/mealplan/sqlc"
	"github.com/JRAdams472/LENA2/internal/mealplan/sqlc/mock"
)

var errDB = errors.New("db error")

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq}, mq
}

func TestCreateMealPlan(t *testing.T) {
	ctx := context.Background()
	week := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	in := MealPlan{UserID: 42, Name: "Week 37", WeekStartDate: week, WeekStartDayOfWeek: 1, IsActive: true}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		want := sqlc.CreateMealPlanParams{
			UserID:             42,
			Name:               "Week 37",
			WeekStartDate:      pgtype.Date{Time: week, Valid: true},
			WeekStartDayOfWeek: 1,
			IsActive:           true,
			CreatedBy:          "tester",
			UpdatedBy:          pgtype.Text{String: "tester", Valid: true},
		}
		mq.EXPECT().CreateMealPlan(ctx, want).Return(sqlc.MealplanMealPlan{
			MealPlanID:         7,
			UserID:             42,
			Name:               "Week 37",
			WeekStartDate:      pgtype.Date{Time: week, Valid: true},
			WeekStartDayOfWeek: 1,
			IsActive:           true,
		}, nil)

		got, err := s.CreateMealPlan(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.MealPlanID)
		assert.Equal(t, int64(42), got.UserID)
		assert.Equal(t, "Week 37", got.Name)
		assert.Equal(t, week, got.WeekStartDate)
		assert.Equal(t, int16(1), got.WeekStartDayOfWeek)
		assert.True(t, got.IsActive)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateMealPlan(ctx, gomock.Any()).Return(sqlc.MealplanMealPlan{}, errDB)
		_, err := s.CreateMealPlan(ctx, in, "tester")
		assert.ErrorContains(t, err, "create meal plan")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetMealPlanByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success threads userID", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetMealPlanByID(ctx, sqlc.GetMealPlanByIDParams{MealPlanID: 7, UserID: 42}).
			Return(sqlc.MealplanMealPlan{MealPlanID: 7, UserID: 42, Name: "Week 37"}, nil)

		got, err := s.GetMealPlanByID(ctx, 7, 42)
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.MealPlanID)
		assert.Equal(t, int64(42), got.UserID)
		assert.Equal(t, "Week 37", got.Name)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetMealPlanByID(ctx, gomock.Any()).Return(sqlc.MealplanMealPlan{}, errDB)
		_, err := s.GetMealPlanByID(ctx, 7, 42)
		assert.ErrorContains(t, err, "get meal plan")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListMealPlans(t *testing.T) {
	ctx := context.Background()

	t.Run("success threads userID and paging", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListMealPlans(ctx, sqlc.ListMealPlansParams{UserID: 42, Limit: 10, Offset: 5}).
			Return([]sqlc.MealplanMealPlan{
				{MealPlanID: 7, UserID: 42, Name: "A"},
				{MealPlanID: 8, UserID: 42, Name: "B"},
			}, nil)

		got, err := s.ListMealPlans(ctx, 42, 10, 5)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "A", got[0].Name)
		assert.Equal(t, "B", got[1].Name)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListMealPlans(ctx, gomock.Any()).Return(nil, errDB)
		_, err := s.ListMealPlans(ctx, 42, 10, 0)
		assert.ErrorContains(t, err, "list meal plans")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUpdateMealPlan(t *testing.T) {
	ctx := context.Background()
	week := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	in := MealPlan{Name: "Renamed", WeekStartDate: week, WeekStartDayOfWeek: 2, IsActive: false}

	t.Run("success threads userID", func(t *testing.T) {
		s, mq := newService(t)
		want := sqlc.UpdateMealPlanParams{
			MealPlanID:         7,
			UserID:             42,
			Name:               "Renamed",
			WeekStartDate:      pgtype.Date{Time: week, Valid: true},
			WeekStartDayOfWeek: 2,
			IsActive:           false,
			UpdatedBy:          pgtype.Text{String: "tester", Valid: true},
		}
		mq.EXPECT().UpdateMealPlan(ctx, want).Return(nil)

		require.NoError(t, s.UpdateMealPlan(ctx, 7, 42, in, "tester"))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpdateMealPlan(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.UpdateMealPlan(ctx, 7, 42, in, "tester"), errDB)
	})
}

func TestDeleteMealPlan(t *testing.T) {
	ctx := context.Background()

	t.Run("success threads userID", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteMealPlan(ctx, sqlc.DeleteMealPlanParams{MealPlanID: 7, UserID: 42}).Return(nil)
		require.NoError(t, s.DeleteMealPlan(ctx, 7, 42))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteMealPlan(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.DeleteMealPlan(ctx, 7, 42), errDB)
	})
}

func TestAddMealSlot(t *testing.T) {
	ctx := context.Background()
	recipeID, servings := int64(99), int32(4)
	in := MealSlot{MealPlanID: 7, DayOfWeek: 3, MealType: "dinner", RecipeID: &recipeID, Servings: &servings, ReplacementNote: "no nuts"}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		want := sqlc.AddMealSlotParams{
			MealPlanID:      7,
			DayOfWeek:       3,
			MealType:        "dinner",
			RecipeID:        pgtype.Int8{Int64: 99, Valid: true},
			Servings:        pgtype.Int4{Int32: 4, Valid: true},
			ReplacementNote: pgtype.Text{String: "no nuts", Valid: true},
			CreatedBy:       "tester",
			UpdatedBy:       pgtype.Text{String: "tester", Valid: true},
		}
		mq.EXPECT().AddMealSlot(ctx, want).Return(sqlc.MealplanMealSlot{
			SlotID:          55,
			MealPlanID:      7,
			DayOfWeek:       3,
			MealType:        "dinner",
			RecipeID:        pgtype.Int8{Int64: 99, Valid: true},
			Servings:        pgtype.Int4{Int32: 4, Valid: true},
			ReplacementNote: pgtype.Text{String: "no nuts", Valid: true},
		}, nil)

		got, err := s.AddMealSlot(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(55), got.SlotID)
		assert.Equal(t, int64(7), got.MealPlanID)
		assert.Equal(t, "dinner", got.MealType)
		require.NotNil(t, got.RecipeID)
		assert.Equal(t, int64(99), *got.RecipeID)
		require.NotNil(t, got.Servings)
		assert.Equal(t, int32(4), *got.Servings)
		assert.Equal(t, "no nuts", got.ReplacementNote)
	})

	t.Run("nil optionals become null params", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().AddMealSlot(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, arg sqlc.AddMealSlotParams) (sqlc.MealplanMealSlot, error) {
				assert.False(t, arg.RecipeID.Valid)
				assert.False(t, arg.Servings.Valid)
				assert.False(t, arg.ReplacementNote.Valid)
				return sqlc.MealplanMealSlot{SlotID: 56, MealPlanID: arg.MealPlanID}, nil
			})

		got, err := s.AddMealSlot(ctx, MealSlot{MealPlanID: 7, MealType: "lunch"}, "tester")
		require.NoError(t, err)
		assert.Nil(t, got.RecipeID)
		assert.Nil(t, got.Servings)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().AddMealSlot(ctx, gomock.Any()).Return(sqlc.MealplanMealSlot{}, errDB)
		_, err := s.AddMealSlot(ctx, in, "tester")
		assert.ErrorContains(t, err, "add meal slot")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetMealSlotByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetMealSlotByID(ctx, int64(55)).
			Return(sqlc.MealplanMealSlot{SlotID: 55, MealPlanID: 7, MealType: "lunch"}, nil)

		got, err := s.GetMealSlotByID(ctx, 55)
		require.NoError(t, err)
		assert.Equal(t, int64(55), got.SlotID)
		assert.Equal(t, "lunch", got.MealType)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetMealSlotByID(ctx, gomock.Any()).Return(sqlc.MealplanMealSlot{}, errDB)
		_, err := s.GetMealSlotByID(ctx, 55)
		assert.ErrorContains(t, err, "get meal slot")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListMealSlotsForPlan(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListMealSlotsForPlan(ctx, int64(7)).
			Return([]sqlc.MealplanMealSlot{
				{SlotID: 55, MealPlanID: 7, MealType: "lunch"},
				{SlotID: 56, MealPlanID: 7, MealType: "dinner"},
			}, nil)

		got, err := s.ListMealSlotsForPlan(ctx, 7)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "lunch", got[0].MealType)
		assert.Equal(t, "dinner", got[1].MealType)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListMealSlotsForPlan(ctx, gomock.Any()).Return(nil, errDB)
		_, err := s.ListMealSlotsForPlan(ctx, 7)
		assert.ErrorContains(t, err, "list meal slots")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUpdateMealSlot(t *testing.T) {
	ctx := context.Background()
	recipeID := int64(99)
	in := MealSlot{DayOfWeek: 4, MealType: "dinner", RecipeID: &recipeID, ReplacementNote: "spicy"}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		want := sqlc.UpdateMealSlotParams{
			SlotID:          55,
			DayOfWeek:       4,
			MealType:        "dinner",
			RecipeID:        pgtype.Int8{Int64: 99, Valid: true},
			Servings:        pgtype.Int4{},
			ReplacementNote: pgtype.Text{String: "spicy", Valid: true},
			UpdatedBy:       pgtype.Text{String: "tester", Valid: true},
		}
		mq.EXPECT().UpdateMealSlot(ctx, want).Return(nil)

		require.NoError(t, s.UpdateMealSlot(ctx, 55, in, "tester"))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpdateMealSlot(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.UpdateMealSlot(ctx, 55, in, "tester"), errDB)
	})
}

func TestDeleteMealSlot(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteMealSlot(ctx, int64(55)).Return(nil)
		require.NoError(t, s.DeleteMealSlot(ctx, 55))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteMealSlot(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.DeleteMealSlot(ctx, 55), errDB)
	})
}

func TestAddMealSlotItem(t *testing.T) {
	ctx := context.Background()
	itemID := int64(123)
	in := MealSlotItem{SlotID: 55, ItemID: &itemID, Quantity: 2.5, Unit: "cups", IsFromRecipe: true}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().AddMealSlotItem(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, arg sqlc.AddMealSlotItemParams) (sqlc.MealplanMealSlotItem, error) {
				assert.Equal(t, int64(55), arg.SlotID)
				assert.Equal(t, pgtype.Int8{Int64: 123, Valid: true}, arg.ItemID)
				f8, err := arg.Quantity.Float64Value()
				require.NoError(t, err)
				assert.InDelta(t, 2.5, f8.Float64, 1e-9)
				assert.Equal(t, "cups", arg.Unit)
				assert.True(t, arg.IsFromRecipe)
				assert.Equal(t, "tester", arg.CreatedBy)
				return sqlc.MealplanMealSlotItem{
					SlotItemID:   900,
					SlotID:       arg.SlotID,
					ItemID:       arg.ItemID,
					Quantity:     arg.Quantity,
					Unit:         arg.Unit,
					IsFromRecipe: arg.IsFromRecipe,
				}, nil
			})

		got, err := s.AddMealSlotItem(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(900), got.SlotItemID)
		assert.Equal(t, int64(55), got.SlotID)
		require.NotNil(t, got.ItemID)
		assert.Equal(t, int64(123), *got.ItemID)
		assert.InDelta(t, 2.5, got.Quantity, 1e-9)
		assert.Equal(t, "cups", got.Unit)
		assert.True(t, got.IsFromRecipe)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().AddMealSlotItem(ctx, gomock.Any()).Return(sqlc.MealplanMealSlotItem{}, errDB)
		_, err := s.AddMealSlotItem(ctx, in, "tester")
		assert.ErrorContains(t, err, "add meal slot item")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestListMealSlotItems(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListMealSlotItems(ctx, int64(55)).
			Return([]sqlc.MealplanMealSlotItem{
				{SlotItemID: 900, SlotID: 55, Unit: "cups"},
				{SlotItemID: 901, SlotID: 55, Unit: "g"},
			}, nil)

		got, err := s.ListMealSlotItems(ctx, 55)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(900), got[0].SlotItemID)
		assert.Equal(t, "g", got[1].Unit)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ListMealSlotItems(ctx, gomock.Any()).Return(nil, errDB)
		_, err := s.ListMealSlotItems(ctx, 55)
		assert.ErrorContains(t, err, "list meal slot items")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestDeleteMealSlotItem(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteMealSlotItem(ctx, int64(900)).Return(nil)
		require.NoError(t, s.DeleteMealSlotItem(ctx, 900))
	})

	t.Run("error propagates", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().DeleteMealSlotItem(ctx, gomock.Any()).Return(errDB)
		assert.ErrorIs(t, s.DeleteMealSlotItem(ctx, 900), errDB)
	})
}
