package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

const itBy = "analytics-integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool), pool
}

func TestIntegrationRecordEventAndCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)
	userID := testenv.MustUser(ctx, t, pool, "analytics-a@example.com")

	err := svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventItemSelected,
		EntityType: EntityItem,
		EntityID:   1,
	}, itBy)
	require.NoError(t, err)

	err = svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventItemSelected,
		EntityType: EntityItem,
		EntityID:   1,
	}, itBy)
	require.NoError(t, err)

	err = svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventBrandSelected,
		EntityType: EntityBrand,
		EntityID:   2,
	}, itBy)
	require.NoError(t, err)

	err = svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventItemSearched,
		EntityType: EntityItem,
		SearchTerm: "milk",
	}, itBy)
	require.NoError(t, err)

	counts, err := svc.GetUserSelectionCounts(ctx, userID, EntityItem, []int64{1})
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, int64(2), counts[0].SelectCount)

	global, err := svc.GetGlobalSelectionCounts(ctx, EntityBrand, []int64{2})
	require.NoError(t, err)
	require.Len(t, global, 1)
	assert.Equal(t, int64(1), global[0].SelectCount)

	top, err := svc.TopUserSelections(ctx, userID, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, int64(1), top[0].EntityID)
}

func TestIntegrationIngredientOverlap(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)
	userA := testenv.MustUser(ctx, t, pool, "overlap-a@example.com")
	userB := testenv.MustUser(ctx, t, pool, "overlap-b@example.com")

	invSvc := inventory.NewService(pool)
	recSvc := recipe.NewService(pool)
	mpSvc := mealplan.NewService(pool)

	cat, err := invSvc.CreateCategory(ctx, "IT Overlap Category", "", itBy)
	require.NoError(t, err)
	gID, err := invSvc.GetUnitByName(ctx, "g")
	require.NoError(t, err)
	mkItem := func(name string) inventory.Item {
		it, err := invSvc.CreateItem(ctx, inventory.Item{
			Name: name, CategoryID: cat.CategoryID, UnitID: gID.UnitID,
		}, itBy)
		require.NoError(t, err)
		return it
	}
	i1, i2, i3 := mkItem("IT Overlap Item 1"), mkItem("IT Overlap Item 2"), mkItem("IT Overlap Item 3")
	i9, i10 := mkItem("IT Overlap Item 9"), mkItem("IT Overlap Item 10")

	// userA's menu history: a recipe sharing 2 of the new recipe's 3 items.
	histA, err := recSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{
		Name: "IT Overlap History A", IsActive: true,
	}, []recipe.RecipeItem{
		{ItemID: i1.ItemID, Quantity: 1, UnitID: gID.UnitID},
		{ItemID: i2.ItemID, Quantity: 1, UnitID: gID.UnitID},
	}, nil, itBy)
	require.NoError(t, err)
	planA, err := mpSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		UserID: userA, Name: "IT Overlap Plan A", WeekStartDate: time.Now(), IsActive: true,
	}, itBy)
	require.NoError(t, err)
	_, err = mpSvc.AddMealSlot(ctx, mealplan.MealSlot{
		MealPlanID: planA.MealPlanID, DayOfWeek: 1, MealType: "dinner", RecipeID: &histA.RecipeID,
	}, userA, itBy)
	require.NoError(t, err)

	// userB's menu history: a recipe sharing none of the new recipe's items.
	histB, err := recSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{
		Name: "IT Overlap History B", IsActive: true,
	}, []recipe.RecipeItem{
		{ItemID: i9.ItemID, Quantity: 1, UnitID: gID.UnitID},
		{ItemID: i10.ItemID, Quantity: 1, UnitID: gID.UnitID},
	}, nil, itBy)
	require.NoError(t, err)
	planB, err := mpSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		UserID: userB, Name: "IT Overlap Plan B", WeekStartDate: time.Now(), IsActive: true,
	}, itBy)
	require.NoError(t, err)
	_, err = mpSvc.AddMealSlot(ctx, mealplan.MealSlot{
		MealPlanID: planB.MealPlanID, DayOfWeek: 1, MealType: "dinner", RecipeID: &histB.RecipeID,
	}, userB, itBy)
	require.NoError(t, err)

	// New recipe shares items 1,2 with userA's history (Jaccard 2/3) and
	// nothing with userB's (0) — only userA gets a recommendation.
	newRec, err := recSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{
		Name: "IT Overlap New", IsActive: true,
	}, []recipe.RecipeItem{
		{ItemID: i1.ItemID, Quantity: 1, UnitID: gID.UnitID},
		{ItemID: i2.ItemID, Quantity: 1, UnitID: gID.UnitID},
		{ItemID: i3.ItemID, Quantity: 1, UnitID: gID.UnitID},
	}, nil, itBy)
	require.NoError(t, err)

	n, err := svc.ComputeIngredientOverlapSuggestions(ctx, newRec.RecipeID)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	recsA, err := svc.ListRecipeRecommendations(ctx, userA, ReasonIngredientOverlap, 10)
	require.NoError(t, err)
	require.Len(t, recsA, 1)
	assert.Equal(t, newRec.RecipeID, recsA[0].RecipeID)
	assert.InDelta(t, 2.0/3.0, recsA[0].Score, 0.001)

	recsB, err := svc.ListRecipeRecommendations(ctx, userB, ReasonIngredientOverlap, 10)
	require.NoError(t, err)
	assert.Empty(t, recsB)

	// Recomputing is idempotent: the upsert replaces the row.
	n, err = svc.ComputeIngredientOverlapSuggestions(ctx, newRec.RecipeID)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	recsA, err = svc.ListRecipeRecommendations(ctx, userA, ReasonIngredientOverlap, 10)
	require.NoError(t, err)
	assert.Len(t, recsA, 1)
}
