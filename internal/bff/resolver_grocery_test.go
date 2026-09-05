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
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

var errGrocBoom = errors.New("grocery boom")

const (
	grocUserID = int64(7)
	grocEmail  = "groc@example.com"
)

func grocCtx() context.Context {
	return testenv.WithUser(context.Background(), grocUserID, grocEmail)
}

func TestResolver_GroceryList_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	genAt := time.Date(2025, 2, 3, 10, 30, 0, 0, time.UTC)
	list := grocery.GroceryList{GroceryListID: 11, UserID: grocUserID, GeneratedAt: genAt}
	g.EXPECT().GetGroceryListByID(gomock.Any(), int64(11), grocUserID).Return(list, nil)

	res, err := r.GroceryList(grocCtx(), struct{ ID graphql.ID }{ID: "11"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("11"), res.ID())
	assert.True(t, res.GeneratedAt().Equal(genAt))
}

func TestResolver_GroceryList_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.GroceryList(context.Background(), struct{ ID graphql.ID }{ID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_GroceryList_InvalidID(t *testing.T) {
	r := &Resolver{}
	res, err := r.GroceryList(grocCtx(), struct{ ID graphql.ID }{ID: "abc"})
	assert.Nil(t, res)
	assert.Error(t, err)
}

func TestResolver_GroceryList_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().GetGroceryListByID(gomock.Any(), int64(11), grocUserID).Return(grocery.GroceryList{}, errGrocBoom)

	res, err := r.GroceryList(grocCtx(), struct{ ID graphql.ID }{ID: "11"})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_GroceryLists_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	lists := []grocery.GroceryList{
		{GroceryListID: 11, UserID: grocUserID},
		{GroceryListID: 12, UserID: grocUserID},
	}
	// page=2, pageSize=10 -> limit=10, offset=10
	g.EXPECT().ListGroceryLists(gomock.Any(), grocUserID, int32(10), int32(10)).Return(lists, nil)

	res, err := r.GroceryLists(grocCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 2, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, res)

	items := res.Items()
	require.Len(t, items, 2)
	assert.Equal(t, graphql.ID("11"), items[0].ID())
	assert.Equal(t, graphql.ID("12"), items[1].ID())

	pi := res.PageInfo()
	assert.Equal(t, int32(2), pi.PageNumber())
	assert.Equal(t, int32(10), pi.PageSize())
	assert.Equal(t, int32(2), pi.TotalCount())
}

func TestResolver_GroceryLists_ClampsPageArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	// page clamps to 1, pageSize clamps to 100.
	g.EXPECT().ListGroceryLists(gomock.Any(), grocUserID, int32(100), int32(0)).Return(nil, nil)

	res, err := r.GroceryLists(grocCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 0, PageSize: 5000})
	require.NoError(t, err)
	assert.Equal(t, int32(1), res.PageInfo().PageNumber())
	assert.Equal(t, int32(100), res.PageInfo().PageSize())
}

func TestResolver_GroceryLists_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.GroceryLists(context.Background(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_GroceryLists_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().ListGroceryLists(gomock.Any(), grocUserID, int32(10), int32(0)).Return(nil, errGrocBoom)

	res, err := r.GroceryLists(grocCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_GroceryList_ItemsSubResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)
	r := &Resolver{GroceryService: g, InventoryService: inv}

	g.EXPECT().GetGroceryListByID(gomock.Any(), int64(11), grocUserID).
		Return(grocery.GroceryList{GroceryListID: 11, UserID: grocUserID}, nil)

	listRes, err := r.GroceryList(grocCtx(), struct{ ID graphql.ID }{ID: "11"})
	require.NoError(t, err)

	itemID := int64(42)
	g.EXPECT().ListGroceryListItems(gomock.Any(), int64(11)).Return([]grocery.GroceryListItem{
		{GroceryListItemID: 100, GroceryListID: 11, ItemID: &itemID, QuantityNeeded: 2.5, UnitOfMeasure: "cups", Source: "recipe"},
		{GroceryListItemID: 101, GroceryListID: 11, ManualItemName: "Bananas", QuantityNeeded: 3, Source: "manual", IsChecked: true},
	}, nil)

	items, err := listRes.Items(grocCtx())
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, graphql.ID("100"), items[0].ID())
	assert.Nil(t, items[0].ManualItemName())
	assert.Equal(t, 2.5, items[0].QuantityNeeded())
	assert.Equal(t, "cups", *items[0].UnitOfMeasure())
	assert.Equal(t, "recipe", items[0].Source())
	assert.False(t, items[0].IsChecked())

	assert.Equal(t, "Bananas", *items[1].ManualItemName())
	assert.True(t, items[1].IsChecked())

	// Nested item lookup goes through InventoryService for catalog items.
	inv.EXPECT().GetItemByID(gomock.Any(), itemID).Return(inventory.Item{ItemID: itemID, Name: "Flour"}, nil)
	cat, err := items[0].Item(grocCtx())
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("42"), cat.ID())
	assert.Equal(t, "Flour", cat.Name())

	// Manual items have no catalog item.
	catItem, err := items[1].Item(grocCtx())
	require.NoError(t, err)
	assert.Nil(t, catItem)
}

func TestResolver_GroceryList_ItemsSubResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)

	lr := &groceryListResolver{g: g, list: grocery.GroceryList{GroceryListID: 11}}
	g.EXPECT().ListGroceryListItems(gomock.Any(), int64(11)).Return(nil, errGrocBoom)

	items, err := lr.Items(context.Background())
	assert.Nil(t, items)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_GenerateGroceryList_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	mealPlanID := int64(55)
	g.EXPECT().Generate(gomock.Any(), grocUserID, mealPlanID, grocEmail).
		Return(grocery.GroceryList{GroceryListID: 21, UserID: grocUserID, MealPlanID: &mealPlanID}, nil)

	res, err := r.GenerateGroceryList(grocCtx(), struct{ MealPlanID graphql.ID }{MealPlanID: "55"})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("21"), res.ID())
}

func TestResolver_GenerateGroceryList_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.GenerateGroceryList(context.Background(), struct{ MealPlanID graphql.ID }{MealPlanID: "55"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_GenerateGroceryList_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().Generate(gomock.Any(), grocUserID, int64(55), grocEmail).Return(grocery.GroceryList{}, errGrocBoom)

	res, err := r.GenerateGroceryList(grocCtx(), struct{ MealPlanID graphql.ID }{MealPlanID: "55"})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_AddGroceryItem_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	itemID := graphql.ID("42")
	manual := "Bananas"
	want := grocery.GroceryListItem{
		GroceryListID:  11,
		ItemID:         ptrToGrocInt64(42),
		ManualItemName: "Bananas",
		QuantityNeeded: 2,
		UnitOfMeasure:  "bunch",
		Source:         "manual",
	}
	g.EXPECT().AddGroceryListItem(gomock.Any(), gomock.Eq(want), grocEmail).
		Return(grocery.GroceryListItem{GroceryListItemID: 100, GroceryListID: 11, ItemID: ptrToGrocInt64(42), ManualItemName: "Bananas", QuantityNeeded: 2, UnitOfMeasure: "bunch", Source: "manual"}, nil)

	res, err := r.AddGroceryItem(grocCtx(), struct{ Input addGroceryItemInput }{
		Input: addGroceryItemInput{
			GroceryListID:  "11",
			ItemID:         &itemID,
			ManualItemName: &manual,
			Quantity:       2,
			Unit:           "bunch",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("100"), res.ID())
	assert.Equal(t, "Bananas", *res.ManualItemName())
	assert.Equal(t, "manual", res.Source())
}

func TestResolver_AddGroceryItem_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.AddGroceryItem(context.Background(), struct{ Input addGroceryItemInput }{
		Input: addGroceryItemInput{GroceryListID: "11"},
	})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_AddGroceryItem_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().AddGroceryListItem(gomock.Any(), gomock.Any(), grocEmail).Return(grocery.GroceryListItem{}, errGrocBoom)

	res, err := r.AddGroceryItem(grocCtx(), struct{ Input addGroceryItemInput }{
		Input: addGroceryItemInput{GroceryListID: "11", Quantity: 1},
	})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_ToggleGroceryItemChecked_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	before := grocery.GroceryListItem{GroceryListItemID: 100, GroceryListID: 11, ManualItemName: "Eggs", IsChecked: false}
	flipped := before
	flipped.IsChecked = true
	after := flipped

	gomock.InOrder(
		g.EXPECT().GetGroceryListItemByID(gomock.Any(), int64(100)).Return(before, nil),
		g.EXPECT().UpdateGroceryListItem(gomock.Any(), int64(100), gomock.Eq(flipped), grocEmail).Return(nil),
		g.EXPECT().GetGroceryListItemByID(gomock.Any(), int64(100)).Return(after, nil),
	)

	res, err := r.ToggleGroceryItemChecked(grocCtx(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("100"), res.ID())
	assert.True(t, res.IsChecked())
}

func TestResolver_ToggleGroceryItemChecked_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.ToggleGroceryItemChecked(context.Background(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_ToggleGroceryItemChecked_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().GetGroceryListItemByID(gomock.Any(), int64(100)).Return(grocery.GroceryListItem{}, errGrocBoom)

	res, err := r.ToggleGroceryItemChecked(grocCtx(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_ToggleGroceryItemChecked_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	before := grocery.GroceryListItem{GroceryListItemID: 100, IsChecked: true}
	flipped := before
	flipped.IsChecked = false
	g.EXPECT().GetGroceryListItemByID(gomock.Any(), int64(100)).Return(before, nil)
	g.EXPECT().UpdateGroceryListItem(gomock.Any(), int64(100), gomock.Eq(flipped), grocEmail).Return(errGrocBoom)

	res, err := r.ToggleGroceryItemChecked(grocCtx(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_DeleteGroceryItem_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().DeleteGroceryListItem(gomock.Any(), int64(100)).Return(nil)

	ok, err := r.DeleteGroceryItem(grocCtx(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_DeleteGroceryItem_Unauthorized(t *testing.T) {
	r := &Resolver{}
	ok, err := r.DeleteGroceryItem(context.Background(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	assert.False(t, ok)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_DeleteGroceryItem_InvalidID(t *testing.T) {
	r := &Resolver{}
	ok, err := r.DeleteGroceryItem(grocCtx(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "nope"})
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestResolver_DeleteGroceryItem_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().DeleteGroceryListItem(gomock.Any(), int64(100)).Return(errGrocBoom)

	ok, err := r.DeleteGroceryItem(grocCtx(), struct{ GroceryListItemID graphql.ID }{GroceryListItemID: "100"})
	assert.False(t, ok)
	assert.ErrorIs(t, err, errGrocBoom)
}

func ptrToGrocInt64(v int64) *int64 { return &v }
