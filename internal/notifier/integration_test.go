package notifier_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/notifier"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

const itBy = "it-notifier@example.com"

func itUnitID(t *testing.T, ctx context.Context, invSvc *inventory.Service, name string) int64 {
	t.Helper()
	u, err := invSvc.GetUnitByName(ctx, name)
	require.NoError(t, err, "unit %q should be seeded by migration 0012", name)
	return u.UnitID
}

// TestIntegrationSweep seeds a two-member household with a protein recipe
// and a multi-day step on this week's meal plan plus an expiring pantry
// item, runs the sweep, and verifies dedup'd per-member reminders.
func TestIntegrationSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)

	owner := testutil.MustUser(ctx, t, pool, "notify-owner@example.com")
	member := testutil.MustUser(ctx, t, pool, "notify-member@example.com")
	testutil.JoinHousehold(ctx, t, pool, member, owner)

	invSvc := inventory.NewService(pool)
	brand, err := invSvc.CreateBrand(ctx, "IT Notify Brand", itBy)
	require.NoError(t, err)
	proteinCat, err := invSvc.CreateCategory(ctx, "IT Protein", "", true, itBy)
	require.NoError(t, err)
	netWeight := 100.0
	proteinItem, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Roast Beef",
		BrandID:    &brand.BrandID,
		CategoryID: proteinCat.CategoryID,
		UnitID:     itUnitID(t, ctx, invSvc, "g"),
		NetWeight:  &netWeight,
		IsMetric:   true,
	}, itBy)
	require.NoError(t, err)
	pantryCat, err := invSvc.CreateCategory(ctx, "IT Pantry", "", false, itBy)
	require.NoError(t, err)
	pantryItem, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Milk",
		BrandID:    &brand.BrandID,
		CategoryID: pantryCat.CategoryID,
		UnitID:     itUnitID(t, ctx, invSvc, "each"),
	}, itBy)
	require.NoError(t, err)

	// ~10 lb of protein -> 3-day lead; a 3-day step -> 3-day lead. Both
	// become due the day before a meal two days out.
	grams := itUnitID(t, ctx, invSvc, "g")
	threeDays := int32(4320)
	recipeSvc := recipe.NewService(pool)
	rec, err := recipeSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{
		Name: "IT Big Roast", Servings: intPtr4(4), IsActive: true,
	}, []recipe.RecipeItem{{
		ItemID: proteinItem.ItemID, Quantity: 4536, UnitID: grams,
	}}, []recipe.RecipeStep{{
		StepNumber: 1, Instruction: "Brine three days", DurationMinutes: &threeDays,
	}}, itBy)
	require.NoError(t, err)

	now := time.Now()
	mealDate := now.AddDate(0, 0, 2)
	weekStart := mealDate.AddDate(0, 0, -((int(mealDate.Weekday()) + 6) % 7)) // Monday
	mealSvc := mealplan.NewService(pool)
	plan, err := mealSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		HouseholdID:        owner,
		Name:               "IT Notify Week",
		WeekStartDate:      weekStart,
		WeekStartDayOfWeek: 1,
		IsActive:           true,
	}, itBy)
	require.NoError(t, err)
	servings := int32(4)
	_, err = mealSvc.AddMealSlot(ctx, mealplan.MealSlot{
		MealPlanID: plan.MealPlanID,
		//nolint:gosec // Weekday is bounded [0,6].
		DayOfWeek: int16(mealDate.Weekday()),
		MealType:  "dinner",
		RecipeID:  &rec.RecipeID,
		Servings:  &servings,
	}, owner, itBy)
	require.NoError(t, err)

	prefsSvc := userprefs.NewService(pool)
	expires := now.AddDate(0, 0, 2)
	_, err = prefsSvc.UpsertHouseholdItem(ctx, userprefs.HouseholdItem{
		HouseholdID: owner, ItemID: pantryItem.ItemID, CurrentQty: 1, ExpiresAt: &expires,
	}, itBy)
	require.NoError(t, err)

	svc := notifier.NewService(pool, notifier.Config{ExpiryDays: 3})

	created, err := svc.Sweep(ctx, now)
	require.NoError(t, err)
	// 3 reminder kinds x 2 household members.
	assert.Equal(t, 6, created)

	countKinds := func() map[string]int {
		rows, err := pool.Query(ctx,
			`SELECT kind, COUNT(*) FROM household.notifications
			 WHERE household_id = $1 GROUP BY kind ORDER BY kind`, owner)
		require.NoError(t, err)
		defer rows.Close()
		out := map[string]int{}
		for rows.Next() {
			var k string
			var c int
			require.NoError(t, rows.Scan(&k, &c))
			out[k] = c
		}
		return out
	}
	kinds := countKinds()
	assert.Equal(t, 2, kinds["item_expiring"])
	assert.Equal(t, 2, kinds["meal_prep_advance"])
	assert.Equal(t, 2, kinds["protein_defrost"])

	// Reminder rows carry server-rendered text and the deep-link FKs.
	var title, body string
	var recipeID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT title, body, recipe_id FROM household.notifications
		 WHERE kind = 'protein_defrost' AND user_id = $1`, owner).
		Scan(&title, &body, &recipeID))
	assert.NotEmpty(t, title)
	assert.NotEmpty(t, body)
	assert.Equal(t, rec.RecipeID, recipeID)

	// Re-sweeping the same period creates nothing — dedup keys hold.
	again, err := svc.Sweep(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 0, again)

	// Opting out of the expiry category suppresses future expiry reminders
	// for that member only.
	require.NoError(t, svc.SetCategoryEnabled(ctx, member, "expiry", false))
	newExpiry := expires.AddDate(0, 0, 1)
	_, err = pool.Exec(ctx,
		`UPDATE userprefs.household_item SET expires_at = $1
		 WHERE household_id = $2 AND item_id = $3`, newExpiry, owner, pantryItem.ItemID)
	require.NoError(t, err)
	more, err := svc.Sweep(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 1, more) // owner only

	var memberExpiry int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM household.notifications
		 WHERE kind = 'item_expiring' AND user_id = $1`, member).Scan(&memberExpiry))
	assert.Equal(t, 1, memberExpiry) // only the first sweep's row — mute held
}

func intPtr4(v int32) *int32 { return &v }

func TestMain(m *testing.M) {
	os.Exit(testutil.SharedDBTestMain(m))
}
