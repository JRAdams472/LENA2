package event

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
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

func TestIntegrationFoodEventLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	hhA := testutil.MustHousehold(ctx, t, pool)
	hhB := testutil.MustHousehold(ctx, t, pool)

	recipeSvc := recipe.NewService(pool)
	rec, err := recipeSvc.CreateRecipe(ctx, recipe.Recipe{Name: "IT Event Recipe", IsActive: true}, itBy)
	require.NoError(t, err)

	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	ev, err := svc.CreateFoodEvent(ctx, FoodEvent{
		HouseholdID:            hhA,
		Name:                   "Halloween Party",
		EventDate:              day,
		SlotGranularityMinutes: 15,
		IsActive:               true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, ev.FoodEventID)
	assert.Equal(t, hhA, ev.HouseholdID)
	assert.Equal(t, "Halloween Party", ev.Name)
	assert.Equal(t, day, ev.EventDate)
	assert.Equal(t, int16(15), ev.SlotGranularityMinutes)
	assert.True(t, ev.IsActive)

	got, err := svc.GetFoodEventByID(ctx, ev.FoodEventID, hhA)
	require.NoError(t, err)
	assert.Equal(t, ev.FoodEventID, got.FoodEventID)

	_, err = svc.GetFoodEventByID(ctx, ev.FoodEventID, hhB)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	listA, err := svc.ListFoodEvents(ctx, hhA, 100, 0)
	require.NoError(t, err)
	require.Len(t, listA, 1)
	listB, err := svc.ListFoodEvents(ctx, hhB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, listB)

	total, err := svc.CountFoodEvents(ctx, hhA)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	require.NoError(t, svc.UpdateFoodEvent(ctx, ev.FoodEventID, hhA, FoodEvent{
		Name:                   "Renamed Party",
		EventDate:              day,
		SlotGranularityMinutes: 30,
		IsActive:               false,
	}, itBy))
	got, err = svc.GetFoodEventByID(ctx, ev.FoodEventID, hhA)
	require.NoError(t, err)
	assert.Equal(t, "Renamed Party", got.Name)
	assert.Equal(t, int16(30), got.SlotGranularityMinutes)
	assert.False(t, got.IsActive)

	recipeID := rec.RecipeID
	servings := int32(8)
	target := time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)
	er, err := svc.AddEventRecipe(ctx, EventRecipe{
		FoodEventID: ev.FoodEventID,
		RecipeID:    &recipeID,
		MealType:    "dinner",
		TargetTime:  target,
		Servings:    &servings,
		Notes:       "serve hot",
	}, hhA, itBy)
	require.NoError(t, err)
	require.NotZero(t, er.EventRecipeID)
	assert.Equal(t, ev.FoodEventID, er.FoodEventID)
	require.NotNil(t, er.RecipeID)
	assert.Equal(t, recipeID, *er.RecipeID)
	assert.Equal(t, target.UTC(), er.TargetTime.UTC())

	gotER, err := svc.GetEventRecipeByID(ctx, er.EventRecipeID, hhA)
	require.NoError(t, err)
	assert.Equal(t, er.EventRecipeID, gotER.EventRecipeID)
	_, err = svc.GetEventRecipeByID(ctx, er.EventRecipeID, hhB)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	recipes, err := svc.ListEventRecipesForEvent(ctx, ev.FoodEventID, hhA)
	require.NoError(t, err)
	require.Len(t, recipes, 1)

	byEvents, err := svc.ListEventRecipesByEvents(ctx, []int64{ev.FoodEventID}, hhA)
	require.NoError(t, err)
	require.Len(t, byEvents, 1)
	byEventsB, err := svc.ListEventRecipesByEvents(ctx, []int64{ev.FoodEventID}, hhB)
	require.NoError(t, err)
	assert.Empty(t, byEventsB)

	newTarget := time.Date(2026, 10, 31, 19, 0, 0, 0, time.UTC)
	require.NoError(t, svc.UpdateEventRecipe(ctx, er.EventRecipeID, hhA, EventRecipe{
		RecipeID:   &recipeID,
		MealType:   "dessert",
		TargetTime: newTarget,
		Servings:   &servings,
		Notes:      "chilled",
	}, itBy))
	gotER, err = svc.GetEventRecipeByID(ctx, er.EventRecipeID, hhA)
	require.NoError(t, err)
	assert.Equal(t, "dessert", gotER.MealType)
	assert.Equal(t, newTarget.UTC(), gotER.TargetTime.UTC())
	assert.Equal(t, "chilled", gotER.Notes)

	require.NoError(t, svc.DeleteEventRecipe(ctx, er.EventRecipeID, hhA))
	recipes, err = svc.ListEventRecipesForEvent(ctx, ev.FoodEventID, hhA)
	require.NoError(t, err)
	assert.Empty(t, recipes)

	require.NoError(t, svc.DeleteFoodEvent(ctx, ev.FoodEventID, hhA))
	_, err = svc.GetFoodEventByID(ctx, ev.FoodEventID, hhA)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

// TestIntegrationFoodEventCrossHouseholdDenied verifies event and
// event-recipe operations are scoped to the owning household.
func TestIntegrationFoodEventCrossHouseholdDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	hhA := testutil.MustHousehold(ctx, t, pool)
	hhB := testutil.MustHousehold(ctx, t, pool)

	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	ev, err := svc.CreateFoodEvent(ctx, FoodEvent{
		HouseholdID: hhA, Name: "Owned", EventDate: day, SlotGranularityMinutes: 15, IsActive: true,
	}, itBy)
	require.NoError(t, err)

	target := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	er, err := svc.AddEventRecipe(ctx, EventRecipe{
		FoodEventID: ev.FoodEventID, MealType: "dinner", TargetTime: target,
	}, hhA, itBy)
	require.NoError(t, err)

	_, err = svc.AddEventRecipe(ctx, EventRecipe{
		FoodEventID: ev.FoodEventID, MealType: "lunch", TargetTime: target,
	}, hhB, itBy)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)

	recipes, err := svc.ListEventRecipesForEvent(ctx, ev.FoodEventID, hhB)
	require.NoError(t, err)
	assert.Empty(t, recipes)

	require.NoError(t, svc.UpdateEventRecipe(ctx, er.EventRecipeID, hhB, EventRecipe{MealType: "x", TargetTime: target}, itBy))
	got, err := svc.GetEventRecipeByID(ctx, er.EventRecipeID, hhA)
	require.NoError(t, err)
	assert.Equal(t, "dinner", got.MealType, "wrong-household update must not mutate the row")

	require.NoError(t, svc.DeleteEventRecipe(ctx, er.EventRecipeID, hhB))
	recipes, err = svc.ListEventRecipesForEvent(ctx, ev.FoodEventID, hhA)
	require.NoError(t, err)
	assert.Len(t, recipes, 1, "wrong-household delete must not remove the row")

	require.NoError(t, svc.DeleteFoodEvent(ctx, ev.FoodEventID, hhB))
	_, err = svc.GetFoodEventByID(ctx, ev.FoodEventID, hhA)
	require.NoError(t, err, "wrong-household delete must not remove the event")
}

// TestIntegrationReassignHousehold verifies events follow the merge.
func TestIntegrationReassignHousehold(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	hhA := testutil.MustHousehold(ctx, t, pool)
	hhB := testutil.MustHousehold(ctx, t, pool)

	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	ev, err := svc.CreateFoodEvent(ctx, FoodEvent{
		HouseholdID: hhA, Name: "Move Me", EventDate: day, SlotGranularityMinutes: 15, IsActive: true,
	}, itBy)
	require.NoError(t, err)

	require.NoError(t, svc.ReassignHousehold(ctx, hhA, hhB, itBy))

	_, err = svc.GetFoodEventByID(ctx, ev.FoodEventID, hhA)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
	got, err := svc.GetFoodEventByID(ctx, ev.FoodEventID, hhB)
	require.NoError(t, err)
	assert.Equal(t, hhB, got.HouseholdID)

	// Same-id reassign is a no-op, not an error.
	require.NoError(t, svc.ReassignHousehold(ctx, hhB, hhB, itBy))
}
