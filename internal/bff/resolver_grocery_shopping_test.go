package bff

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/instacartclient"
)

func TestShoppingLineItems_NamePrecedence(t *testing.T) {
	itemID, ingID, ingWithUsual, unitID := int64(42), int64(7), int64(8), int64(3)
	gc := &groceryChildren{
		itemsByList: map[int64][]grocery.GroceryListItem{11: {
			{GroceryListItemID: 1, GroceryListID: 11, ItemID: &itemID, QuantityNeeded: 2, UnitID: &unitID},
			{GroceryListItemID: 2, GroceryListID: 11, IngredientID: &ingWithUsual, QuantityNeeded: 1},
			{GroceryListItemID: 3, GroceryListID: 11, IngredientID: &ingID, QuantityNeeded: 4},
			{GroceryListItemID: 4, GroceryListID: 11, ManualItemName: " Birthday candles "},
		}},
		items: map[int64]inventory.Item{
			42: {ItemID: 42, Name: "All-Purpose Flour"},
			55: {ItemID: 55, Name: "Organic Whole Milk"},
		},
		units:       map[int64]inventory.Unit{3: {UnitID: 3, Name: "cup", Kind: "volume"}},
		ingredients: map[int64]inventory.Ingredient{7: {IngredientID: 7, Name: "Bananas"}, 8: {IngredientID: 8, Name: "Milk"}},
		usualItems:  map[int64]int64{8: 55},
	}

	lines := shoppingLineItems(gc, 11, false)
	require.Len(t, lines, 4)
	assert.Equal(t, "All-Purpose Flour", lines[0].Name)
	assert.Equal(t, "Organic Whole Milk", lines[1].Name) // usual-brand item beats the ingredient name
	assert.Equal(t, "Bananas", lines[2].Name)
	assert.Equal(t, "Birthday candles", lines[3].Name)
	assert.Equal(t, "2 cup All-Purpose Flour", lines[0].DisplayText)
	assert.Equal(t, "Birthday candles", lines[3].DisplayText)
}

func TestShoppingLineItems_CheckedFiltered(t *testing.T) {
	gc := &groceryChildren{
		itemsByList: map[int64][]grocery.GroceryListItem{11: {
			{GroceryListItemID: 1, GroceryListID: 11, ManualItemName: "Eggs"},
			{GroceryListItemID: 2, GroceryListID: 11, ManualItemName: "Milk", IsChecked: true},
		}},
	}

	assert.Len(t, shoppingLineItems(gc, 11, false), 1)
	assert.Len(t, shoppingLineItems(gc, 11, true), 2)
}

func TestShoppingLineItems_UPCAndBrand(t *testing.T) {
	itemID, brandID, unitID := int64(42), int64(9), int64(14)
	gc := &groceryChildren{
		itemsByList: map[int64][]grocery.GroceryListItem{11: {
			{GroceryListItemID: 1, GroceryListID: 11, ItemID: &itemID, QuantityNeeded: 1, UnitID: &unitID},
		}},
		items: map[int64]inventory.Item{
			42: {ItemID: 42, Name: "Whole Milk", BrandID: &brandID, Upc12: "012345678905"},
		},
		units: map[int64]inventory.Unit{14: {UnitID: 14, Name: "gallon", Kind: "volume"}},
		ch: &itemChildren{
			brands: map[int64]inventory.Brand{9: {BrandID: 9, Name: "Horizon"}},
		},
	}

	lines := shoppingLineItems(gc, 11, false)
	require.Len(t, lines, 1)
	assert.Equal(t, []string{"012345678905"}, lines[0].UPCs)
	require.NotNil(t, lines[0].Filters)
	assert.Equal(t, []string{"Horizon"}, lines[0].Filters.BrandFilters)
	require.Len(t, lines[0].LineItemMeasurements, 1)
	assert.Equal(t, instacartclient.Measurement{Quantity: 1, Unit: "gallon"}, lines[0].LineItemMeasurements[0])
}

func TestShoppingLineItems_SkipsUnnameable(t *testing.T) {
	gc := &groceryChildren{
		itemsByList: map[int64][]grocery.GroceryListItem{11: {
			{GroceryListItemID: 1, GroceryListID: 11, ManualItemName: "  "},
			{GroceryListItemID: 2, GroceryListID: 11},
			{GroceryListItemID: 3, GroceryListID: 11, ManualItemName: "Eggs"},
		}},
	}
	lines := shoppingLineItems(gc, 11, false)
	require.Len(t, lines, 1)
	assert.Equal(t, "Eggs", lines[0].Name)
}

func TestInstacartMeasurement_UnitMapping(t *testing.T) {
	cupID, pinchID, weirdID := int64(3), int64(16), int64(99)
	gc := &groceryChildren{units: map[int64]inventory.Unit{
		3:  {UnitID: 3, Name: "cup", Abbreviation: "c", Kind: "volume"},
		16: {UnitID: 16, Name: "pinch", Kind: "count"},
		99: {UnitID: 99, Name: "hectoliter", Kind: "volume"},
	}}

	tests := []struct {
		name     string
		line     grocery.GroceryListItem
		wantUnit string
		wantOK   bool
	}{
		{"mapped unit", grocery.GroceryListItem{QuantityNeeded: 2, UnitID: &cupID}, "cup", true},
		{"unknown count unit", grocery.GroceryListItem{QuantityNeeded: 1, UnitID: &pinchID}, "each", true},
		{"unknown weight/volume omits", grocery.GroceryListItem{QuantityNeeded: 1, UnitID: &weirdID}, "", false},
		{"no unit defaults each", grocery.GroceryListItem{QuantityNeeded: 3}, "each", true},
		{"zero quantity omits", grocery.GroceryListItem{QuantityNeeded: 0}, "", false},
		{"negative quantity omits", grocery.GroceryListItem{QuantityNeeded: -1}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := instacartMeasurement(gc, tc.line)
			assert.Equal(t, tc.wantOK, ok)
			if ok {
				assert.Equal(t, tc.wantUnit, m.Unit)
			}
		})
	}
}

func shoppingMockChain(g *mock.MockGroceryService, id *mock.MockIdentityService, up *mock.MockUserPrefsService, items []grocery.GroceryListItem) {
	g.EXPECT().GetGroceryListByID(gomock.Any(), int64(11), grocUserID).
		Return(grocery.GroceryList{GroceryListID: 11, HouseholdID: grocUserID, GeneratedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}, nil)
	g.EXPECT().ListGroceryListItemsByLists(gomock.Any(), []int64{11}, grocUserID).Return(items, nil)
	// loadItemChildren always reads household members for allergy context.
	id.EXPECT().ListUsersByHousehold(gomock.Any(), grocUserID).Return(nil, nil)
	up.EXPECT().ListUserAllergensByUsers(gomock.Any(), gomock.Any()).Return(nil, nil)
}

func TestResolver_ShopperProviders(t *testing.T) {
	r := &Resolver{}
	providers, err := r.ShopperProviders(grocCtx())
	require.NoError(t, err)
	assert.Empty(t, providers)

	ctrl := gomock.NewController(t)
	r = &Resolver{ShoppingClient: mock.NewMockShoppingLinkClient(ctrl)}
	providers, err = r.ShopperProviders(grocCtx())
	require.NoError(t, err)
	assert.Equal(t, []string{"INSTACART"}, providers)

	providers, err = r.ShopperProviders(context.Background())
	assert.Nil(t, providers)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_CreateShoppingLink_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	id := mock.NewMockIdentityService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	shop := mock.NewMockShoppingLinkClient(ctrl)
	r := &Resolver{GroceryService: g, IdentityService: id, UserPrefsService: up, InventoryService: mock.NewMockInventoryService(ctrl), ShoppingClient: shop}

	shoppingMockChain(g, id, up, []grocery.GroceryListItem{
		{GroceryListItemID: 1, GroceryListID: 11, ManualItemName: "Eggs", QuantityNeeded: 12},
		{GroceryListItemID: 2, GroceryListID: 11, ManualItemName: "Milk", IsChecked: true},
	})
	shop.EXPECT().CreateShoppingList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req instacartclient.ShoppingListRequest) (string, error) {
			assert.Equal(t, "shopping_list", req.LinkType)
			require.Len(t, req.LineItems, 1)
			assert.Equal(t, "Eggs", req.LineItems[0].Name)
			return "https://shop.example/link/xyz", nil
		})

	res, err := r.CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11"})
	require.NoError(t, err)
	assert.Equal(t, "INSTACART", res.Provider())
	assert.Equal(t, "https://shop.example/link/xyz", res.URL())
}

func TestResolver_CreateShoppingLink_IncludeChecked(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	id := mock.NewMockIdentityService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	shop := mock.NewMockShoppingLinkClient(ctrl)
	r := &Resolver{GroceryService: g, IdentityService: id, UserPrefsService: up, InventoryService: mock.NewMockInventoryService(ctrl), ShoppingClient: shop}

	shoppingMockChain(g, id, up, []grocery.GroceryListItem{
		{GroceryListItemID: 2, GroceryListID: 11, ManualItemName: "Milk", IsChecked: true},
	})
	shop.EXPECT().CreateShoppingList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req instacartclient.ShoppingListRequest) (string, error) {
			require.Len(t, req.LineItems, 1)
			return "u", nil
		})

	includeChecked := true
	res, err := r.CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11", IncludeChecked: &includeChecked})
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestResolver_CreateShoppingLink_Guards(t *testing.T) {
	// No user in context.
	res, err := (&Resolver{}).CreateShoppingLink(context.Background(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")

	// Unknown provider name is rejected before the client check.
	bogus := "SHOPIFY"
	res, err = (&Resolver{}).CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11", Provider: &bogus})
	assert.Nil(t, res)
	assert.EqualError(t, err, `unknown shopping provider "SHOPIFY"`)

	// No client configured → UNAVAILABLE.
	res, err = (&Resolver{}).CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "shopping integration is not configured")
}

func TestResolver_CreateShoppingLink_NothingToShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	id := mock.NewMockIdentityService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{GroceryService: g, IdentityService: id, UserPrefsService: up, InventoryService: mock.NewMockInventoryService(ctrl), ShoppingClient: mock.NewMockShoppingLinkClient(ctrl)}

	shoppingMockChain(g, id, up, []grocery.GroceryListItem{
		{GroceryListItemID: 2, GroceryListID: 11, ManualItemName: "Milk", IsChecked: true},
	})

	res, err := r.CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "grocery list has no unchecked items to shop")
}

func TestResolver_CreateShoppingLink_UpstreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	id := mock.NewMockIdentityService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	shop := mock.NewMockShoppingLinkClient(ctrl)
	r := &Resolver{GroceryService: g, IdentityService: id, UserPrefsService: up, InventoryService: mock.NewMockInventoryService(ctrl), ShoppingClient: shop}

	shoppingMockChain(g, id, up, []grocery.GroceryListItem{
		{GroceryListItemID: 1, GroceryListID: 11, ManualItemName: "Eggs"},
	})
	// The upstream detail must never reach the client — the resolver
	// replaces it with a generic UNAVAILABLE message.
	shop.EXPECT().CreateShoppingList(gomock.Any(), gomock.Any()).
		Return("", &instacartclient.APIError{StatusCode: 401, Detail: "unauthorized key abc123"})

	res, err := r.CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "shopping provider request failed")
}

func TestResolver_CreateShoppingLink_NonAPIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	id := mock.NewMockIdentityService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	shop := mock.NewMockShoppingLinkClient(ctrl)
	r := &Resolver{GroceryService: g, IdentityService: id, UserPrefsService: up, InventoryService: mock.NewMockInventoryService(ctrl), ShoppingClient: shop}

	shoppingMockChain(g, id, up, []grocery.GroceryListItem{
		{GroceryListItemID: 1, GroceryListID: 11, ManualItemName: "Eggs"},
	})
	shop.EXPECT().CreateShoppingList(gomock.Any(), gomock.Any()).Return("", errors.New("connection refused"))

	res, err := r.CreateShoppingLink(grocCtx(), struct {
		GroceryListID  graphql.ID
		Provider       *string
		IncludeChecked *bool
	}{GroceryListID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "shopping provider request failed")
}
