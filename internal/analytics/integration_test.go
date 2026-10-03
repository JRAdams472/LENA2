package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

const itBy = "analytics-integration-test"

func ptrInt64(v int64) *int64 { return &v }

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)
	return NewService(pool, Config{}), pool
}

func TestIntegrationRecordEventAndCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)
	userID := testutil.MustUser(ctx, t, pool, "analytics-a@example.com")

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
	userA := testutil.MustUser(ctx, t, pool, "overlap-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "overlap-b@example.com")

	invSvc := inventory.NewService(pool)
	recSvc := recipe.NewService(pool)
	mpSvc := mealplan.NewService(pool)

	cat, err := invSvc.CreateCategory(ctx, "IT Overlap Category", "", false, itBy)
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
		{ItemID: ptrInt64(i1.ItemID), Quantity: 1, UnitID: gID.UnitID},
		{ItemID: ptrInt64(i2.ItemID), Quantity: 1, UnitID: gID.UnitID},
	}, nil, itBy)
	require.NoError(t, err)
	planA, err := mpSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		HouseholdID: userA, Name: "IT Overlap Plan A", WeekStartDate: time.Now(), IsActive: true,
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
		{ItemID: ptrInt64(i9.ItemID), Quantity: 1, UnitID: gID.UnitID},
		{ItemID: ptrInt64(i10.ItemID), Quantity: 1, UnitID: gID.UnitID},
	}, nil, itBy)
	require.NoError(t, err)
	planB, err := mpSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		HouseholdID: userB, Name: "IT Overlap Plan B", WeekStartDate: time.Now(), IsActive: true,
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
		{ItemID: ptrInt64(i1.ItemID), Quantity: 1, UnitID: gID.UnitID},
		{ItemID: ptrInt64(i2.ItemID), Quantity: 1, UnitID: gID.UnitID},
		{ItemID: ptrInt64(i3.ItemID), Quantity: 1, UnitID: gID.UnitID},
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

func TestIntegrationDecayAndEntityEngagement(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	// Two members of one household plus a loner in another.
	hh := testutil.MustHousehold(ctx, t, pool)
	userA := testutil.MustUser(ctx, t, pool, "decay-a@example.com")
	testutil.JoinHousehold(ctx, t, pool, userA, hh)
	userB := testutil.MustUser(ctx, t, pool, "decay-b@example.com")
	testutil.JoinHousehold(ctx, t, pool, userB, hh)
	userC := testutil.MustUser(ctx, t, pool, "decay-c@example.com")

	rec := func(userID int64, eventType, entityType string, entityID int64) {
		require.NoError(t, svc.RecordEvent(ctx, Event{
			UserID: userID, EventType: eventType, EntityType: entityType, EntityID: entityID,
		}, itBy))
	}

	// A: selects item 1 twice, searches "milk", views item 3.
	rec(userA, EventItemSelected, EntityItem, 1)
	rec(userA, EventItemSelected, EntityItem, 1)
	require.NoError(t, svc.RecordEvent(ctx, Event{
		UserID: userA, EventType: EventItemSearched, EntityType: EntityItem, SearchTerm: "milk",
	}, itBy))
	require.NoError(t, svc.RecordView(ctx, Event{
		UserID: userA, EventType: EventItemViewed, EntityType: EntityItem, EntityID: 3,
	}, itBy))
	// B: selects item 2 once — household scope should see items 1 and 2.
	rec(userB, EventItemSelected, EntityItem, 2)
	// C: selects item 4 in a different household — visible only globally.
	rec(userC, EventItemSelected, EntityItem, 4)

	require.NoError(t, svc.DecayScores(ctx))

	// Personal scope: A has item 1 only.
	personal, err := svc.TopScores(ctx, ScopeUser, userA, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, personal, 1)
	assert.Equal(t, int64(1), personal[0].EntityID)
	assert.InDelta(t, 2.0, personal[0].Score, 0.001) // two fresh selections × weight 1

	// Household scope aggregates members; searched/viewed events excluded.
	house, err := svc.TopScores(ctx, ScopeHousehold, hh, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, house, 2)
	assert.Equal(t, int64(1), house[0].EntityID) // score 2 beats score 1
	assert.Equal(t, int64(2), house[1].EntityID)

	// Global scope spans households.
	global, err := svc.TopScores(ctx, ScopeGlobal, 0, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, global, 3)

	eng, err := svc.EntityEngagementSets(ctx, userA, hh, EntityItem)
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, eng.PersonalIDs)
	assert.Equal(t, []int64{1, 2}, eng.HouseholdIDs)
	assert.Equal(t, []int64{3}, eng.ViewedIDs)
	assert.Equal(t, []string{"milk"}, eng.SearchTerms)

	// Backdate item 1's events by exactly one half-life: score halves.
	halfLife := svc.cfg.HalfLifeDays
	_, err = pool.Exec(ctx,
		`UPDATE analytics.interaction_event SET created_at = now() - make_interval(days => $1)
		 WHERE user_id = $2 AND entity_id = 1`, halfLife, userA)
	require.NoError(t, err)
	require.NoError(t, svc.DecayScores(ctx))
	personal, err = svc.TopScores(ctx, ScopeUser, userA, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, personal, 1)
	assert.InDelta(t, 1.0, personal[0].Score, 0.02) // ~2 × 2^-1

	// A second rebuild is a clean recompute, not an accumulation.
	global, err = svc.TopScores(ctx, ScopeGlobal, 0, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, global, 3)
}

func TestIntegrationHouseholdCountsAndVelocity(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)

	hh := testutil.MustHousehold(ctx, t, pool)
	userA := testutil.MustUser(ctx, t, pool, "hh-a@example.com")
	testutil.JoinHousehold(ctx, t, pool, userA, hh)
	userB := testutil.MustUser(ctx, t, pool, "hh-b@example.com")
	testutil.JoinHousehold(ctx, t, pool, userB, hh)
	other := testutil.MustUser(ctx, t, pool, "hh-other@example.com") // not in hh

	// Selection counts aggregate across household members only.
	for i := 0; i < 2; i++ {
		require.NoError(t, svc.RecordEvent(ctx, Event{
			UserID: userA, EventType: EventItemSelected, EntityType: EntityItem, EntityID: 10,
		}, itBy))
	}
	require.NoError(t, svc.RecordEvent(ctx, Event{
		UserID: userB, EventType: EventItemSelected, EntityType: EntityItem, EntityID: 10,
	}, itBy))
	require.NoError(t, svc.RecordEvent(ctx, Event{
		UserID: userB, EventType: EventItemSelected, EntityType: EntityItem, EntityID: 20,
	}, itBy))
	require.NoError(t, svc.RecordEvent(ctx, Event{
		UserID: other, EventType: EventItemSelected, EntityType: EntityItem, EntityID: 10,
	}, itBy))

	counts, err := svc.HouseholdSelectionCounts(ctx, hh, EntityItem, []int64{10, 20, 99})
	require.NoError(t, err)
	assert.Equal(t, int64(3), counts[10])
	assert.Equal(t, int64(1), counts[20])
	_, ok := counts[99]
	assert.False(t, ok)

	// Velocity: one backdated event leaves recent_count inside the window.
	require.NoError(t, svc.RecordEvent(ctx, Event{
		UserID: userA, EventType: EventRecipeSelected, EntityType: EntityRecipe, EntityID: 7,
	}, itBy))
	require.NoError(t, svc.RecordEvent(ctx, Event{
		UserID: userB, EventType: EventRecipeSelected, EntityType: EntityRecipe, EntityID: 7,
	}, itBy))
	_, err = pool.Exec(ctx,
		`UPDATE analytics.interaction_event SET created_at = now() - interval '60 days'
		 WHERE user_id = $1 AND entity_id = 7 AND entity_type = 'recipe'`, userA)
	require.NoError(t, err)

	velocities, err := svc.HouseholdRecipeVelocities(ctx, hh, 30)
	require.NoError(t, err)
	require.Len(t, velocities, 1)
	assert.Equal(t, int64(7), velocities[0].RecipeID)
	assert.Equal(t, int64(1), velocities[0].RecentCount)
	assert.Equal(t, int64(2), velocities[0].TotalCount)
	assert.GreaterOrEqual(t, velocities[0].AgeDays, 60.0)

	// A different household sees nothing.
	velocities, err = svc.HouseholdRecipeVelocities(ctx, 99999, 30)
	require.NoError(t, err)
	assert.Empty(t, velocities)
}

func TestMain(m *testing.M) {
	os.Exit(testutil.SharedDBTestMain(m))
}
