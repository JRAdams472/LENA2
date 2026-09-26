package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/event/sqlc"
	"github.com/JRAdams472/LENA2/internal/event/sqlc/mock"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

var errDB = errors.New("db error")

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq}, mq
}

func TestCreateFoodEvent(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	in := FoodEvent{HouseholdID: 42, Name: "Halloween Party", EventDate: day, SlotGranularityMinutes: 15, IsActive: true}

	t.Run("success", func(t *testing.T) {
		s, mq := newService(t)
		want := sqlc.CreateFoodEventParams{
			HouseholdID:            42,
			Name:                   "Halloween Party",
			EventDate:              pgtype.Date{Time: day, Valid: true},
			SlotGranularityMinutes: 15,
			IsActive:               true,
			CreatedBy:              "tester",
			UpdatedBy:              pgtype.Text{String: "tester", Valid: true},
		}
		mq.EXPECT().CreateFoodEvent(ctx, want).Return(sqlc.EventFoodEvent{
			FoodEventID:            7,
			HouseholdID:            42,
			Name:                   "Halloween Party",
			EventDate:              pgtype.Date{Time: day, Valid: true},
			SlotGranularityMinutes: 15,
			IsActive:               true,
		}, nil)

		got, err := s.CreateFoodEvent(ctx, in, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.FoodEventID)
		assert.Equal(t, int64(42), got.HouseholdID)
		assert.Equal(t, "Halloween Party", got.Name)
		assert.Equal(t, day, got.EventDate)
		assert.Equal(t, int16(15), got.SlotGranularityMinutes)
		assert.True(t, got.IsActive)
	})

	t.Run("error", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateFoodEvent(ctx, gomock.Any()).Return(sqlc.EventFoodEvent{}, errDB)
		_, err := s.CreateFoodEvent(ctx, in, "tester")
		require.Error(t, err)
	})
}

func TestGetFoodEventByID(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	t.Run("found", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetFoodEventByID(ctx, sqlc.GetFoodEventByIDParams{FoodEventID: 7, HouseholdID: 42}).
			Return(sqlc.EventFoodEvent{FoodEventID: 7, HouseholdID: 42, Name: "Party", EventDate: pgtype.Date{Time: day, Valid: true}, SlotGranularityMinutes: 30, IsActive: true}, nil)

		got, err := s.GetFoodEventByID(ctx, 7, 42)
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.FoodEventID)
		assert.Equal(t, int16(30), got.SlotGranularityMinutes)
	})

	t.Run("not found maps to domain error", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetFoodEventByID(ctx, gomock.Any()).Return(sqlc.EventFoodEvent{}, pgx.ErrNoRows)
		_, err := s.GetFoodEventByID(ctx, 7, 42)
		require.ErrorIs(t, err, domainerr.ErrNotFound)
	})
}

func TestListFoodEvents(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	s, mq := newService(t)
	mq.EXPECT().ListFoodEvents(ctx, sqlc.ListFoodEventsParams{HouseholdID: 42, Limit: 10, Offset: 0}).
		Return([]sqlc.EventFoodEvent{
			{FoodEventID: 8, HouseholdID: 42, Name: "B", EventDate: pgtype.Date{Time: day, Valid: true}, SlotGranularityMinutes: 15, IsActive: true},
			{FoodEventID: 7, HouseholdID: 42, Name: "A", EventDate: pgtype.Date{Time: day, Valid: true}, SlotGranularityMinutes: 15, IsActive: false},
		}, nil)

	got, err := s.ListFoodEvents(ctx, 42, 10, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(8), got[0].FoodEventID)
	assert.False(t, got[1].IsActive)
}

func TestCountFoodEvents(t *testing.T) {
	ctx := context.Background()
	s, mq := newService(t)
	mq.EXPECT().CountFoodEvents(ctx, int64(42)).Return(int64(3), nil)
	got, err := s.CountFoodEvents(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(3), got)
}

func TestUpdateFoodEvent(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	s, mq := newService(t)
	want := sqlc.UpdateFoodEventParams{
		FoodEventID:            7,
		HouseholdID:            42,
		Name:                   "Renamed",
		EventDate:              pgtype.Date{Time: day, Valid: true},
		SlotGranularityMinutes: 30,
		IsActive:               false,
		UpdatedBy:              pgtype.Text{String: "tester", Valid: true},
	}
	mq.EXPECT().UpdateFoodEvent(ctx, want).Return(nil)

	err := s.UpdateFoodEvent(ctx, 7, 42, FoodEvent{Name: "Renamed", EventDate: day, SlotGranularityMinutes: 30}, "tester")
	require.NoError(t, err)
}

func TestDeleteFoodEvent(t *testing.T) {
	ctx := context.Background()
	s, mq := newService(t)
	mq.EXPECT().DeleteFoodEvent(ctx, sqlc.DeleteFoodEventParams{FoodEventID: 7, HouseholdID: 42}).Return(nil)
	require.NoError(t, s.DeleteFoodEvent(ctx, 7, 42))
}

func TestReassignHousehold(t *testing.T) {
	ctx := context.Background()

	t.Run("same id is a no-op", func(t *testing.T) {
		s, _ := newService(t)
		require.NoError(t, s.ReassignHousehold(ctx, 5, 5, "tester"))
	})

	t.Run("reassigns", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().ReassignFoodEventsToHousehold(ctx, sqlc.ReassignFoodEventsToHouseholdParams{
			ToHouseholdID: 9, UpdatedBy: "tester", FromHouseholdID: 5,
		}).Return(nil)
		require.NoError(t, s.ReassignHousehold(ctx, 5, 9, "tester"))
	})
}

func TestAddEventRecipe(t *testing.T) {
	ctx := context.Background()
	target := time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)
	recipeID := int64(11)
	servings := int32(8)

	t.Run("ownership check then insert", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetFoodEventByID(ctx, sqlc.GetFoodEventByIDParams{FoodEventID: 7, HouseholdID: 42}).
			Return(sqlc.EventFoodEvent{FoodEventID: 7, HouseholdID: 42}, nil)
		mq.EXPECT().AddEventRecipe(ctx, sqlc.AddEventRecipeParams{
			FoodEventID: 7,
			RecipeID:    pgtype.Int8{Int64: 11, Valid: true},
			MealType:    "dinner",
			TargetTime:  target,
			Servings:    pgtype.Int4{Int32: 8, Valid: true},
			Notes:       pgtype.Text{String: "serve hot", Valid: true},
			CreatedBy:   "tester",
			UpdatedBy:   pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.EventEventRecipe{
			EventRecipeID: 3,
			FoodEventID:   7,
			RecipeID:      pgtype.Int8{Int64: 11, Valid: true},
			MealType:      "dinner",
			TargetTime:    target,
			Servings:      pgtype.Int4{Int32: 8, Valid: true},
			Notes:         pgtype.Text{String: "serve hot", Valid: true},
		}, nil)

		got, err := s.AddEventRecipe(ctx, EventRecipe{
			FoodEventID: 7, RecipeID: &recipeID, MealType: "dinner",
			TargetTime: target, Servings: &servings, Notes: "serve hot",
		}, 42, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(3), got.EventRecipeID)
		assert.Equal(t, int64(11), *got.RecipeID)
		assert.Equal(t, int32(8), *got.Servings)
		assert.Equal(t, target, got.TargetTime)
		assert.Equal(t, "serve hot", got.Notes)
	})

	t.Run("wrong household fails before insert", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetFoodEventByID(ctx, gomock.Any()).Return(sqlc.EventFoodEvent{}, pgx.ErrNoRows)
		_, err := s.AddEventRecipe(ctx, EventRecipe{FoodEventID: 7, MealType: "dinner", TargetTime: target}, 42, "tester")
		require.ErrorIs(t, err, domainerr.ErrNotFound)
	})
}

func TestListEventRecipesByEvents(t *testing.T) {
	ctx := context.Background()
	target := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	s, mq := newService(t)
	mq.EXPECT().ListEventRecipesByEvents(ctx, sqlc.ListEventRecipesByEventsParams{FoodEventIds: []int64{7, 8}, HouseholdID: 42}).
		Return([]sqlc.EventEventRecipe{
			{EventRecipeID: 3, FoodEventID: 7, MealType: "dinner", TargetTime: target},
			{EventRecipeID: 4, FoodEventID: 8, MealType: "dessert", TargetTime: target},
		}, nil)

	got, err := s.ListEventRecipesByEvents(ctx, []int64{7, 8}, 42)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(7), got[0].FoodEventID)
	assert.Nil(t, got[0].RecipeID)
}

func TestUpdateEventRecipe(t *testing.T) {
	ctx := context.Background()
	target := time.Date(2026, 10, 31, 19, 0, 0, 0, time.UTC)
	recipeID := int64(11)
	s, mq := newService(t)
	mq.EXPECT().UpdateEventRecipe(ctx, sqlc.UpdateEventRecipeParams{
		EventRecipeID: 3,
		HouseholdID:   42,
		RecipeID:      pgtype.Int8{Int64: 11, Valid: true},
		MealType:      "dinner",
		TargetTime:    target,
		UpdatedBy:     pgtype.Text{String: "tester", Valid: true},
	}).Return(nil)

	err := s.UpdateEventRecipe(ctx, 3, 42, EventRecipe{RecipeID: &recipeID, MealType: "dinner", TargetTime: target}, "tester")
	require.NoError(t, err)
}

func TestDeleteEventRecipe(t *testing.T) {
	ctx := context.Background()
	s, mq := newService(t)
	mq.EXPECT().DeleteEventRecipe(ctx, sqlc.DeleteEventRecipeParams{EventRecipeID: 3, HouseholdID: 42}).Return(nil)
	require.NoError(t, s.DeleteEventRecipe(ctx, 3, 42))
}
