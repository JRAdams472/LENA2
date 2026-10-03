package inventory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

const itBy = "integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) *Service {
	t.Helper()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)
	return NewService(pool)
}

func newIntegrationServiceWithPool(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)
	return NewService(pool), pool
}

// unitID returns the id of a seeded unit by name or abbreviation.
func unitID(t *testing.T, ctx context.Context, svc *Service, name string) int64 {
	t.Helper()
	u, err := svc.GetUnitByName(ctx, name)
	require.NoError(t, err, "unit %q should be seeded by migration 0012", name)
	return u.UnitID
}

func TestIntegrationBrandCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	brand, err := svc.CreateBrand(ctx, "IT Brand Alpha", itBy)
	require.NoError(t, err)
	require.NotZero(t, brand.BrandID)
	assert.Equal(t, "IT Brand Alpha", brand.Name)

	got, err := svc.GetBrandByID(ctx, brand.BrandID)
	require.NoError(t, err)
	assert.Equal(t, brand.BrandID, got.BrandID)
	assert.Equal(t, "IT Brand Alpha", got.Name)

	brands, err := svc.ListBrands(ctx)
	require.NoError(t, err)
	var found bool
	for _, b := range brands {
		if b.BrandID == brand.BrandID {
			found = true
		}
	}
	assert.True(t, found, "created brand should appear in ListBrands")

	updated, err := svc.UpdateBrand(ctx, brand.BrandID, "IT Brand Beta")
	require.NoError(t, err)
	assert.Equal(t, "IT Brand Beta", updated.Name)

	require.NoError(t, svc.DeleteBrand(ctx, brand.BrandID))
	_, err = svc.GetBrandByID(ctx, brand.BrandID)
	assert.Error(t, err, "deleted brand should not be retrievable")
}

func TestIntegrationBrandDuplicateName(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	_, err := svc.CreateBrand(ctx, "IT Brand Dup", itBy)
	require.NoError(t, err)
	_, err = svc.CreateBrand(ctx, "IT Brand Dup", itBy)
	assert.Error(t, err, "duplicate brand name should violate unique constraint")
}

func TestIntegrationCategoryCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	cat, err := svc.CreateCategory(ctx, "IT Category Alpha", "test category", false, itBy)
	require.NoError(t, err)
	require.NotZero(t, cat.CategoryID)
	assert.Equal(t, "IT Category Alpha", cat.Name)
	assert.Equal(t, "test category", cat.Description)
	assert.True(t, cat.IsActive)

	got, err := svc.GetCategoryByID(ctx, cat.CategoryID)
	require.NoError(t, err)
	assert.Equal(t, "IT Category Alpha", got.Name)

	cats, err := svc.ListCategories(ctx)
	require.NoError(t, err)
	var found bool
	for _, c := range cats {
		if c.CategoryID == cat.CategoryID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateCategory(ctx, cat.CategoryID, "IT Category Beta", "updated", false, false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Category Beta", updated.Name)
	assert.Equal(t, "updated", updated.Description)
	assert.False(t, updated.IsActive)

	require.NoError(t, svc.DeleteCategory(ctx, cat.CategoryID))
	_, err = svc.GetCategoryByID(ctx, cat.CategoryID)
	assert.Error(t, err)
}

func TestIntegrationItemCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	cat, err := svc.CreateCategory(ctx, "IT Item Category", "", false, itBy)
	require.NoError(t, err)
	brand, err := svc.CreateBrand(ctx, "IT Item Brand", itBy)
	require.NoError(t, err)

	alphaWeight := 100.0
	item, err := svc.CreateItem(ctx, Item{
		Name:       "IT Item Alpha",
		BrandID:    &brand.BrandID,
		Upc12:      "123456789012",
		CategoryID: cat.CategoryID,
		UnitID:     unitID(t, ctx, svc, "g"),
		NetWeight:  &alphaWeight,
		IsMetric:   true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, item.ItemID)
	assert.Equal(t, "IT Item Alpha", item.Name)
	require.NotNil(t, item.BrandID)
	assert.Equal(t, brand.BrandID, *item.BrandID)
	assert.Equal(t, "123456789012", item.Upc12)

	got, err := svc.GetItemByID(ctx, item.ItemID)
	require.NoError(t, err)
	assert.Equal(t, item.ItemID, got.ItemID)

	byIDs, err := svc.GetItemsByIDs(ctx, []int64{item.ItemID})
	require.NoError(t, err)
	require.Len(t, byIDs, 1)
	assert.Equal(t, item.ItemID, byIDs[0].ItemID)

	ozID := unitID(t, ctx, svc, "oz")
	require.NoError(t, svc.UpdateItem(ctx, item.ItemID, Item{
		Name:       "IT Item Beta",
		BrandID:    &brand.BrandID,
		CategoryID: cat.CategoryID,
		UnitID:     ozID,
		NetWeight:  item.NetWeight,
		IsMetric:   item.IsMetric,
	}, itBy))
	got, err = svc.GetItemByID(ctx, item.ItemID)
	require.NoError(t, err)
	assert.Equal(t, "IT Item Beta", got.Name)
	assert.Equal(t, ozID, got.UnitID)

	// FK violation: item referencing a non-existent category.
	_, err = svc.CreateItem(ctx, Item{
		Name:       "IT Item BadCat",
		CategoryID: 99999999,
		UnitID:     unitID(t, ctx, svc, "g"),
	}, itBy)
	assert.Error(t, err, "item with non-existent category_id should fail")

	require.NoError(t, svc.DeleteItem(ctx, item.ItemID))
	_, err = svc.GetItemByID(ctx, item.ItemID)
	assert.Error(t, err)
}

func TestIntegrationFlavorProfileCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	fp, err := svc.CreateFlavorProfile(ctx, "IT Flavor Alpha", itBy)
	require.NoError(t, err)
	require.NotZero(t, fp.FlavorID)
	assert.True(t, fp.IsActive)

	got, err := svc.GetFlavorProfileByID(ctx, fp.FlavorID)
	require.NoError(t, err)
	assert.Equal(t, "IT Flavor Alpha", got.Name)

	fps, err := svc.ListFlavorProfiles(ctx)
	require.NoError(t, err)
	var found bool
	for _, f := range fps {
		if f.FlavorID == fp.FlavorID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateFlavorProfile(ctx, fp.FlavorID, "IT Flavor Beta", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Flavor Beta", updated.Name)
	assert.False(t, updated.IsActive)

	require.NoError(t, svc.DeleteFlavorProfile(ctx, fp.FlavorID))
	_, err = svc.GetFlavorProfileByID(ctx, fp.FlavorID)
	assert.Error(t, err)
}

func TestIntegrationNutrientTypeCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	nt, err := svc.CreateNutrientType(ctx, "IT Nutrient Alpha", "mg")
	require.NoError(t, err)
	require.NotZero(t, nt.NutrientID)
	assert.Equal(t, "mg", nt.Unit)

	got, err := svc.GetNutrientTypeByID(ctx, nt.NutrientID)
	require.NoError(t, err)
	assert.Equal(t, "IT Nutrient Alpha", got.Name)

	nts, err := svc.ListNutrientTypes(ctx)
	require.NoError(t, err)
	var found bool
	for _, n := range nts {
		if n.NutrientID == nt.NutrientID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateNutrientType(ctx, nt.NutrientID, "IT Nutrient Beta", "g")
	require.NoError(t, err)
	assert.Equal(t, "IT Nutrient Beta", updated.Name)
	assert.Equal(t, "g", updated.Unit)

	require.NoError(t, svc.DeleteNutrientType(ctx, nt.NutrientID))
	_, err = svc.GetNutrientTypeByID(ctx, nt.NutrientID)
	assert.Error(t, err)
}

func TestIntegrationFoodJunctions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	cat, err := svc.CreateCategory(ctx, "IT Junction Category", "", false, itBy)
	require.NoError(t, err)
	junctionWeight := 50.0
	item, err := svc.CreateItem(ctx, Item{
		Name:       "IT Junction Item",
		CategoryID: cat.CategoryID,
		UnitID:     unitID(t, ctx, svc, "g"),
		NetWeight:  &junctionWeight,
		IsMetric:   true,
	}, itBy)
	require.NoError(t, err)
	nt, err := svc.CreateNutrientType(ctx, "IT Junction Nutrient", "mg")
	require.NoError(t, err)
	fp, err := svc.CreateFlavorProfile(ctx, "IT Junction Flavor", itBy)
	require.NoError(t, err)

	// Food nutrient junction.
	fn, err := svc.CreateFoodNutrient(ctx, item.ItemID, nt.NutrientID, 42.5, itBy)
	require.NoError(t, err)
	assert.Equal(t, nt.NutrientID, fn.NutrientID)
	assert.InDelta(t, 42.5, fn.Amount, 0.0001)

	fns, err := svc.ListFoodNutrientsByItem(ctx, item.ItemID)
	require.NoError(t, err)
	require.Len(t, fns, 1)
	assert.Equal(t, "IT Junction Nutrient", fns[0].Name)
	assert.InDelta(t, 42.5, fns[0].Amount, 0.0001)

	require.NoError(t, svc.DeleteFoodNutrient(ctx, item.ItemID, nt.NutrientID))
	fns, err = svc.ListFoodNutrientsByItem(ctx, item.ItemID)
	require.NoError(t, err)
	assert.Empty(t, fns)

	// Food flavor junction.
	ff, err := svc.CreateFoodFlavor(ctx, item.ItemID, fp.FlavorID, 4, itBy)
	require.NoError(t, err)
	assert.Equal(t, fp.FlavorID, ff.FlavorID)
	assert.Equal(t, int16(4), ff.Intensity)

	ffs, err := svc.ListFoodFlavorsByItem(ctx, item.ItemID)
	require.NoError(t, err)
	require.Len(t, ffs, 1)
	assert.Equal(t, "IT Junction Flavor", ffs[0].Name)

	require.NoError(t, svc.DeleteFoodFlavor(ctx, item.ItemID, fp.FlavorID))
	ffs, err = svc.ListFoodFlavorsByItem(ctx, item.ItemID)
	require.NoError(t, err)
	assert.Empty(t, ffs)

	// Check constraint: intensity outside 1..5.
	_, err = svc.CreateFoodFlavor(ctx, item.ItemID, fp.FlavorID, 9, itBy)
	assert.Error(t, err, "intensity outside 1-5 should violate check constraint")
}

func TestIntegrationIngredientCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	cat, err := svc.CreateCategory(ctx, "IT Ingredient Category", "", false, itBy)
	require.NoError(t, err)

	gID := unitID(t, ctx, svc, "g")
	in, err := svc.CreateIngredient(ctx, Ingredient{
		Name:          "IT Ingredient Alpha",
		CategoryID:    &cat.CategoryID,
		DefaultUnitID: &gID,
		IsActive:      true,
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, in.IngredientID)
	assert.Equal(t, "IT Ingredient Alpha", in.Name)
	require.NotNil(t, in.CategoryID)
	assert.Equal(t, cat.CategoryID, *in.CategoryID)
	require.NotNil(t, in.DefaultUnitID)
	assert.Equal(t, gID, *in.DefaultUnitID)

	got, err := svc.GetIngredientByID(ctx, in.IngredientID)
	require.NoError(t, err)
	assert.Equal(t, in.IngredientID, got.IngredientID)

	ins, err := svc.GetIngredientsByIDs(ctx, []int64{in.IngredientID})
	require.NoError(t, err)
	require.Len(t, ins, 1)

	kgID := unitID(t, ctx, svc, "kg")
	updated, err := svc.UpdateIngredient(ctx, in.IngredientID, Ingredient{
		Name:          "IT Ingredient Beta",
		DefaultUnitID: &kgID,
		IsActive:      false,
	}, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Ingredient Beta", updated.Name)
	assert.Nil(t, updated.CategoryID)
	assert.False(t, updated.IsActive)

	require.NoError(t, svc.DeleteIngredient(ctx, in.IngredientID))
	_, err = svc.GetIngredientByID(ctx, in.IngredientID)
	assert.Error(t, err)
}

func TestIntegrationSubmitBrandConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationServiceWithPool(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "brand-concurrent-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "brand-concurrent-b@example.com")
	name := fmt.Sprintf("Concurrent Brand %d", time.Now().UnixNano())

	results := make(chan struct {
		brandID int64
		err     error
	}, 2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b, err := svc.SubmitBrand(ctx, name, userA, itBy)
		results <- struct {
			brandID int64
			err     error
		}{b.BrandID, err}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		b, err := svc.SubmitBrand(ctx, name, userB, itBy)
		results <- struct {
			brandID int64
			err     error
		}{b.BrandID, err}
	}()
	wg.Wait()
	close(results)

	var ids []int64
	var gotErr bool
	for r := range results {
		if r.err != nil {
			require.True(t, strings.Contains(r.err.Error(), "conflict"))
			gotErr = true
			continue
		}
		require.NotZero(t, r.brandID)
		ids = append(ids, r.brandID)
	}
	assert.True(t, gotErr, "one concurrent submit should conflict")
	require.Len(t, ids, 1)

	brands, err := svc.ListBrands(ctx)
	require.NoError(t, err)
	var count int
	for _, b := range brands {
		if b.BrandID == ids[0] {
			count++
		}
	}
	assert.Equal(t, 1, count, "only one brand row should exist for the normalized name")
}

func TestIntegrationBrandModeration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationServiceWithPool(t, ctx)

	userA := testutil.MustUser(ctx, t, pool, "brand-mod-a@example.com")
	userB := testutil.MustUser(ctx, t, pool, "brand-mod-b@example.com")
	adminID := testutil.MustUser(ctx, t, pool, "brand-mod-admin@example.com")

	t.Run("submit creates pending brand for the submitter", func(t *testing.T) {
		b, err := svc.SubmitBrand(ctx, "Mod Alpha", userA, itBy)
		require.NoError(t, err)
		assert.Equal(t, BrandStatusPending, b.Status)
		require.NotNil(t, b.SubmittedByUserID)
		assert.Equal(t, userA, *b.SubmittedByUserID)
	})

	t.Run("duplicate name with different casing returns same brand", func(t *testing.T) {
		first, err := svc.SubmitBrand(ctx, "Mod Dup", userA, itBy)
		require.NoError(t, err)
		again, err := svc.SubmitBrand(ctx, "  MOD DUP ", userA, itBy)
		require.NoError(t, err)
		assert.Equal(t, first.BrandID, again.BrandID, "normalized-name match must reuse the row")
	})

	t.Run("foreign pending brand conflicts for another user", func(t *testing.T) {
		_, err := svc.SubmitBrand(ctx, "Mod Owned", userA, itBy)
		require.NoError(t, err)
		_, err = svc.SubmitBrand(ctx, "mod owned", userB, itBy)
		assert.ErrorIs(t, err, domainerr.ErrConflict,
			"a pending brand owned by another user must not be visible")
	})

	t.Run("pending queue lists only pending oldest first", func(t *testing.T) {
		older, err := svc.SubmitBrand(ctx, "Mod Queue One", userA, itBy)
		require.NoError(t, err)
		newer, err := svc.SubmitBrand(ctx, "Mod Queue Two", userB, itBy)
		require.NoError(t, err)
		_, err = svc.CreateBrand(ctx, "Mod Queue Approved", itBy)
		require.NoError(t, err)

		pending, err := svc.ListPendingBrands(ctx, 100, 0)
		require.NoError(t, err)
		var ids []int64
		for _, b := range pending {
			if b.BrandID == older.BrandID || b.BrandID == newer.BrandID {
				ids = append(ids, b.BrandID)
			}
			assert.NotEqual(t, "Mod Queue Approved", b.Name, "approved brand must not be pending")
		}
		require.Len(t, ids, 2)
		assert.Equal(t, older.BrandID, ids[0], "oldest pending first")
		assert.Equal(t, newer.BrandID, ids[1])

		n, err := svc.CountPendingBrands(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(2))
	})

	t.Run("search shows own pending and approved, hides foreign pending", func(t *testing.T) {
		_, err := svc.SubmitBrand(ctx, "Searchable Mine", userA, itBy)
		require.NoError(t, err)
		_, err = svc.SubmitBrand(ctx, "Searchable Theirs", userB, itBy)
		require.NoError(t, err)
		_, err = svc.CreateBrand(ctx, "Searchable Public", itBy)
		require.NoError(t, err)

		mine, err := svc.SearchBrands(ctx, "searchable", userA, RankParams{}, 50)
		require.NoError(t, err)
		names := map[string]string{}
		for _, b := range mine {
			names[b.Name] = b.Status
		}
		assert.Equal(t, BrandStatusPending, names["Searchable Mine"])
		assert.Equal(t, BrandStatusApproved, names["Searchable Public"])
		assert.NotContains(t, names, "Searchable Theirs",
			"another user's pending brand must not appear in search")
	})

	t.Run("approve records approver and exits the queue", func(t *testing.T) {
		b, err := svc.SubmitBrand(ctx, "Mod Approve Me", userA, itBy)
		require.NoError(t, err)
		require.NoError(t, svc.SetBrandStatus(ctx, b.BrandID, BrandStatusApproved, adminID, itBy))

		got, err := svc.GetBrandByID(ctx, b.BrandID)
		require.NoError(t, err)
		assert.Equal(t, BrandStatusApproved, got.Status)
		require.NotNil(t, got.ApprovedByUserID)
		assert.Equal(t, adminID, *got.ApprovedByUserID)
		assert.NotNil(t, got.ApprovedAt)

		pending, err := svc.ListPendingBrands(ctx, 100, 0)
		require.NoError(t, err)
		for _, p := range pending {
			assert.NotEqual(t, b.BrandID, p.BrandID)
		}
	})

	t.Run("reject clears approver and frees the name for resubmission", func(t *testing.T) {
		b, err := svc.SubmitBrand(ctx, "Mod Reject Me", userA, itBy)
		require.NoError(t, err)
		require.NoError(t, svc.SetBrandStatus(ctx, b.BrandID, BrandStatusApproved, adminID, itBy))
		require.NoError(t, svc.SetBrandStatus(ctx, b.BrandID, BrandStatusRejected, adminID, itBy))

		got, err := svc.GetBrandByID(ctx, b.BrandID)
		require.NoError(t, err)
		assert.Equal(t, BrandStatusRejected, got.Status)
		assert.Nil(t, got.ApprovedByUserID, "rejecting clears the approver")
		assert.Nil(t, got.ApprovedAt)

		// The partial unique index only covers non-rejected rows, so a
		// rejected name may be resubmitted — as a new pending row.
		resub, err := svc.SubmitBrand(ctx, "mod reject me", userA, itBy)
		require.NoError(t, err)
		assert.NotEqual(t, b.BrandID, resub.BrandID)
		assert.Equal(t, BrandStatusPending, resub.Status)
	})

	t.Run("resubmitting own approved name returns the brand", func(t *testing.T) {
		b, err := svc.SubmitBrand(ctx, "Mod Twice", userA, itBy)
		require.NoError(t, err)
		require.NoError(t, svc.SetBrandStatus(ctx, b.BrandID, BrandStatusApproved, adminID, itBy))
		again, err := svc.SubmitBrand(ctx, "MOD TWICE", userB, itBy)
		require.NoError(t, err)
		assert.Equal(t, b.BrandID, again.BrandID)
	})
}

func TestIntegrationSearchItemsRanking(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationServiceWithPool(t, ctx)

	uid := testutil.MustUser(ctx, t, pool, "rank-items@example.com")
	cat, err := svc.CreateCategory(ctx, "IT Rank Category", "", false, itBy)
	require.NoError(t, err)
	g := unitID(t, ctx, svc, "g")

	mkItem := func(name string) int64 {
		it, err := svc.CreateItem(ctx, Item{Name: name, CategoryID: cat.CategoryID, UnitID: g}, itBy)
		require.NoError(t, err)
		return it.ItemID
	}
	// Alphabetical order is the reverse of engagement order — proves ranking
	// happens in SQL before LIMIT, not client-side after the window.
	alphaID := mkItem("QZ9 AAA Plain")
	globalID := mkItem("QZ9 BBB Global")
	searchedID := mkItem("QZ9 CCC Searched")
	householdItemID := mkItem("QZ9 DDD Household")
	personalID := mkItem("QZ9 EEE Personal")
	favID := mkItem("QZ9 FFF Favorite")

	rank := RankParams{
		FavoriteIDs:  []int64{favID},
		PersonalIDs:  []int64{personalID},
		HouseholdIDs: []int64{householdItemID},
		GlobalIDs:    []int64{globalID},
		SearchTerms:  []string{"qz9 ccc"},
	}
	got, err := svc.SearchItems(ctx, uid, "qz9", nil, rank, 20, 0)
	require.NoError(t, err)
	require.Len(t, got, 6)
	order := []int64{favID, personalID, householdItemID, searchedID, globalID, alphaID}
	for i, it := range got {
		assert.Equal(t, order[i], it.ItemID, "position %d", i)
	}

	total, err := svc.CountSearchItems(ctx, uid, "qz9", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)

	t.Run("cold start falls back to alphabetical", func(t *testing.T) {
		got, err := svc.SearchItems(ctx, uid, "qz9", nil, RankParams{}, 20, 0)
		require.NoError(t, err)
		require.Len(t, got, 6)
		for i, it := range got {
			assert.Equal(t, []int64{alphaID, globalID, searchedID, householdItemID, personalID, favID}[i], it.ItemID)
		}
	})

	t.Run("term scopes the ranked window", func(t *testing.T) {
		got, err := svc.SearchItems(ctx, uid, "qz9 eee", nil, rank, 20, 0)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, personalID, got[0].ItemID)
	})

	t.Run("MatchItemIDs feeds include_ids", func(t *testing.T) {
		ids, err := svc.MatchItemIDs(ctx, "qz9 bbb", uid)
		require.NoError(t, err)
		assert.Equal(t, []int64{globalID}, ids)
	})

	t.Run("brand filter scopes ranked and remainder", func(t *testing.T) {
		brand, err := svc.CreateBrand(ctx, "IT Rank Brand", itBy)
		require.NoError(t, err)
		brandedID := func() int64 {
			it, err := svc.CreateItem(ctx, Item{Name: "QZ9 GGG Branded", BrandID: &brand.BrandID, CategoryID: cat.CategoryID, UnitID: g}, itBy)
			require.NoError(t, err)
			return it.ItemID
		}()
		got, err := svc.SearchItems(ctx, uid, "qz9", &brand.BrandID, rank, 20, 0)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, brandedID, got[0].ItemID)
		total, err := svc.CountSearchItems(ctx, uid, "qz9", &brand.BrandID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})
}

func TestMain(m *testing.M) {
	os.Exit(testutil.SharedDBTestMain(m))
}

func TestIntegrationGetOrCreateIngredientDedupe(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	first, err := svc.GetOrCreateIngredient(ctx, "IT Dedupe Corn", nil, nil, itBy)
	require.NoError(t, err)
	require.NotZero(t, first.IngredientID)
	assert.Equal(t, "it dedupe corn", first.Name, "stored name is canonical lowercase")

	// Case/whitespace variants resolve to the same row — the normalized
	// unique index backs this, GetOrCreate exercises it end to end.
	for _, variant := range []string{"IT DEDUPE CORN", "  it   dedupe  corn ", "It Dedupe Corn"} {
		again, err := svc.GetOrCreateIngredient(ctx, variant, nil, nil, itBy)
		require.NoError(t, err, "variant %q", variant)
		assert.Equal(t, first.IngredientID, again.IngredientID, "variant %q should dedupe", variant)
	}

	// Direct insert of a normalized duplicate is rejected by the index.
	_, err = svc.CreateIngredient(ctx, Ingredient{Name: "it dedupe  corn", IsActive: true}, itBy)
	assert.Error(t, err, "raw insert of a normalized duplicate must violate the index")
}

func TestIntegrationItemIngredientResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationServiceWithPool(t, ctx)

	hh := testutil.MustHousehold(ctx, t, pool)
	hhOther := testutil.MustHousehold(ctx, t, pool)
	uid := testutil.MustUser(ctx, t, pool, "it-resolve@example.com")

	cat, err := svc.CreateCategory(ctx, "IT Resolve Category", "", false, itBy)
	require.NoError(t, err)
	corn, err := svc.GetOrCreateIngredient(ctx, "it resolve corn", &cat.CategoryID, nil, itBy)
	require.NoError(t, err)
	alt, err := svc.GetOrCreateIngredient(ctx, "it resolve maize", &cat.CategoryID, nil, itBy)
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, Item{
		Name: "IT Resolve Green Giant", CategoryID: cat.CategoryID,
		UnitID: unitID(t, ctx, svc, "can"),
	}, itBy)
	require.NoError(t, err)
	loose, err := svc.CreateItem(ctx, Item{
		Name: "IT Resolve Loose", CategoryID: cat.CategoryID,
		UnitID: unitID(t, ctx, svc, "can"),
	}, itBy)
	require.NoError(t, err)

	// Unlinked item resolves to nil.
	got, err := svc.ResolveItemIngredient(ctx, hh, loose.ItemID)
	require.NoError(t, err)
	assert.Nil(t, got)

	// Catalog link resolves for every household.
	require.NoError(t, svc.SetItemIngredient(ctx, item.ItemID, &corn.IngredientID, itBy))
	got, err = svc.ResolveItemIngredient(ctx, hh, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, corn.IngredientID, *got)

	// Household override remaps the item — only for that household.
	require.NoError(t, svc.SetItemIngredientOverride(ctx, hh, item.ItemID, alt.IngredientID, itBy))
	got, err = svc.ResolveItemIngredient(ctx, hh, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, alt.IngredientID, *got)
	got, err = svc.ResolveItemIngredient(ctx, hhOther, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, corn.IngredientID, *got, "other households keep the catalog link")

	// ResolveIngredientItems follows the same override-wins order.
	items, err := svc.ResolveIngredientItems(ctx, hh, alt.IngredientID, uid)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, item.ItemID, items[0].ItemID)
	items, err = svc.ResolveIngredientItems(ctx, hhOther, corn.IngredientID, uid)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Clearing the override restores catalog resolution.
	require.NoError(t, svc.ClearItemIngredientOverride(ctx, hh, item.ItemID))
	got, err = svc.ResolveItemIngredient(ctx, hh, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, corn.IngredientID, *got)

	// Usual brand round-trips per household.
	require.NoError(t, svc.SetUsualItemForIngredient(ctx, hh, corn.IngredientID, item.ItemID, itBy))
	u, err := svc.GetUsualItemForIngredient(ctx, hh, corn.IngredientID)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, item.ItemID, u.ItemID)
	u, err = svc.GetUsualItemForIngredient(ctx, hhOther, corn.IngredientID)
	require.NoError(t, err)
	assert.Nil(t, u, "usual brand is household-scoped")
}

func TestIntegrationMergeIngredients(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationServiceWithPool(t, ctx)

	hh := testutil.MustHousehold(ctx, t, pool)
	cat, err := svc.CreateCategory(ctx, "IT Merge Category", "", false, itBy)
	require.NoError(t, err)
	src, err := svc.GetOrCreateIngredient(ctx, "it merge source", &cat.CategoryID, nil, itBy)
	require.NoError(t, err)
	tgt, err := svc.GetOrCreateIngredient(ctx, "it merge target", &cat.CategoryID, nil, itBy)
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, Item{
		Name: "IT Merge Item", CategoryID: cat.CategoryID,
		UnitID: unitID(t, ctx, svc, "each"),
	}, itBy)
	require.NoError(t, err)
	require.NoError(t, svc.SetItemIngredient(ctx, item.ItemID, &src.IngredientID, itBy))
	require.NoError(t, svc.SetUsualItemForIngredient(ctx, hh, src.IngredientID, item.ItemID, itBy))

	// A recipe with two lines: one ingredient-only (src), one that already
	// references the target. After merge the first repoints; a second
	// source-ref line carrying item_id demotes to item-only instead of
	// duplicating the target row.
	var recipeID int64
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recipe.recipe (name, created_by) VALUES ('it merge recipe', 'it') RETURNING recipe_id`).Scan(&recipeID))
	unitID := unitID(t, ctx, svc, "each")
	var lineA, lineB, lineC int64
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recipe.recipe_item (recipe_id, ingredient_id, quantity, unit_id)
		 VALUES ($1, $2, 1, $3) RETURNING recipe_item_id`, recipeID, src.IngredientID, unitID).Scan(&lineA))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recipe.recipe_item (recipe_id, ingredient_id, quantity, unit_id)
		 VALUES ($1, $2, 1, $3) RETURNING recipe_item_id`, recipeID, tgt.IngredientID, unitID).Scan(&lineB))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recipe.recipe_item (recipe_id, item_id, ingredient_id, quantity, unit_id)
		 VALUES ($1, $2, $3, 1, $4) RETURNING recipe_item_id`, recipeID, item.ItemID, src.IngredientID, unitID).Scan(&lineC))

	require.NoError(t, svc.MergeIngredients(ctx, src.IngredientID, tgt.IngredientID))

	// Item link and usual brand repoint.
	got, err := svc.ResolveItemIngredient(ctx, hh, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tgt.IngredientID, *got)
	u, err := svc.GetUsualItemForIngredient(ctx, hh, tgt.IngredientID)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, item.ItemID, u.ItemID)

	// lineA is a true duplicate of lineB post-merge (same recipe, same
	// ingredient, no item hint) — deleted. lineC conflicts too but carries
	// item_id, so it demotes to item-only and the brand hint survives.
	err = pool.QueryRow(ctx,
		`SELECT ingredient_id FROM recipe.recipe_item WHERE recipe_item_id = $1`, lineA).Scan(new(int64))
	assert.Error(t, err, "duplicate line should be removed by merge")
	var ing *int64
	var itemRef *int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ingredient_id, item_id FROM recipe.recipe_item WHERE recipe_item_id = $1`, lineC).Scan(&ing, &itemRef))
	assert.Nil(t, ing)
	require.NotNil(t, itemRef)
	assert.Equal(t, item.ItemID, *itemRef)
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM recipe.recipe_item WHERE ingredient_id = $1`, src.IngredientID).Scan(&n))
	assert.Zero(t, n)

	// Source row is gone.
	_, err = svc.GetIngredientByID(ctx, src.IngredientID)
	assert.Error(t, err)
}

// Batch resolution mirrors the single-item semantics: household override
// wins, catalog link next, unlinked items map to nil — in one round trip.
func TestIntegrationItemIngredientBatchResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationServiceWithPool(t, ctx)

	hh := testutil.MustHousehold(ctx, t, pool)
	hhOther := testutil.MustHousehold(ctx, t, pool)

	cat, err := svc.CreateCategory(ctx, "IT Batch Category", "", false, itBy)
	require.NoError(t, err)
	corn, err := svc.GetOrCreateIngredient(ctx, "it batch corn", &cat.CategoryID, nil, itBy)
	require.NoError(t, err)
	alt, err := svc.GetOrCreateIngredient(ctx, "it batch maize", &cat.CategoryID, nil, itBy)
	require.NoError(t, err)

	linked, err := svc.CreateItem(ctx, Item{
		Name: "IT Batch Linked", CategoryID: cat.CategoryID,
		UnitID: unitID(t, ctx, svc, "can"),
	}, itBy)
	require.NoError(t, err)
	overridden, err := svc.CreateItem(ctx, Item{
		Name: "IT Batch Overridden", CategoryID: cat.CategoryID,
		UnitID: unitID(t, ctx, svc, "can"),
	}, itBy)
	require.NoError(t, err)
	loose, err := svc.CreateItem(ctx, Item{
		Name: "IT Batch Loose", CategoryID: cat.CategoryID,
		UnitID: unitID(t, ctx, svc, "can"),
	}, itBy)
	require.NoError(t, err)

	require.NoError(t, svc.SetItemIngredient(ctx, linked.ItemID, &corn.IngredientID, itBy))
	require.NoError(t, svc.SetItemIngredient(ctx, overridden.ItemID, &corn.IngredientID, itBy))
	require.NoError(t, svc.SetItemIngredientOverride(ctx, hh, overridden.ItemID, alt.IngredientID, itBy))

	ids := []int64{linked.ItemID, overridden.ItemID, loose.ItemID}
	got, err := svc.ResolveItemIngredients(ctx, hh, ids)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.NotNil(t, got[linked.ItemID])
	assert.Equal(t, corn.IngredientID, *got[linked.ItemID])
	require.NotNil(t, got[overridden.ItemID])
	assert.Equal(t, alt.IngredientID, *got[overridden.ItemID], "override beats catalog link")
	assert.Nil(t, got[loose.ItemID], "unlinked item resolves to nil")

	// The other household sees catalog links only.
	got, err = svc.ResolveItemIngredients(ctx, hhOther, ids)
	require.NoError(t, err)
	assert.Equal(t, corn.IngredientID, *got[overridden.ItemID])

	// Empty input short-circuits.
	got, err = svc.ResolveItemIngredients(ctx, hh, nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	// Batch usual-brand lookup mirrors GetUsualItemForIngredient.
	require.NoError(t, svc.SetUsualItemForIngredient(ctx, hh, corn.IngredientID, linked.ItemID, itBy))
	require.NoError(t, svc.SetUsualItemForIngredient(ctx, hh, alt.IngredientID, overridden.ItemID, itBy))
	usuals, err := svc.GetUsualItemsForIngredients(ctx, hh, []int64{corn.IngredientID, alt.IngredientID})
	require.NoError(t, err)
	require.Len(t, usuals, 2)
	assert.Equal(t, linked.ItemID, usuals[corn.IngredientID].ItemID)
	assert.Equal(t, overridden.ItemID, usuals[alt.IngredientID].ItemID)
	usuals, err = svc.GetUsualItemsForIngredients(ctx, hhOther, []int64{corn.IngredientID})
	require.NoError(t, err)
	assert.Empty(t, usuals, "usuals are household-scoped")
}
