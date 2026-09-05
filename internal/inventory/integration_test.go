package inventory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

const itBy = "integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) *Service {
	t.Helper()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool)
}

func TestIntegrationBrandCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc := newIntegrationService(t, ctx)

	brand, err := svc.CreateBrand(ctx, "IT Brand Alpha")
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

	_, err := svc.CreateBrand(ctx, "IT Brand Dup")
	require.NoError(t, err)
	_, err = svc.CreateBrand(ctx, "IT Brand Dup")
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
	brand, err := svc.CreateBrand(ctx, "IT Item Brand")
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, Item{
		Name:       "IT Item Alpha",
		BrandID:    &brand.BrandID,
		Upc12:      "123456789012",
		CategoryID: cat.CategoryID,
		Unit:       "g",
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

	items, err := svc.ListItems(ctx, 100, 0)
	require.NoError(t, err)
	var found bool
	for _, it := range items {
		if it.ItemID == item.ItemID {
			found = true
		}
	}
	assert.True(t, found)

	require.NoError(t, svc.UpdateItem(ctx, item.ItemID, Item{
		Name:       "IT Item Beta",
		BrandID:    &brand.BrandID,
		CategoryID: cat.CategoryID,
		Unit:       "oz",
	}, itBy))
	got, err = svc.GetItemByID(ctx, item.ItemID)
	require.NoError(t, err)
	assert.Equal(t, "IT Item Beta", got.Name)
	assert.Equal(t, "oz", got.Unit)

	// FK violation: item referencing a non-existent category.
	_, err = svc.CreateItem(ctx, Item{
		Name:       "IT Item BadCat",
		CategoryID: 99999999,
		Unit:       "g",
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
	item, err := svc.CreateItem(ctx, Item{
		Name:       "IT Junction Item",
		CategoryID: cat.CategoryID,
		Unit:       "g",
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
