package mealplan

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

const itBy = "integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool), pool
}

func TestIntegrationMealPlanLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	userA := testenv.MustUser(ctx, t, pool, "meal-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "meal-b@example.com")

	invSvc := inventory.NewService(pool)
	brand, err := invSvc.CreateBrand(ctx, "IT Meal Brand")
	require.NoError(t, err)
	cat, err := invSvc.CreateCategory(ctx, "IT Meal Category", "", itBy)
	require.NoError(t, err)
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Meal Item",
		BrandID:    &brand.BrandID,
		CategoryID: cat.CategoryID,
		Unit:       "g",
	}, itBy)
	require.NoError(t, err)

	recipeSvc := recipe.NewService(pool)
	rec, err := recipeSvc.CreateRecipe(ctx, recipe.Recipe{Name: "IT Meal Recipe", IsActive: true}, itBy)
	require.NoError(t, err)

	week := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	plan, err := svc.CreateMealPlan(ctx, MealPlan{
		UserID:             userA,
		Name:               "Week 37",
		WeekStartDate:      week,
		WeekStartDayOfWeek: 1,
		IsActive:           true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, plan.MealPlanID)
	assert.Equal(t, userA, plan.UserID)
	assert.Equal(t, "Week 37", plan.Name)
	assert.Equal(t, week, plan.WeekStartDate)
	assert.Equal(t, int16(1), plan.WeekStartDayOfWeek)
	assert.True(t, plan.IsActive)

	got, err := svc.GetMealPlanByID(ctx, plan.MealPlanID, userA)
	require.NoError(t, err)
	assert.Equal(t, plan.MealPlanID, got.MealPlanID)

	_, err = svc.GetMealPlanByID(ctx, plan.MealPlanID, userB)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	plansA, err := svc.ListMealPlans(ctx, userA, 100, 0)
	require.NoError(t, err)
	require.Len(t, plansA, 1)
	assert.Equal(t, plan.MealPlanID, plansA[0].MealPlanID)

	plansB, err := svc.ListMealPlans(ctx, userB, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, plansB)

	require.NoError(t, svc.UpdateMealPlan(ctx, plan.MealPlanID, userA, MealPlan{
		Name:               "Week 38",
		WeekStartDate:      week,
		WeekStartDayOfWeek: 2,
		IsActive:           false,
	}, itBy))
	updated, err := svc.GetMealPlanByID(ctx, plan.MealPlanID, userA)
	require.NoError(t, err)
	assert.Equal(t, "Week 38", updated.Name)
	assert.Equal(t, int16(2), updated.WeekStartDayOfWeek)
	assert.False(t, updated.IsActive)

	recipeID := rec.RecipeID
	servings := int32(4)
	slot, err := svc.AddMealSlot(ctx, MealSlot{
		MealPlanID: plan.MealPlanID,
		DayOfWeek:  1,
		MealType:   "dinner",
		RecipeID:   &recipeID,
		Servings:   &servings,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, slot.SlotID)
	assert.Equal(t, plan.MealPlanID, slot.MealPlanID)

	gotSlot, err := svc.GetMealSlotByID(ctx, slot.SlotID)
	require.NoError(t, err)
	assert.Equal(t, slot.SlotID, gotSlot.SlotID)
	require.NotNil(t, gotSlot.RecipeID)
	assert.Equal(t, recipeID, *gotSlot.RecipeID)

	slots, err := svc.ListMealSlotsForPlan(ctx, plan.MealPlanID)
	require.NoError(t, err)
	require.Len(t, slots, 1)

	itemID := item.ItemID
	slotItem, err := svc.AddMealSlotItem(ctx, MealSlotItem{
		SlotID:       slot.SlotID,
		ItemID:       &itemID,
		Quantity:     1.5,
		Unit:         "cup",
		IsFromRecipe: true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, slotItem.SlotItemID)

	items, err := svc.ListMealSlotItems(ctx, slot.SlotID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, slotItem.SlotItemID, items[0].SlotItemID)
	assert.InDelta(t, 1.5, items[0].Quantity, 0.0001)

	require.NoError(t, svc.DeleteMealSlotItem(ctx, slotItem.SlotItemID))
	items, err = svc.ListMealSlotItems(ctx, slot.SlotID)
	require.NoError(t, err)
	assert.Empty(t, items)

	slotItem2, err := svc.AddMealSlotItem(ctx, MealSlotItem{
		SlotID:   slot.SlotID,
		ItemID:   &itemID,
		Quantity: 2.0,
		Unit:     "tbsp",
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, slotItem2.SlotItemID)

	newServings := int32(2)
	require.NoError(t, svc.UpdateMealSlot(ctx, slot.SlotID, MealSlot{
		DayOfWeek:       2,
		MealType:        "lunch",
		RecipeID:        &recipeID,
		Servings:        &newServings,
		ReplacementNote: "no nuts",
	}, itBy))
	gotSlot, err = svc.GetMealSlotByID(ctx, slot.SlotID)
	require.NoError(t, err)
	assert.Equal(t, int16(2), gotSlot.DayOfWeek)
	assert.Equal(t, "lunch", gotSlot.MealType)
	assert.Equal(t, "no nuts", gotSlot.ReplacementNote)
	require.NotNil(t, gotSlot.Servings)
	assert.Equal(t, int32(2), *gotSlot.Servings)

	require.NoError(t, svc.DeleteMealPlan(ctx, plan.MealPlanID, userB))
	_, err = svc.GetMealPlanByID(ctx, plan.MealPlanID, userA)
	require.NoError(t, err, "plan should still exist after wrong-user delete")

	require.NoError(t, svc.DeleteMealSlot(ctx, slot.SlotID))
	_, err = svc.GetMealSlotByID(ctx, slot.SlotID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	items, err = svc.ListMealSlotItems(ctx, slot.SlotID)
	require.NoError(t, err)
	assert.Empty(t, items)

	slot2, err := svc.AddMealSlot(ctx, MealSlot{
		MealPlanID: plan.MealPlanID,
		DayOfWeek:  3,
		MealType:   "breakfast",
	}, itBy)
	require.NoError(t, err)
	_, err = svc.AddMealSlotItem(ctx, MealSlotItem{
		SlotID:   slot2.SlotID,
		ItemID:   &itemID,
		Quantity: 1.0,
		Unit:     "each",
	}, itBy)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteMealPlan(ctx, plan.MealPlanID, userA))
	_, err = svc.GetMealPlanByID(ctx, plan.MealPlanID, userA)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	slots, err = svc.ListMealSlotsForPlan(ctx, plan.MealPlanID)
	require.NoError(t, err)
	assert.Empty(t, slots)
}
