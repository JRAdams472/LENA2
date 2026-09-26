package event

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
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

// TestIntegrationEventRecipeSteps exercises the per-slot step snapshot:
// materialize a recipe's steps into the slot, edit them without touching
// shared recipe rows, batch-load, and delete.
func TestIntegrationEventRecipeSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	hhA := testutil.MustHousehold(ctx, t, pool)
	hhB := testutil.MustHousehold(ctx, t, pool)

	recipeSvc := recipe.NewService(pool)
	dur := int32(30)
	rec, err := recipeSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{Name: "IT Snapshot Recipe", IsActive: true},
		nil, []recipe.RecipeStep{
			{StepNumber: 1, Instruction: "mix", DurationMinutes: &dur},
			{StepNumber: 2, Instruction: "bake", DurationMinutes: &dur, Appliance: "oven"},
		}, itBy)
	require.NoError(t, err)

	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	ev, err := svc.CreateFoodEvent(ctx, FoodEvent{
		HouseholdID: hhA, Name: "Party", EventDate: day, SlotGranularityMinutes: 15, IsActive: true,
	}, itBy)
	require.NoError(t, err)
	slot, err := svc.AddEventRecipe(ctx, EventRecipe{
		FoodEventID: ev.FoodEventID, RecipeID: &rec.RecipeID, MealType: "dinner",
		TargetTime: time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC),
	}, hhA, itBy)
	require.NoError(t, err)

	// Materialize the snapshot — mirrors what the BFF does on add.
	steps, err := recipeSvc.ListRecipeStepsByRecipes(ctx, []int64{rec.RecipeID})
	require.NoError(t, err)
	snap := make([]EventRecipeStep, len(steps))
	for i, s := range steps {
		snap[i] = EventRecipeStep{
			StepNumber: s.StepNumber, Instruction: s.Instruction,
			DurationMinutes: s.DurationMinutes, StepType: s.StepType,
			IsPassive: s.IsPassive, DependsOnStepNumber: s.DependsOnStepNumber,
			Appliance: s.Appliance,
		}
	}
	require.NoError(t, svc.ReplaceEventRecipeSteps(ctx, slot.EventRecipeID, hhA, snap, itBy))

	got, err := svc.ListEventRecipeSteps(ctx, slot.EventRecipeID, hhA)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "mix", got[0].Instruction)

	// Batch load by event.
	batch, err := svc.ListEventRecipeStepsForEvents(ctx, []int64{ev.FoodEventID}, hhA)
	require.NoError(t, err)
	assert.Len(t, batch, 2)
	batchB, err := svc.ListEventRecipeStepsForEvents(ctx, []int64{ev.FoodEventID}, hhB)
	require.NoError(t, err)
	assert.Empty(t, batchB)

	// Editing the snapshot must not alter the shared recipe step.
	dur45 := int32(45)
	require.NoError(t, svc.UpdateEventRecipeStep(ctx, got[0].EventRecipeStepID, hhA,
		EventRecipeStep{Instruction: "mix longer", DurationMinutes: &dur45}, itBy))
	orig, err := recipeSvc.ListRecipeStepsByRecipes(ctx, []int64{rec.RecipeID})
	require.NoError(t, err)
	assert.Equal(t, "mix", orig[0].Instruction)

	// Add a hand-entered step (gets the next step number), then remove it.
	added, err := svc.AddEventRecipeStep(ctx, EventRecipeStep{EventRecipeID: slot.EventRecipeID, Instruction: "rest"}, hhA, itBy)
	require.NoError(t, err)
	assert.Equal(t, int32(3), added.StepNumber)
	require.NoError(t, svc.DeleteEventRecipeStep(ctx, added.EventRecipeStepID, hhA))
	got, err = svc.ListEventRecipeSteps(ctx, slot.EventRecipeID, hhA)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	// Cross-household writes denied.
	err = svc.ReplaceEventRecipeSteps(ctx, slot.EventRecipeID, hhB, snap, itBy)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

// TestIntegrationEventRecipeItems exercises the per-slot ingredient
// snapshot: materialize a recipe's items with the frozen base_servings
// denominator, edit them without touching shared recipe rows, batch-load,
// and delete.
func TestIntegrationEventRecipeItems(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	hhA := testutil.MustHousehold(ctx, t, pool)
	hhB := testutil.MustHousehold(ctx, t, pool)

	invSvc := inventory.NewService(pool)
	recipeSvc := recipe.NewService(pool)
	cat, err := invSvc.CreateCategory(ctx, "IT Event Item Cat", "", itBy)
	require.NoError(t, err)
	unit, err := invSvc.GetUnitByName(ctx, "cup")
	require.NoError(t, err)
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name: "IT Event Flour", CategoryID: cat.CategoryID, UnitID: unit.UnitID,
	}, itBy)
	require.NoError(t, err)

	servings := int32(4)
	rec, err := recipeSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{
		Name: "IT Item Snapshot Recipe", Servings: &servings, IsActive: true,
	}, []recipe.RecipeItem{
		{ItemID: item.ItemID, Quantity: 2, UnitID: unit.UnitID},
	}, nil, itBy)
	require.NoError(t, err)

	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	ev, err := svc.CreateFoodEvent(ctx, FoodEvent{
		HouseholdID: hhA, Name: "Party", EventDate: day, SlotGranularityMinutes: 15, IsActive: true,
	}, itBy)
	require.NoError(t, err)
	slot, err := svc.AddEventRecipe(ctx, EventRecipe{
		FoodEventID: ev.FoodEventID, RecipeID: &rec.RecipeID, MealType: "dinner",
		TargetTime: time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC), Servings: &[]int32{8}[0],
	}, hhA, itBy)
	require.NoError(t, err)

	// Materialize the snapshot — mirrors what the BFF does on add.
	src, err := recipeSvc.ListRecipeItemsByRecipes(ctx, []int64{rec.RecipeID})
	require.NoError(t, err)
	snap := make([]EventRecipeItem, len(src))
	for i, s := range src {
		snap[i] = EventRecipeItem{
			ItemID: s.ItemID, Quantity: s.Quantity, UnitID: s.UnitID,
		}
	}
	require.NoError(t, svc.ReplaceEventRecipeItems(ctx, slot.EventRecipeID, hhA, snap, rec.Servings, itBy))

	got, err := svc.ListEventRecipeItems(ctx, slot.EventRecipeID, hhA)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2.0, got[0].Quantity)

	// The slot now carries the frozen denominator — 8 over 4 scales ×2.
	loaded, err := svc.GetEventRecipeByID(ctx, slot.EventRecipeID, hhA)
	require.NoError(t, err)
	require.NotNil(t, loaded.BaseServings)
	assert.Equal(t, int32(4), *loaded.BaseServings)
	assert.Equal(t, 2.0, loaded.ScalingFactor())

	// Batch load by event; other households see nothing.
	batch, err := svc.ListEventRecipeItemsForEvents(ctx, []int64{ev.FoodEventID}, hhA)
	require.NoError(t, err)
	assert.Len(t, batch, 1)
	batchB, err := svc.ListEventRecipeItemsForEvents(ctx, []int64{ev.FoodEventID}, hhB)
	require.NoError(t, err)
	assert.Empty(t, batchB)

	// Editing the snapshot must not alter the shared recipe item.
	require.NoError(t, svc.UpdateEventRecipeItem(ctx, got[0].EventRecipeItemID, hhA,
		EventRecipeItem{ItemID: item.ItemID, Quantity: 5, UnitID: unit.UnitID}, itBy))
	orig, err := recipeSvc.ListRecipeItemsByRecipes(ctx, []int64{rec.RecipeID})
	require.NoError(t, err)
	assert.Equal(t, 2.0, orig[0].Quantity)

	// Add a hand-entered item, then remove it.
	added, err := svc.AddEventRecipeItem(ctx, EventRecipeItem{
		EventRecipeID: slot.EventRecipeID, ItemID: item.ItemID, Quantity: 1, UnitID: unit.UnitID,
	}, hhA, itBy)
	require.NoError(t, err)
	require.NoError(t, svc.DeleteEventRecipeItem(ctx, added.EventRecipeItemID, hhA))
	got, err = svc.ListEventRecipeItems(ctx, slot.EventRecipeID, hhA)
	require.NoError(t, err)
	assert.Len(t, got, 1)

	// Cross-household writes denied.
	err = svc.ReplaceEventRecipeItems(ctx, slot.EventRecipeID, hhB, snap, rec.Servings, itBy)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}
