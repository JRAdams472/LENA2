package inventory

import (
	"context"
	"fmt"
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
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool)
}

func newIntegrationServiceWithPool(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
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

	cat, err := svc.CreateCategory(ctx, "IT Category Alpha", "test category", itBy)
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

	updated, err := svc.UpdateCategory(ctx, cat.CategoryID, "IT Category Beta", "updated", false, itBy)
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

	cat, err := svc.CreateCategory(ctx, "IT Item Category", "", itBy)
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

	cat, err := svc.CreateCategory(ctx, "IT Junction Category", "", itBy)
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

	cat, err := svc.CreateCategory(ctx, "IT Ingredient Category", "", itBy)
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

		mine, err := svc.SearchBrands(ctx, "searchable", userA, 50)
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
