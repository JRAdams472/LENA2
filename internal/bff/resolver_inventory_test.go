package bff

import (
	"context"
	"errors"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

var errInvBoom = errors.New("boom")

const invTestEmail = "inv@example.com"

func invCtx() context.Context {
	return testenv.WithAdmin(context.Background(), 7, invTestEmail)
}

func invUserCtx() context.Context {
	return testenv.WithUser(context.Background(), 7, invTestEmail)
}

func invStrPtr(s string) *string { return &s }
func invBoolPtr(b bool) *bool    { return &b }

func newInvMock(t *testing.T) *mock.MockInventoryService {
	t.Helper()
	return mock.NewMockInventoryService(gomock.NewController(t))
}

func TestResolver_Inventory_Brand(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetBrandByID(gomock.Any(), int64(3)).Return(inventory.Brand{BrandID: 3, Name: "Acme"}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Brand(invCtx(), struct{ ID graphql.ID }{ID: "3"})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("3"), res.ID())
		assert.Equal(t, "Acme", res.Name())
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Brand(context.Background(), struct{ ID graphql.ID }{ID: "3"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Brand(invCtx(), struct{ ID graphql.ID }{ID: "abc"})
		require.Error(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetBrandByID(gomock.Any(), int64(3)).Return(inventory.Brand{}, errInvBoom)
		r := &Resolver{InventoryService: inv}
		_, err := r.Brand(invCtx(), struct{ ID graphql.ID }{ID: "3"})
		require.ErrorIs(t, err, errInvBoom)
	})
}

func TestResolver_Inventory_Brands(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().ListBrands(gomock.Any()).Return([]inventory.Brand{
			{BrandID: 1, Name: "Acme"},
			{BrandID: 2, Name: "Beta"},
		}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Brands(invCtx())
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Equal(t, graphql.ID("1"), res[0].ID())
		assert.Equal(t, "Acme", res[0].Name())
		assert.Equal(t, graphql.ID("2"), res[1].ID())
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Brands(context.Background())
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("service error", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().ListBrands(gomock.Any()).Return(nil, errInvBoom)
		r := &Resolver{InventoryService: inv}
		_, err := r.Brands(invCtx())
		require.ErrorIs(t, err, errInvBoom)
	})
}

func TestResolver_Inventory_Category(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetCategoryByID(gomock.Any(), int64(4)).Return(inventory.Category{
			CategoryID: 4, Name: "Produce", Description: "Fresh",
		}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Category(invCtx(), struct{ ID graphql.ID }{ID: "4"})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("4"), res.ID())
		assert.Equal(t, "Produce", res.Name())
		require.NotNil(t, res.Description())
		assert.Equal(t, "Fresh", *res.Description())
	})

	t.Run("nil description", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetCategoryByID(gomock.Any(), int64(4)).Return(inventory.Category{CategoryID: 4, Name: "Produce"}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Category(invCtx(), struct{ ID graphql.ID }{ID: "4"})
		require.NoError(t, err)
		assert.Nil(t, res.Description())
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Category(context.Background(), struct{ ID graphql.ID }{ID: "4"})
		require.ErrorContains(t, err, "unauthorized")
	})
}

func TestResolver_Inventory_Categories(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().ListCategories(gomock.Any()).Return([]inventory.Category{
		{CategoryID: 1, Name: "Produce"},
	}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.Categories(invCtx())
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, graphql.ID("1"), res[0].ID())
	assert.Equal(t, "Produce", res[0].Name())

	_, err = r.Categories(context.Background())
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_FlavorProfiles(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().ListFlavorProfiles(gomock.Any()).Return([]inventory.FlavorProfile{
		{FlavorID: 9, Name: "Spicy", IsActive: true},
	}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.FlavorProfiles(invCtx())
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, graphql.ID("9"), res[0].ID())
	assert.Equal(t, "Spicy", res[0].Name())
	assert.True(t, res[0].IsActive())

	_, err = r.FlavorProfiles(context.Background())
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_NutrientTypes(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().ListNutrientTypes(gomock.Any()).Return([]inventory.NutrientType{
		{NutrientID: 2, Name: "Sodium", Unit: "mg"},
	}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.NutrientTypes(invCtx())
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, graphql.ID("2"), res[0].ID())
	assert.Equal(t, "Sodium", res[0].Name())
	assert.Equal(t, "mg", res[0].Unit())

	_, err = r.NutrientTypes(context.Background())
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_Item(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		brandID := int64(8)
		inv := newInvMock(t)
		inv.EXPECT().GetItemByID(gomock.Any(), int64(5)).Return(inventory.Item{
			ItemID: 5, Name: "Milk", BrandID: &brandID, Upc12: "012345678901",
			CategoryID: 2, UnitID: 9,
		}, nil)
		inv.EXPECT().GetUnitByID(gomock.Any(), int64(9)).Return(inventory.Unit{UnitID: 9, Name: "gallon"}, nil)
		inv.EXPECT().GetBrandByID(gomock.Any(), int64(8)).Return(inventory.Brand{BrandID: 8, Name: "DairyCo"}, nil)
		inv.EXPECT().GetCategoryByID(gomock.Any(), int64(2)).Return(inventory.Category{CategoryID: 2, Name: "Dairy"}, nil)
		inv.EXPECT().ListFoodNutrientsByItem(gomock.Any(), int64(5)).Return([]inventory.FoodNutrient{
			{NutrientID: 1, Name: "Calcium", Unit: "mg", Amount: 300},
		}, nil)
		inv.EXPECT().ListFoodFlavorsByItem(gomock.Any(), int64(5)).Return([]inventory.FoodFlavor{
			{FlavorID: 4, Name: "Sweet", Intensity: 2},
		}, nil)

		r := &Resolver{InventoryService: inv}
		res, err := r.Item(invCtx(), struct{ ID graphql.ID }{ID: "5"})
		require.NoError(t, err)
		ctx := invCtx()
		assert.Equal(t, graphql.ID("5"), res.ID())
		assert.Equal(t, "Milk", res.Name())
		require.NotNil(t, res.Upc12())
		assert.Equal(t, "012345678901", *res.Upc12())
		assert.Nil(t, res.Upc14())
		unit, err := res.Unit(ctx)
		require.NoError(t, err)
		assert.Equal(t, "gallon", unit)

		brand, err := res.Brand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "DairyCo", brand.Name())

		cat, err := res.Category(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Dairy", cat.Name())

		nutrients, err := res.Nutrients(ctx)
		require.NoError(t, err)
		require.Len(t, nutrients, 1)
		assert.Equal(t, 300.0, nutrients[0].Amount())
		nt, err := nutrients[0].Nutrient(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Calcium", nt.Name())

		flavors, err := res.Flavors(ctx)
		require.NoError(t, err)
		require.Len(t, flavors, 1)
		assert.Equal(t, int32(2), flavors[0].Intensity())
		fp, err := flavors[0].Flavor(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Sweet", fp.Name())
	})

	t.Run("no brand", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetItemByID(gomock.Any(), int64(5)).Return(inventory.Item{ItemID: 5, Name: "Milk", CategoryID: 2}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Item(invCtx(), struct{ ID graphql.ID }{ID: "5"})
		require.NoError(t, err)
		brand, err := res.Brand(invCtx())
		require.NoError(t, err)
		assert.Nil(t, brand)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Item(context.Background(), struct{ ID graphql.ID }{ID: "5"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Item(invCtx(), struct{ ID graphql.ID }{ID: "abc"})
		require.Error(t, err)
	})
}

func TestResolver_Inventory_Items(t *testing.T) {
	type pageArgs = struct {
		Page     int32
		PageSize int32
	}

	t.Run("happy path", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().ListItems(gomock.Any(), int32(10), int32(10)).Return([]inventory.Item{
			{ItemID: 1, Name: "Milk"},
			{ItemID: 2, Name: "Eggs"},
		}, nil)
		inv.EXPECT().CountItems(gomock.Any()).Return(int64(5), nil)
		inv.EXPECT().GetCategoriesByIDs(gomock.Any(), []int64{0}).Return(nil, nil)
		inv.EXPECT().ListFoodNutrientsByItems(gomock.Any(), []int64{1, 2}).Return(nil, nil)
		inv.EXPECT().ListFoodFlavorsByItems(gomock.Any(), []int64{1, 2}).Return(nil, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Items(invCtx(), pageArgs{Page: 2, PageSize: 10})
		require.NoError(t, err)
		require.Len(t, res.Items(), 2)
		assert.Equal(t, graphql.ID("1"), res.Items()[0].ID())
		pi := res.PageInfo()
		assert.Equal(t, int32(2), pi.PageNumber())
		assert.Equal(t, int32(10), pi.PageSize())
		assert.Equal(t, int32(5), pi.TotalCount())
	})

	t.Run("clamps page and page size", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().ListItems(gomock.Any(), int32(100), int32(0)).Return([]inventory.Item{}, nil)
		inv.EXPECT().CountItems(gomock.Any()).Return(int64(0), nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Items(invCtx(), pageArgs{Page: 0, PageSize: 500})
		require.NoError(t, err)
		pi := res.PageInfo()
		assert.Equal(t, int32(1), pi.PageNumber())
		assert.Equal(t, int32(100), pi.PageSize())
		assert.Equal(t, int32(0), pi.TotalCount())
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Items(context.Background(), pageArgs{Page: 1, PageSize: 10})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("service error", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().ListItems(gomock.Any(), int32(10), int32(0)).Return(nil, errInvBoom)
		r := &Resolver{InventoryService: inv}
		_, err := r.Items(invCtx(), pageArgs{Page: 1, PageSize: 10})
		require.ErrorIs(t, err, errInvBoom)
	})
}

func TestResolver_Inventory_CreateBrand(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().CreateBrand(gomock.Any(), "Acme").Return(inventory.Brand{BrandID: 1, Name: "Acme"}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.CreateBrand(invCtx(), struct{ Input createBrandInput }{Input: createBrandInput{Name: "Acme"}})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("1"), res.ID())
	assert.Equal(t, "Acme", res.Name())

	_, err = r.CreateBrand(context.Background(), struct{ Input createBrandInput }{Input: createBrandInput{Name: "X"}})
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_UpdateBrand(t *testing.T) {
	type args = struct {
		ID    graphql.ID
		Input updateBrandInput
	}

	inv := newInvMock(t)
	inv.EXPECT().UpdateBrand(gomock.Any(), int64(1), "NewName").Return(inventory.Brand{BrandID: 1, Name: "NewName"}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.UpdateBrand(invCtx(), args{ID: "1", Input: updateBrandInput{Name: "NewName"}})
	require.NoError(t, err)
	assert.Equal(t, "NewName", res.Name())

	_, err = r.UpdateBrand(context.Background(), args{ID: "1"})
	require.ErrorContains(t, err, "unauthorized")

	_, err = r.UpdateBrand(invCtx(), args{ID: "abc"})
	require.Error(t, err)
}

func TestResolver_Inventory_DeleteBrand(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().DeleteBrand(gomock.Any(), int64(1)).Return(nil)
	r := &Resolver{InventoryService: inv}
	ok, err := r.DeleteBrand(invCtx(), struct{ ID graphql.ID }{ID: "1"})
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = r.DeleteBrand(context.Background(), struct{ ID graphql.ID }{ID: "1"})
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_CreateCategory(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().CreateCategory(gomock.Any(), "Produce", "Fresh", invTestEmail).
		Return(inventory.Category{CategoryID: 2, Name: "Produce", Description: "Fresh"}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.CreateCategory(invCtx(), struct{ Input createCategoryInput }{
		Input: createCategoryInput{Name: "Produce", Description: invStrPtr("Fresh")},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("2"), res.ID())
	require.NotNil(t, res.Description())
	assert.Equal(t, "Fresh", *res.Description())

	_, err = r.CreateCategory(context.Background(), struct{ Input createCategoryInput }{})
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_UpdateCategory(t *testing.T) {
	type args = struct {
		ID    graphql.ID
		Input updateCategoryInput
	}

	inv := newInvMock(t)
	inv.EXPECT().GetCategoryByID(gomock.Any(), int64(2)).
		Return(inventory.Category{CategoryID: 2, Name: "Old", Description: "Desc", IsActive: true}, nil)
	inv.EXPECT().UpdateCategory(gomock.Any(), int64(2), "New", "Desc", false, invTestEmail).
		Return(inventory.Category{CategoryID: 2, Name: "New", Description: "Desc"}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.UpdateCategory(invCtx(), args{
		ID:    "2",
		Input: updateCategoryInput{Name: invStrPtr("New"), IsActive: invBoolPtr(false)},
	})
	require.NoError(t, err)
	assert.Equal(t, "New", res.Name())

	_, err = r.UpdateCategory(context.Background(), args{ID: "2"})
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_DeleteCategory(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().DeleteCategory(gomock.Any(), int64(2)).Return(nil)
	r := &Resolver{InventoryService: inv}
	ok, err := r.DeleteCategory(invCtx(), struct{ ID graphql.ID }{ID: "2"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_Inventory_CreateItem(t *testing.T) {
	inv := newInvMock(t)
	brandID := graphql.ID("8")
	expected := inventory.Item{Name: "Milk", BrandID: invInt64Ptr(8), Upc12: "0123", CategoryID: 2, UnitID: 9}
	inv.EXPECT().GetUnitByName(gomock.Any(), "gal").Return(inventory.Unit{UnitID: 9, Name: "gallon"}, nil)
	inv.EXPECT().CreateItem(gomock.Any(), expected, invTestEmail).
		Return(inventory.Item{ItemID: 5, Name: "Milk", BrandID: invInt64Ptr(8), CategoryID: 2, UnitID: 9}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.CreateItem(invCtx(), struct{ Input createItemInput }{
		Input: createItemInput{
			Name: "Milk", BrandID: &brandID, Upc12: invStrPtr("0123"),
			CategoryID: "2", Unit: "gal",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("5"), res.ID())
	assert.Equal(t, "Milk", res.Name())

	_, err = r.CreateItem(context.Background(), struct{ Input createItemInput }{})
	require.ErrorContains(t, err, "unauthorized")

	_, err = r.CreateItem(invUserCtx(), struct{ Input createItemInput }{
		Input: createItemInput{Name: "Milk", CategoryID: "2"},
	})
	require.ErrorContains(t, err, "forbidden")

	_, err = r.CreateItem(invCtx(), struct{ Input createItemInput }{
		Input: createItemInput{Name: "Milk", CategoryID: "abc"},
	})
	require.Error(t, err)
}

func invInt64Ptr(v int64) *int64 { return &v }

func TestResolver_Inventory_UpdateItem(t *testing.T) {
	type args = struct {
		ID    graphql.ID
		Input updateItemInput
	}

	inv := newInvMock(t)
	inv.EXPECT().GetItemByID(gomock.Any(), int64(5)).
		Return(inventory.Item{ItemID: 5, Name: "Old", CategoryID: 2, UnitID: 9}, nil)
	inv.EXPECT().UpdateItem(gomock.Any(), int64(5), inventory.Item{
		Name: "New", CategoryID: 2, UnitID: 9,
	}, invTestEmail).Return(nil)
	inv.EXPECT().GetItemByID(gomock.Any(), int64(5)).
		Return(inventory.Item{ItemID: 5, Name: "New", CategoryID: 2, UnitID: 9}, nil)
	r := &Resolver{InventoryService: inv}
	res, err := r.UpdateItem(invCtx(), args{ID: "5", Input: updateItemInput{Name: invStrPtr("New")}})
	require.NoError(t, err)
	assert.Equal(t, "New", res.Name())

	_, err = r.UpdateItem(context.Background(), args{ID: "5"})
	require.ErrorContains(t, err, "unauthorized")

	_, err = r.UpdateItem(invCtx(), args{ID: "abc"})
	require.Error(t, err)
}

func TestResolver_Inventory_DeleteItem(t *testing.T) {
	inv := newInvMock(t)
	inv.EXPECT().DeleteItem(gomock.Any(), int64(5)).Return(nil)
	r := &Resolver{InventoryService: inv}
	ok, err := r.DeleteItem(invCtx(), struct{ ID graphql.ID }{ID: "5"})
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = r.DeleteItem(context.Background(), struct{ ID graphql.ID }{ID: "5"})
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_Inventory_FlavorProfileMutations(t *testing.T) {
	type updateArgs = struct {
		ID    graphql.ID
		Input updateFlavorProfileInput
	}

	t.Run("create", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().CreateFlavorProfile(gomock.Any(), "Spicy", invTestEmail).
			Return(inventory.FlavorProfile{FlavorID: 9, Name: "Spicy", IsActive: true}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.CreateFlavorProfile(invCtx(), struct{ Input createFlavorProfileInput }{
			Input: createFlavorProfileInput{Name: "Spicy"},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("9"), res.ID())
		assert.True(t, res.IsActive())
	})

	t.Run("update", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetFlavorProfileByID(gomock.Any(), int64(9)).
			Return(inventory.FlavorProfile{FlavorID: 9, Name: "Spicy", IsActive: true}, nil)
		inv.EXPECT().UpdateFlavorProfile(gomock.Any(), int64(9), "Mild", false, invTestEmail).
			Return(inventory.FlavorProfile{FlavorID: 9, Name: "Mild"}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.UpdateFlavorProfile(invCtx(), updateArgs{
			ID:    "9",
			Input: updateFlavorProfileInput{Name: invStrPtr("Mild"), IsActive: invBoolPtr(false)},
		})
		require.NoError(t, err)
		assert.Equal(t, "Mild", res.Name())
	})

	t.Run("delete", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().DeleteFlavorProfile(gomock.Any(), int64(9)).Return(nil)
		r := &Resolver{InventoryService: inv}
		ok, err := r.DeleteFlavorProfile(invCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.CreateFlavorProfile(context.Background(), struct{ Input createFlavorProfileInput }{})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.UpdateFlavorProfile(context.Background(), updateArgs{ID: "9"})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.DeleteFlavorProfile(context.Background(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorContains(t, err, "unauthorized")
	})
}

func TestResolver_Inventory_NutrientTypeMutations(t *testing.T) {
	type updateArgs = struct {
		ID    graphql.ID
		Input updateNutrientTypeInput
	}

	t.Run("create", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().CreateNutrientType(gomock.Any(), "Sodium", "mg").
			Return(inventory.NutrientType{NutrientID: 2, Name: "Sodium", Unit: "mg"}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.CreateNutrientType(invCtx(), struct{ Input createNutrientTypeInput }{
			Input: createNutrientTypeInput{Name: "Sodium", Unit: "mg"},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("2"), res.ID())
		assert.Equal(t, "mg", res.Unit())
	})

	t.Run("update", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetNutrientTypeByID(gomock.Any(), int64(2)).
			Return(inventory.NutrientType{NutrientID: 2, Name: "Sodium", Unit: "mg"}, nil)
		inv.EXPECT().UpdateNutrientType(gomock.Any(), int64(2), "Sodium", "g").
			Return(inventory.NutrientType{NutrientID: 2, Name: "Sodium", Unit: "g"}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.UpdateNutrientType(invCtx(), updateArgs{
			ID:    "2",
			Input: updateNutrientTypeInput{Unit: invStrPtr("g")},
		})
		require.NoError(t, err)
		assert.Equal(t, "g", res.Unit())
	})

	t.Run("delete", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().DeleteNutrientType(gomock.Any(), int64(2)).Return(nil)
		r := &Resolver{InventoryService: inv}
		ok, err := r.DeleteNutrientType(invCtx(), struct{ ID graphql.ID }{ID: "2"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.CreateNutrientType(context.Background(), struct{ Input createNutrientTypeInput }{})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.UpdateNutrientType(context.Background(), updateArgs{ID: "2"})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.DeleteNutrientType(context.Background(), struct{ ID graphql.ID }{ID: "2"})
		require.ErrorContains(t, err, "unauthorized")
	})
}

func TestResolver_Inventory_FoodNutrientMutations(t *testing.T) {
	type removeArgs = struct {
		ItemID     graphql.ID
		NutrientID graphql.ID
	}

	t.Run("add", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().CreateFoodNutrient(gomock.Any(), int64(5), int64(2), 300.0, invTestEmail).
			Return(inventory.FoodNutrient{NutrientID: 2, Name: "Sodium", Unit: "mg", Amount: 300}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.AddFoodNutrient(invCtx(), struct{ Input addFoodNutrientInput }{
			Input: addFoodNutrientInput{ItemID: "5", NutrientID: "2", Amount: 300},
		})
		require.NoError(t, err)
		assert.Equal(t, 300.0, res.Amount())
		nt, err := res.Nutrient(invCtx())
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("2"), nt.ID())
		assert.Equal(t, "Sodium", nt.Name())
	})

	t.Run("remove", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().DeleteFoodNutrient(gomock.Any(), int64(5), int64(2)).Return(nil)
		r := &Resolver{InventoryService: inv}
		ok, err := r.RemoveFoodNutrient(invCtx(), removeArgs{ItemID: "5", NutrientID: "2"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("service error", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().DeleteFoodNutrient(gomock.Any(), int64(5), int64(2)).Return(errInvBoom)
		r := &Resolver{InventoryService: inv}
		ok, err := r.RemoveFoodNutrient(invCtx(), removeArgs{ItemID: "5", NutrientID: "2"})
		require.ErrorIs(t, err, errInvBoom)
		assert.False(t, ok)
	})

	t.Run("unauthorized and invalid ids", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.AddFoodNutrient(context.Background(), struct{ Input addFoodNutrientInput }{})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.RemoveFoodNutrient(context.Background(), removeArgs{ItemID: "5", NutrientID: "2"})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.AddFoodNutrient(invCtx(), struct{ Input addFoodNutrientInput }{
			Input: addFoodNutrientInput{ItemID: "abc", NutrientID: "2"},
		})
		require.Error(t, err)
	})
}

func TestResolver_Inventory_FoodFlavorMutations(t *testing.T) {
	type removeArgs = struct {
		ItemID   graphql.ID
		FlavorID graphql.ID
	}

	t.Run("add", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().CreateFoodFlavor(gomock.Any(), int64(5), int64(9), int16(3), invTestEmail).
			Return(inventory.FoodFlavor{FlavorID: 9, Name: "Spicy", Intensity: 3}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.AddFoodFlavor(invCtx(), struct{ Input addFoodFlavorInput }{
			Input: addFoodFlavorInput{ItemID: "5", FlavorID: "9", Intensity: 3},
		})
		require.NoError(t, err)
		assert.Equal(t, int32(3), res.Intensity())
		f, err := res.Flavor(invCtx())
		require.NoError(t, err)
		assert.Equal(t, "Spicy", f.Name())
	})

	t.Run("remove", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().DeleteFoodFlavor(gomock.Any(), int64(5), int64(9)).Return(nil)
		r := &Resolver{InventoryService: inv}
		ok, err := r.RemoveFoodFlavor(invCtx(), removeArgs{ItemID: "5", FlavorID: "9"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unauthorized and invalid ids", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.AddFoodFlavor(context.Background(), struct{ Input addFoodFlavorInput }{})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.RemoveFoodFlavor(context.Background(), removeArgs{ItemID: "5", FlavorID: "9"})
		require.ErrorContains(t, err, "unauthorized")
		_, err = r.RemoveFoodFlavor(invCtx(), removeArgs{ItemID: "5", FlavorID: "abc"})
		require.Error(t, err)
	})
}

func TestResolver_Inventory_Ingredient(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		catID := int64(4)
		inv := newInvMock(t)
		unitID := int64(10)
		inv.EXPECT().GetIngredientByID(gomock.Any(), int64(9)).Return(inventory.Ingredient{
			IngredientID: 9, Name: "All-Purpose Flour", CategoryID: &catID,
			DefaultUnitID: &unitID, IsActive: true,
		}, nil)
		inv.EXPECT().GetUnitByID(gomock.Any(), int64(10)).Return(inventory.Unit{UnitID: 10, Name: "gram"}, nil)
		inv.EXPECT().GetCategoryByID(gomock.Any(), int64(4)).Return(inventory.Category{
			CategoryID: 4, Name: "Baking",
		}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Ingredient(invCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("9"), res.ID())
		assert.Equal(t, "All-Purpose Flour", res.Name())
		du, err := res.DefaultUnit(invCtx())
		require.NoError(t, err)
		require.NotNil(t, du)
		assert.Equal(t, "gram", *du)
		assert.True(t, res.IsActive())
		cat, err := res.Category(invCtx())
		require.NoError(t, err)
		assert.Equal(t, "Baking", cat.Name())
	})

	t.Run("nil category and unit", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetIngredientByID(gomock.Any(), int64(9)).Return(inventory.Ingredient{
			IngredientID: 9, Name: "Flour", IsActive: true,
		}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Ingredient(invCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		du, err := res.DefaultUnit(invCtx())
		require.NoError(t, err)
		assert.Nil(t, du)
		cat, err := res.Category(invCtx())
		require.NoError(t, err)
		assert.Nil(t, cat)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Ingredient(context.Background(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Ingredient(invCtx(), struct{ ID graphql.ID }{ID: "abc"})
		require.Error(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetIngredientByID(gomock.Any(), int64(9)).Return(inventory.Ingredient{}, errInvBoom)
		r := &Resolver{InventoryService: inv}
		_, err := r.Ingredient(invCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorIs(t, err, errInvBoom)
	})
}

func TestResolver_Inventory_Ingredients(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().ListIngredients(gomock.Any(), int32(25), int32(0)).Return([]inventory.Ingredient{
			{IngredientID: 1, Name: "Flour", IsActive: true},
			{IngredientID: 2, Name: "Sugar", IsActive: true},
		}, nil)
		inv.EXPECT().CountIngredients(gomock.Any()).Return(int64(42), nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.Ingredients(invCtx(), struct {
			Page     int32
			PageSize int32
		}{Page: 1, PageSize: 25})
		require.NoError(t, err)
		require.Len(t, res.Items(), 2)
		assert.Equal(t, int32(42), res.PageInfo().TotalCount())
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.Ingredients(context.Background(), struct {
			Page     int32
			PageSize int32
		}{Page: 1, PageSize: 25})
		require.ErrorContains(t, err, "unauthorized")
	})
}

func TestResolver_Inventory_IngredientMutations(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetUnitByName(gomock.Any(), "g").Return(inventory.Unit{UnitID: 10, Name: "gram"}, nil)
		inv.EXPECT().CreateIngredient(gomock.Any(), inventory.Ingredient{
			Name:          "Flour",
			DefaultUnitID: invInt64Ptr(10),
			IsActive:      true,
		}, invTestEmail).Return(inventory.Ingredient{IngredientID: 9, Name: "Flour", IsActive: true}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.CreateIngredient(invCtx(), struct{ Input createIngredientInput }{
			Input: createIngredientInput{Name: "Flour", DefaultUnit: invStrPtr("g")},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("9"), res.ID())
	})

	t.Run("update", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().GetIngredientByID(gomock.Any(), int64(9)).Return(inventory.Ingredient{
			IngredientID: 9, Name: "Flour", IsActive: true,
		}, nil)
		inv.EXPECT().UpdateIngredient(gomock.Any(), int64(9), inventory.Ingredient{
			Name:     "Bread Flour",
			IsActive: true,
		}, invTestEmail).Return(inventory.Ingredient{IngredientID: 9, Name: "Bread Flour", IsActive: true}, nil)
		r := &Resolver{InventoryService: inv}
		res, err := r.UpdateIngredient(invCtx(), struct {
			ID    graphql.ID
			Input updateIngredientInput
		}{ID: "9", Input: updateIngredientInput{Name: invStrPtr("Bread Flour")}})
		require.NoError(t, err)
		assert.Equal(t, "Bread Flour", res.Name())
	})

	t.Run("delete", func(t *testing.T) {
		inv := newInvMock(t)
		inv.EXPECT().DeleteIngredient(gomock.Any(), int64(9)).Return(nil)
		r := &Resolver{InventoryService: inv}
		ok, err := r.DeleteIngredient(invCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.CreateIngredient(invUserCtx(), struct{ Input createIngredientInput }{
			Input: createIngredientInput{Name: "Flour"},
		})
		require.ErrorContains(t, err, "forbidden")
		_, err = r.UpdateIngredient(invUserCtx(), struct {
			ID    graphql.ID
			Input updateIngredientInput
		}{ID: "9", Input: updateIngredientInput{}})
		require.ErrorContains(t, err, "forbidden")
		_, err = r.DeleteIngredient(invUserCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorContains(t, err, "forbidden")
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{InventoryService: newInvMock(t)}
		_, err := r.CreateIngredient(context.Background(), struct{ Input createIngredientInput }{})
		require.ErrorContains(t, err, "unauthorized")
	})
}
