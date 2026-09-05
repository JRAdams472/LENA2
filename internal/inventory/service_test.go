package inventory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/inventory/sqlc"
	"github.com/JRAdams472/LENA2/internal/inventory/sqlc/mock"
)

var errBoom = errors.New("boom")

func newTestService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	ctrl := gomock.NewController(t)
	q := mock.NewMockQuerier(ctrl)
	return &Service{q: q}, q
}

func TestCreateBrand(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s, q := newTestService(t)
	q.EXPECT().CreateBrand(ctx, "Acme").
		Return(sqlc.InventoryBrand{BrandID: 7, Name: "Acme", CreatedAt: now}, nil)

	b, err := s.CreateBrand(ctx, "Acme")
	require.NoError(t, err)
	assert.Equal(t, Brand{BrandID: 7, Name: "Acme", CreatedAt: now}, b)
}

func TestCreateBrand_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateBrand(ctx, "Acme").Return(sqlc.InventoryBrand{}, errBoom)

	_, err := s.CreateBrand(ctx, "Acme")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create brand:")
}

func TestGetBrandByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s, q := newTestService(t)
	q.EXPECT().GetBrandByID(ctx, int64(7)).
		Return(sqlc.InventoryBrand{BrandID: 7, Name: "Acme", CreatedAt: now}, nil)

	b, err := s.GetBrandByID(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), b.BrandID)
	assert.Equal(t, "Acme", b.Name)
	assert.Equal(t, now, b.CreatedAt)
}

func TestGetBrandByID_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetBrandByID(ctx, int64(7)).Return(sqlc.InventoryBrand{}, errBoom)

	_, err := s.GetBrandByID(ctx, 7)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "get brand by id:")
}

func TestListBrands(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s, q := newTestService(t)
	q.EXPECT().ListBrands(ctx).Return([]sqlc.InventoryBrand{
		{BrandID: 1, Name: "A", CreatedAt: now},
		{BrandID: 2, Name: "B", CreatedAt: now},
	}, nil)

	brands, err := s.ListBrands(ctx)
	require.NoError(t, err)
	require.Len(t, brands, 2)
	assert.Equal(t, "A", brands[0].Name)
	assert.Equal(t, "B", brands[1].Name)
}

func TestListBrands_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListBrands(ctx).Return(nil, errBoom)

	_, err := s.ListBrands(ctx)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list brands:")
}

func TestUpdateBrand(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s, q := newTestService(t)
	q.EXPECT().UpdateBrand(ctx, sqlc.UpdateBrandParams{BrandID: 7, Name: "New"}).
		Return(sqlc.InventoryBrand{BrandID: 7, Name: "New", CreatedAt: now}, nil)

	b, err := s.UpdateBrand(ctx, 7, "New")
	require.NoError(t, err)
	assert.Equal(t, "New", b.Name)
}

func TestUpdateBrand_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateBrand(ctx, sqlc.UpdateBrandParams{BrandID: 7, Name: "New"}).
		Return(sqlc.InventoryBrand{}, errBoom)

	_, err := s.UpdateBrand(ctx, 7, "New")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "update brand:")
}

func TestDeleteBrand(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteBrand(ctx, int64(7)).Return(nil)

	assert.NoError(t, s.DeleteBrand(ctx, 7))
}

func TestDeleteBrand_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteBrand(ctx, int64(7)).Return(errBoom)

	assert.ErrorIs(t, s.DeleteBrand(ctx, 7), errBoom)
}

func TestCreateCategory(t *testing.T) {
	ctx := context.Background()

	s, q := newTestService(t)
	q.EXPECT().CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:        "Produce",
		Description: pgtype.Text{String: "fresh", Valid: true},
		IsActive:    true,
		CreatedBy:   "alice",
		UpdatedBy:   pgtype.Text{String: "alice", Valid: true},
	}).Return(sqlc.InventoryCategory{
		CategoryID:  3,
		Name:        "Produce",
		Description: pgtype.Text{String: "fresh", Valid: true},
		IsActive:    true,
	}, nil)

	c, err := s.CreateCategory(ctx, "Produce", "fresh", "alice")
	require.NoError(t, err)
	assert.Equal(t, Category{CategoryID: 3, Name: "Produce", Description: "fresh", IsActive: true}, c)
}

func TestCreateCategory_EmptyDescription(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateCategory(ctx, gomock.Cond(func(arg sqlc.CreateCategoryParams) bool {
		return !arg.Description.Valid && arg.CreatedBy == "alice" &&
			arg.UpdatedBy.Valid && arg.UpdatedBy.String == "alice"
	})).Return(sqlc.InventoryCategory{CategoryID: 4, Name: "X", IsActive: true}, nil)

	c, err := s.CreateCategory(ctx, "X", "", "alice")
	require.NoError(t, err)
	assert.Equal(t, "", c.Description)
}

func TestCreateCategory_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateCategory(ctx, gomock.Any()).Return(sqlc.InventoryCategory{}, errBoom)

	_, err := s.CreateCategory(ctx, "X", "", "alice")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create category:")
}

func TestGetCategoryByID(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetCategoryByID(ctx, int64(3)).Return(sqlc.InventoryCategory{
		CategoryID:  3,
		Name:        "Produce",
		Description: pgtype.Text{String: "fresh", Valid: true},
		IsActive:    true,
	}, nil)

	c, err := s.GetCategoryByID(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, "fresh", c.Description)
	assert.True(t, c.IsActive)
}

func TestGetCategoryByID_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetCategoryByID(ctx, int64(3)).Return(sqlc.InventoryCategory{}, errBoom)

	_, err := s.GetCategoryByID(ctx, 3)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "get category by id:")
}

func TestListCategories(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListCategories(ctx).Return([]sqlc.InventoryCategory{
		{CategoryID: 1, Name: "A", IsActive: true},
	}, nil)

	cats, err := s.ListCategories(ctx)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	assert.Equal(t, "A", cats[0].Name)
}

func TestListCategories_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListCategories(ctx).Return(nil, errBoom)

	_, err := s.ListCategories(ctx)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list categories:")
}

func TestUpdateCategory(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		CategoryID:  3,
		Name:        "Produce",
		Description: pgtype.Text{String: "ripe", Valid: true},
		IsActive:    false,
		UpdatedBy:   pgtype.Text{String: "bob", Valid: true},
	}).Return(sqlc.InventoryCategory{
		CategoryID:  3,
		Name:        "Produce",
		Description: pgtype.Text{String: "ripe", Valid: true},
		IsActive:    false,
	}, nil)

	c, err := s.UpdateCategory(ctx, 3, "Produce", "ripe", false, "bob")
	require.NoError(t, err)
	assert.Equal(t, "ripe", c.Description)
	assert.False(t, c.IsActive)
}

func TestUpdateCategory_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateCategory(ctx, gomock.Any()).Return(sqlc.InventoryCategory{}, errBoom)

	_, err := s.UpdateCategory(ctx, 3, "Produce", "ripe", false, "bob")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "update category:")
}

func TestDeleteCategory(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteCategory(ctx, int64(3)).Return(nil)
	assert.NoError(t, s.DeleteCategory(ctx, 3))

	q.EXPECT().DeleteCategory(ctx, int64(9)).Return(errBoom)
	assert.ErrorIs(t, s.DeleteCategory(ctx, 9), errBoom)
}

func TestCreateItem(t *testing.T) {
	ctx := context.Background()
	brandID := int64(5)

	s, q := newTestService(t)
	q.EXPECT().CreateItem(ctx, sqlc.CreateItemParams{
		Name:       "Apple",
		BrandID:    pgtype.Int8{Int64: 5, Valid: true},
		Upc12:      pgtype.Text{String: "012345678901", Valid: true},
		Upc14:      pgtype.Text{},
		CategoryID: 3,
		Unit:       "each",
		CreatedBy:  "alice",
		UpdatedBy:  pgtype.Text{String: "alice", Valid: true},
	}).Return(sqlc.InventoryItem{
		ItemID:     11,
		Name:       "Apple",
		BrandID:    pgtype.Int8{Int64: 5, Valid: true},
		Upc12:      pgtype.Text{String: "012345678901", Valid: true},
		CategoryID: 3,
		Unit:       "each",
	}, nil)

	it, err := s.CreateItem(ctx, Item{
		Name:       "Apple",
		BrandID:    &brandID,
		Upc12:      "012345678901",
		CategoryID: 3,
		Unit:       "each",
	}, "alice")
	require.NoError(t, err)
	assert.Equal(t, int64(11), it.ItemID)
	require.NotNil(t, it.BrandID)
	assert.Equal(t, int64(5), *it.BrandID)
	assert.Equal(t, "012345678901", it.Upc12)
	assert.Equal(t, "", it.Upc14)
}

func TestCreateItem_NullBrand(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateItem(ctx, gomock.Cond(func(arg sqlc.CreateItemParams) bool {
		return !arg.BrandID.Valid
	})).Return(sqlc.InventoryItem{ItemID: 12, Name: "X"}, nil)

	it, err := s.CreateItem(ctx, Item{Name: "X"}, "alice")
	require.NoError(t, err)
	assert.Nil(t, it.BrandID)
}

func TestCreateItem_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateItem(ctx, gomock.Any()).Return(sqlc.InventoryItem{}, errBoom)

	_, err := s.CreateItem(ctx, Item{Name: "X"}, "alice")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create item:")
}

func TestGetItemByID(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetItemByID(ctx, int64(11)).Return(sqlc.InventoryItem{
		ItemID:     11,
		Name:       "Apple",
		BrandID:    pgtype.Int8{Int64: 5, Valid: true},
		Upc12:      pgtype.Text{String: "012345678901", Valid: true},
		Upc14:      pgtype.Text{String: "12345678901234", Valid: true},
		CategoryID: 3,
		Unit:       "each",
	}, nil)

	it, err := s.GetItemByID(ctx, 11)
	require.NoError(t, err)
	assert.Equal(t, "Apple", it.Name)
	require.NotNil(t, it.BrandID)
	assert.Equal(t, int64(5), *it.BrandID)
	assert.Equal(t, "012345678901", it.Upc12)
	assert.Equal(t, "12345678901234", it.Upc14)
}

func TestGetItemByID_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetItemByID(ctx, int64(11)).Return(sqlc.InventoryItem{}, errBoom)

	_, err := s.GetItemByID(ctx, 11)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "get item by id:")
}

func TestListItems(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListItems(ctx, sqlc.ListItemsParams{Limit: 10, Offset: 20}).
		Return([]sqlc.InventoryItem{{ItemID: 1, Name: "A"}}, nil)

	items, err := s.ListItems(ctx, 10, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "A", items[0].Name)
}

func TestListItems_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListItems(ctx, gomock.Any()).Return(nil, errBoom)

	_, err := s.ListItems(ctx, 10, 20)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list items:")
}

func TestUpdateItem(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateItem(ctx, sqlc.UpdateItemParams{
		ItemID:     11,
		Name:       "Pear",
		BrandID:    pgtype.Int8{},
		Upc12:      pgtype.Text{},
		Upc14:      pgtype.Text{},
		CategoryID: 3,
		Unit:       "kg",
		UpdatedBy:  pgtype.Text{String: "bob", Valid: true},
	}).Return(nil)

	err := s.UpdateItem(ctx, 11, Item{Name: "Pear", CategoryID: 3, Unit: "kg"}, "bob")
	assert.NoError(t, err)
}

func TestUpdateItem_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateItem(ctx, gomock.Any()).Return(errBoom)

	assert.ErrorIs(t, s.UpdateItem(ctx, 11, Item{}, "bob"), errBoom)
}

func TestDeleteItem(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteItem(ctx, int64(11)).Return(nil)
	assert.NoError(t, s.DeleteItem(ctx, 11))

	q.EXPECT().DeleteItem(ctx, int64(9)).Return(errBoom)
	assert.ErrorIs(t, s.DeleteItem(ctx, 9), errBoom)
}

func TestCreateFlavorProfile(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s, q := newTestService(t)
	q.EXPECT().CreateFlavorProfile(ctx, sqlc.CreateFlavorProfileParams{
		Name:      "Sweet",
		IsActive:  true,
		CreatedBy: "alice",
		UpdatedBy: pgtype.Text{String: "alice", Valid: true},
	}).Return(sqlc.InventoryFlavorProfile{
		FlavorID:  2,
		Name:      "Sweet",
		IsActive:  true,
		CreatedAt: now,
	}, nil)

	f, err := s.CreateFlavorProfile(ctx, "Sweet", "alice")
	require.NoError(t, err)
	assert.Equal(t, FlavorProfile{FlavorID: 2, Name: "Sweet", IsActive: true, CreatedAt: now}, f)
}

func TestCreateFlavorProfile_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateFlavorProfile(ctx, gomock.Any()).Return(sqlc.InventoryFlavorProfile{}, errBoom)

	_, err := s.CreateFlavorProfile(ctx, "Sweet", "alice")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create flavor profile:")
}

func TestGetFlavorProfileByID(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetFlavorProfileByID(ctx, int64(2)).
		Return(sqlc.InventoryFlavorProfile{FlavorID: 2, Name: "Sour", IsActive: true}, nil)

	f, err := s.GetFlavorProfileByID(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, "Sour", f.Name)
}

func TestGetFlavorProfileByID_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetFlavorProfileByID(ctx, int64(2)).Return(sqlc.InventoryFlavorProfile{}, errBoom)

	_, err := s.GetFlavorProfileByID(ctx, 2)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "get flavor profile by id:")
}

func TestListFlavorProfiles(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListFlavorProfiles(ctx).Return([]sqlc.InventoryFlavorProfile{
		{FlavorID: 1, Name: "Bitter", IsActive: true},
		{FlavorID: 2, Name: "Sour", IsActive: false},
	}, nil)

	fps, err := s.ListFlavorProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, fps, 2)
	assert.False(t, fps[1].IsActive)
}

func TestListFlavorProfiles_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListFlavorProfiles(ctx).Return(nil, errBoom)

	_, err := s.ListFlavorProfiles(ctx)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list flavor profiles:")
}

func TestUpdateFlavorProfile(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateFlavorProfile(ctx, sqlc.UpdateFlavorProfileParams{
		FlavorID:  2,
		Name:      "Umami",
		IsActive:  false,
		UpdatedBy: pgtype.Text{String: "bob", Valid: true},
	}).Return(sqlc.InventoryFlavorProfile{FlavorID: 2, Name: "Umami", IsActive: false}, nil)

	f, err := s.UpdateFlavorProfile(ctx, 2, "Umami", false, "bob")
	require.NoError(t, err)
	assert.Equal(t, "Umami", f.Name)
	assert.False(t, f.IsActive)
}

func TestUpdateFlavorProfile_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateFlavorProfile(ctx, gomock.Any()).Return(sqlc.InventoryFlavorProfile{}, errBoom)

	_, err := s.UpdateFlavorProfile(ctx, 2, "Umami", false, "bob")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "update flavor profile:")
}

func TestDeleteFlavorProfile(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteFlavorProfile(ctx, int64(2)).Return(nil)
	assert.NoError(t, s.DeleteFlavorProfile(ctx, 2))

	q.EXPECT().DeleteFlavorProfile(ctx, int64(9)).Return(errBoom)
	assert.ErrorIs(t, s.DeleteFlavorProfile(ctx, 9), errBoom)
}

func TestCreateNutrientType(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s, q := newTestService(t)
	q.EXPECT().CreateNutrientType(ctx, sqlc.CreateNutrientTypeParams{
		Name: "Sodium",
		Unit: pgtype.Text{String: "mg", Valid: true},
	}).Return(sqlc.InventoryNutrientType{
		NutrientID: 4,
		Name:       "Sodium",
		Unit:       pgtype.Text{String: "mg", Valid: true},
		CreatedAt:  now,
	}, nil)

	n, err := s.CreateNutrientType(ctx, "Sodium", "mg")
	require.NoError(t, err)
	assert.Equal(t, NutrientType{NutrientID: 4, Name: "Sodium", Unit: "mg", CreatedAt: now}, n)
}

func TestCreateNutrientType_EmptyUnit(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateNutrientType(ctx, gomock.Cond(func(arg sqlc.CreateNutrientTypeParams) bool {
		return !arg.Unit.Valid
	})).Return(sqlc.InventoryNutrientType{NutrientID: 5, Name: "X"}, nil)

	n, err := s.CreateNutrientType(ctx, "X", "")
	require.NoError(t, err)
	assert.Equal(t, "", n.Unit)
}

func TestCreateNutrientType_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateNutrientType(ctx, gomock.Any()).Return(sqlc.InventoryNutrientType{}, errBoom)

	_, err := s.CreateNutrientType(ctx, "X", "")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create nutrient type:")
}

func TestGetNutrientTypeByID(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetNutrientTypeByID(ctx, int64(4)).Return(sqlc.InventoryNutrientType{
		NutrientID: 4,
		Name:       "Sodium",
		Unit:       pgtype.Text{String: "mg", Valid: true},
	}, nil)

	n, err := s.GetNutrientTypeByID(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, "mg", n.Unit)
}

func TestGetNutrientTypeByID_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().GetNutrientTypeByID(ctx, int64(4)).Return(sqlc.InventoryNutrientType{}, errBoom)

	_, err := s.GetNutrientTypeByID(ctx, 4)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "get nutrient type by id:")
}

func TestListNutrientTypes(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListNutrientTypes(ctx).Return([]sqlc.InventoryNutrientType{
		{NutrientID: 1, Name: "Calories", Unit: pgtype.Text{String: "kcal", Valid: true}},
	}, nil)

	nts, err := s.ListNutrientTypes(ctx)
	require.NoError(t, err)
	require.Len(t, nts, 1)
	assert.Equal(t, "kcal", nts[0].Unit)
}

func TestListNutrientTypes_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListNutrientTypes(ctx).Return(nil, errBoom)

	_, err := s.ListNutrientTypes(ctx)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list nutrient types:")
}

func TestUpdateNutrientType(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateNutrientType(ctx, sqlc.UpdateNutrientTypeParams{
		NutrientID: 4,
		Name:       "Salt",
		Unit:       pgtype.Text{String: "g", Valid: true},
	}).Return(sqlc.InventoryNutrientType{
		NutrientID: 4,
		Name:       "Salt",
		Unit:       pgtype.Text{String: "g", Valid: true},
	}, nil)

	n, err := s.UpdateNutrientType(ctx, 4, "Salt", "g")
	require.NoError(t, err)
	assert.Equal(t, "g", n.Unit)
}

func TestUpdateNutrientType_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().UpdateNutrientType(ctx, gomock.Any()).Return(sqlc.InventoryNutrientType{}, errBoom)

	_, err := s.UpdateNutrientType(ctx, 4, "Salt", "g")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "update nutrient type:")
}

func TestDeleteNutrientType(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteNutrientType(ctx, int64(4)).Return(nil)
	assert.NoError(t, s.DeleteNutrientType(ctx, 4))

	q.EXPECT().DeleteNutrientType(ctx, int64(9)).Return(errBoom)
	assert.ErrorIs(t, s.DeleteNutrientType(ctx, 9), errBoom)
}

func TestListFoodNutrientsByItem(t *testing.T) {
	ctx := context.Background()
	amount := numericFromFloat64(12.5)

	s, q := newTestService(t)
	q.EXPECT().ListFoodNutrientsByItem(ctx, int64(11)).Return([]sqlc.ListFoodNutrientsByItemRow{
		{NutrientID: 4, Name: "Sodium", Unit: pgtype.Text{String: "mg", Valid: true}, Amount: amount},
	}, nil)

	fns, err := s.ListFoodNutrientsByItem(ctx, 11)
	require.NoError(t, err)
	require.Len(t, fns, 1)
	assert.Equal(t, int64(4), fns[0].NutrientID)
	assert.Equal(t, "Sodium", fns[0].Name)
	assert.Equal(t, "mg", fns[0].Unit)
	assert.InDelta(t, 12.5, fns[0].Amount, 1e-9)
}

func TestListFoodNutrientsByItem_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListFoodNutrientsByItem(ctx, int64(11)).Return(nil, errBoom)

	_, err := s.ListFoodNutrientsByItem(ctx, 11)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list food nutrients by item:")
}

func TestCreateFoodNutrient(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateFoodNutrient(ctx, gomock.Cond(func(arg sqlc.CreateFoodNutrientParams) bool {
		return arg.FoodID == 11 && arg.NutrientID == 4 && arg.CreatedBy == "alice" &&
			numericToFloat64(arg.Amount) == 12.5
	})).Return(sqlc.InventoryFoodNutrient{
		NutrientID: 4,
		Amount:     numericFromFloat64(12.5),
	}, nil)

	fn, err := s.CreateFoodNutrient(ctx, 11, 4, 12.5, "alice")
	require.NoError(t, err)
	assert.Equal(t, int64(4), fn.NutrientID)
	assert.InDelta(t, 12.5, fn.Amount, 1e-9)
}

func TestCreateFoodNutrient_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateFoodNutrient(ctx, gomock.Any()).Return(sqlc.InventoryFoodNutrient{}, errBoom)

	_, err := s.CreateFoodNutrient(ctx, 11, 4, 12.5, "alice")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create food nutrient:")
}

func TestDeleteFoodNutrient(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteFoodNutrient(ctx, sqlc.DeleteFoodNutrientParams{FoodID: 11, NutrientID: 4}).Return(nil)
	assert.NoError(t, s.DeleteFoodNutrient(ctx, 11, 4))

	q.EXPECT().DeleteFoodNutrient(ctx, sqlc.DeleteFoodNutrientParams{FoodID: 11, NutrientID: 9}).Return(errBoom)
	assert.ErrorIs(t, s.DeleteFoodNutrient(ctx, 11, 9), errBoom)
}

func TestCreateFoodFlavor(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateFoodFlavor(ctx, sqlc.CreateFoodFlavorParams{
		FoodID:    11,
		FlavorID:  2,
		Intensity: 3,
		CreatedBy: "alice",
	}).Return(sqlc.InventoryFoodFlavor{FlavorID: 2, Intensity: 3}, nil)

	ff, err := s.CreateFoodFlavor(ctx, 11, 2, 3, "alice")
	require.NoError(t, err)
	assert.Equal(t, FoodFlavor{FlavorID: 2, Intensity: 3}, ff)
}

func TestCreateFoodFlavor_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().CreateFoodFlavor(ctx, gomock.Any()).Return(sqlc.InventoryFoodFlavor{}, errBoom)

	_, err := s.CreateFoodFlavor(ctx, 11, 2, 3, "alice")
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "create food flavor:")
}

func TestListFoodFlavorsByItem(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListFoodFlavorsByItem(ctx, int64(11)).Return([]sqlc.ListFoodFlavorsByItemRow{
		{FlavorID: 1, Name: "Salty", Intensity: 2},
		{FlavorID: 2, Name: "Sweet", Intensity: 5},
	}, nil)

	ffs, err := s.ListFoodFlavorsByItem(ctx, 11)
	require.NoError(t, err)
	require.Len(t, ffs, 2)
	assert.Equal(t, "Salty", ffs[0].Name)
	assert.Equal(t, int16(5), ffs[1].Intensity)
}

func TestListFoodFlavorsByItem_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().ListFoodFlavorsByItem(ctx, int64(11)).Return(nil, errBoom)

	_, err := s.ListFoodFlavorsByItem(ctx, 11)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "list food flavors by item:")
}

func TestDeleteFoodFlavor(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)
	q.EXPECT().DeleteFoodFlavor(ctx, sqlc.DeleteFoodFlavorParams{FoodID: 11, FlavorID: 2}).Return(nil)
	assert.NoError(t, s.DeleteFoodFlavor(ctx, 11, 2))

	q.EXPECT().DeleteFoodFlavor(ctx, sqlc.DeleteFoodFlavorParams{FoodID: 11, FlavorID: 9}).Return(errBoom)
	assert.ErrorIs(t, s.DeleteFoodFlavor(ctx, 11, 9), errBoom)
}
