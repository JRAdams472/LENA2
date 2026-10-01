package bff

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/grocery"
)

func TestResolver_GroceryStores_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().ListStores(gomock.Any(), grocUserID).Return([]grocery.Store{
		{StoreID: 7, HouseholdID: grocUserID, Name: "Costco"},
		{StoreID: 9, HouseholdID: grocUserID, Name: "Aldi"},
	}, nil)

	res, err := r.GroceryStores(grocCtx())
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, graphql.ID("7"), res[0].ID())
	assert.Equal(t, "Costco", res[0].Name())
}

func TestResolver_GroceryStores_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.GroceryStores(context.Background())
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_Store_Aisles(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	sr := &storeResolver{g: g, householdID: grocUserID, store: grocery.Store{StoreID: 7, Name: "Costco"}}

	g.EXPECT().ListAisles(gomock.Any(), int64(7), grocUserID).Return([]grocery.StoreAisle{
		{AisleID: 3, StoreID: 7, Name: "Produce", Position: 0},
		{AisleID: 4, StoreID: 7, Name: "Dairy", Position: 1},
	}, nil)

	aisles, err := sr.Aisles(grocCtx())
	require.NoError(t, err)
	require.Len(t, aisles, 2)
	assert.Equal(t, "Produce", aisles[0].Name())
	assert.Equal(t, int32(1), aisles[1].Position())
}

func TestResolver_GroceryRouteGroups_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	groups := []grocery.RouteGroup{
		{
			Aisle: &grocery.StoreAisle{AisleID: 3, StoreID: 7, Name: "Produce", Position: 0},
			Items: []grocery.RouteItem{
				{Item: grocery.GroceryListItem{GroceryListItemID: 100, GroceryListID: 11, ManualItemName: "Bananas"}},
				{Item: grocery.GroceryListItem{GroceryListItemID: 101, GroceryListID: 11, ManualItemName: "Lemons"}, Suggested: true},
			},
		},
		{
			Items: []grocery.RouteItem{
				{Item: grocery.GroceryListItem{GroceryListItemID: 102, GroceryListID: 11, ManualItemName: "Misc"}},
			},
		},
	}
	g.EXPECT().RouteGroups(gomock.Any(), int64(11), grocUserID).Return(groups, nil)
	g.EXPECT().ListGroceryListItemsByLists(gomock.Any(), []int64{11}, grocUserID).Return(nil, nil)

	res, err := r.GroceryRouteGroups(grocCtx(), struct{ GroceryListID graphql.ID }{GroceryListID: "11"})
	require.NoError(t, err)
	require.Len(t, res, 2)
	require.NotNil(t, res[0].Aisle())
	assert.Equal(t, "Produce", res[0].Aisle().Name())
	require.Len(t, res[0].Items(), 2)
	assert.False(t, res[0].Items()[0].Suggested())
	assert.True(t, res[0].Items()[1].Suggested())
	assert.Nil(t, res[1].Aisle())
}

func TestResolver_GroceryRouteGroups_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.GroceryRouteGroups(context.Background(), struct{ GroceryListID graphql.ID }{GroceryListID: "11"})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_SetGroceryListStore_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	storeID := int64(7)
	list := grocery.GroceryList{GroceryListID: 11, HouseholdID: grocUserID, StoreID: &storeID}
	g.EXPECT().SetGroceryListStore(gomock.Any(), int64(11), grocUserID, &storeID, grocEmail).Return(nil)
	g.EXPECT().GetGroceryListByID(gomock.Any(), int64(11), grocUserID).Return(list, nil)
	g.EXPECT().ListGroceryListItemsByLists(gomock.Any(), []int64{11}, grocUserID).Return(nil, nil)

	res, err := r.SetGroceryListStore(grocCtx(), struct {
		GroceryListID graphql.ID
		StoreID       *graphql.ID
	}{GroceryListID: "11", StoreID: ptrGraphqlID("7")})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("11"), res.ID())
}

func TestResolver_SetGroceryListStore_ForeignStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	storeID := int64(7)
	g.EXPECT().SetGroceryListStore(gomock.Any(), int64(11), grocUserID, &storeID, grocEmail).Return(errGrocBoom)

	res, err := r.SetGroceryListStore(grocCtx(), struct {
		GroceryListID graphql.ID
		StoreID       *graphql.ID
	}{GroceryListID: "11", StoreID: ptrGraphqlID("7")})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, errGrocBoom)
}

func TestResolver_ReorderGroceryListItems_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	aisleID := int64(3)
	expected := []grocery.ReorderEntry{
		{GroceryListItemID: 101, AisleID: &aisleID},
		{GroceryListItemID: 100},
	}
	g.EXPECT().ReorderListItems(gomock.Any(), int64(11), grocUserID, expected, grocEmail).Return(nil)

	aisleIDArg := graphql.ID("3")
	ok, err := r.ReorderGroceryListItems(grocCtx(), struct {
		GroceryListID graphql.ID
		Entries       []groceryReorderEntryInput
	}{
		GroceryListID: "11",
		Entries: []groceryReorderEntryInput{
			{GroceryListItemID: "101", AisleID: &aisleIDArg},
			{GroceryListItemID: "100"},
		},
	})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_ReorderGroceryListItems_BadID(t *testing.T) {
	r := &Resolver{}
	ok, err := r.ReorderGroceryListItems(grocCtx(), struct {
		GroceryListID graphql.ID
		Entries       []groceryReorderEntryInput
	}{
		GroceryListID: "11",
		Entries:       []groceryReorderEntryInput{{GroceryListItemID: "abc"}},
	})
	assert.False(t, ok)
	require.Error(t, err)
}

func TestResolver_CreateStoreAisle_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	g := mock.NewMockGroceryService(ctrl)
	r := &Resolver{GroceryService: g}

	g.EXPECT().CreateAisle(gomock.Any(), int64(7), grocUserID, "Produce", int32(0), grocEmail).
		Return(grocery.StoreAisle{AisleID: 3, StoreID: 7, Name: "Produce", Position: 0}, nil)

	res, err := r.CreateStoreAisle(grocCtx(), struct {
		StoreID  graphql.ID
		Name     string
		Position int32
	}{StoreID: "7", Name: "Produce", Position: 0})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("3"), res.ID())
	assert.Equal(t, "Produce", res.Name())
}

func TestResolver_AssignItemToAisle_ValidatesIdentity(t *testing.T) {
	r := &Resolver{}
	ok, err := r.AssignItemToAisle(grocCtx(), struct {
		StoreID        graphql.ID
		AisleID        *graphql.ID
		ItemID         *graphql.ID
		IngredientID   *graphql.ID
		ManualItemName *string
	}{StoreID: "7"})
	assert.False(t, ok)
	require.Error(t, err)
}

func ptrGraphqlID(s string) *graphql.ID {
	id := graphql.ID(s)
	return &id
}
