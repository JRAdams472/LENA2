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
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

var upErrBoom = errors.New("userprefs boom")

const (
	upUserID = int64(9)
	upEmail  = "prefs@example.com"
)

func upCtx() context.Context {
	return testenv.WithUser(context.Background(), upUserID, upEmail)
}

func TestResolver_UserItems_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	minQty := 1.5
	purchaseAt := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(20), int32(40)).Return([]userprefs.UserItem{
		{UserItemID: 5, UserID: upUserID, ItemID: 42, CurrentQty: 3, MinQty: &minQty, PurchaseAt: &purchaseAt, Notes: "restock", IsFavorite: true},
		{UserItemID: 6, UserID: upUserID, ItemID: 43, CurrentQty: 0.5},
	}, nil)

	res, err := r.UserItems(upCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 3, PageSize: 20})
	require.NoError(t, err)
	require.NotNil(t, res)

	items := res.Items()
	require.Len(t, items, 2)
	assert.Equal(t, graphql.ID("5"), items[0].ID())
	assert.Equal(t, 3.0, items[0].CurrentQty())
	assert.Equal(t, &minQty, items[0].MinQty())
	assert.Equal(t, "restock", *items[0].Notes())
	assert.True(t, items[0].IsFavorite())
	assert.True(t, items[0].PurchaseAt().Time.Equal(purchaseAt))
	assert.Nil(t, items[0].ExpiresAt())

	assert.Equal(t, graphql.ID("6"), items[1].ID())
	assert.Nil(t, items[1].Notes())
	assert.False(t, items[1].IsFavorite())
	assert.Nil(t, items[1].PurchaseAt())

	pi := res.PageInfo()
	assert.Equal(t, int32(3), pi.PageNumber())
	assert.Equal(t, int32(20), pi.PageSize())
	assert.Equal(t, int32(2), pi.TotalCount())
}

func TestResolver_UserItems_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.UserItems(context.Background(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_UserItems_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(10), int32(0)).Return(nil, upErrBoom)

	res, err := r.UserItems(upCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, upErrBoom)
}

func TestResolver_UserBottles_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	bottleNum := int32(2)
	price := 24.99
	temp := 55.0
	up.EXPECT().ListUserBottles(gomock.Any(), upUserID, int32(10), int32(0)).Return([]userprefs.UserBottle{
		{UserBottleID: 30, UserID: upUserID, BottleID: 88, BottleNumber: &bottleNum, Quantity: 6, PurchasePrice: &price, StorageTemp: &temp, Location: "cellar", IsFavorite: true},
	}, nil)

	res, err := r.UserBottles(upCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, res)

	items := res.Items()
	require.Len(t, items, 1)
	b := items[0]
	assert.Equal(t, graphql.ID("30"), b.ID())
	assert.Equal(t, &bottleNum, b.BottleNumber())
	assert.Equal(t, int32(6), b.Quantity())
	assert.Equal(t, &price, b.PurchasePrice())
	assert.Equal(t, &temp, b.StorageTemp())
	assert.Equal(t, "cellar", *b.Location())
	assert.True(t, b.IsFavorite())
	assert.Nil(t, b.Notes())

	pi := res.PageInfo()
	assert.Equal(t, int32(1), pi.PageNumber())
	assert.Equal(t, int32(10), pi.PageSize())
	assert.Equal(t, int32(1), pi.TotalCount())
}

func TestResolver_UserBottles_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.UserBottles(context.Background(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_UserBottles_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserBottles(gomock.Any(), upUserID, int32(10), int32(0)).Return(nil, upErrBoom)

	res, err := r.UserBottles(upCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, upErrBoom)
}

func TestResolver_AdjustUserItem_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	minQty := 1.0
	expiresAt := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	existing := userprefs.UserItem{
		UserItemID: 5, UserID: upUserID, ItemID: 42, CurrentQty: 1,
		MinQty: &minQty, ExpiresAt: &expiresAt, Notes: "keep", IsFavorite: true,
	}
	purchaseAt := time.Date(2025, 2, 20, 0, 0, 0, 0, time.UTC)
	wantArg := userprefs.UserItem{
		UserID: upUserID, ItemID: 42, CurrentQty: 4.5,
		MinQty: &minQty, PurchaseAt: &purchaseAt, ExpiresAt: &expiresAt, Notes: "keep", IsFavorite: true,
	}
	updated := wantArg
	updated.UserItemID = 5

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).Return([]userprefs.UserItem{existing}, nil)
	up.EXPECT().UpsertUserItem(gomock.Any(), gomock.Eq(wantArg), upEmail).Return(updated, nil)

	res, err := r.AdjustUserItem(upCtx(), struct {
		ItemID     graphql.ID
		Quantity   float64
		PurchaseAt *graphql.Time
	}{ItemID: "42", Quantity: 4.5, PurchaseAt: &graphql.Time{Time: purchaseAt}})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("5"), res.ID())
	assert.Equal(t, 4.5, res.CurrentQty())
	assert.True(t, res.IsFavorite())
}

func TestResolver_AdjustUserItem_NoExisting(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).Return(nil, nil)
	up.EXPECT().UpsertUserItem(gomock.Any(), gomock.Eq(userprefs.UserItem{
		UserID: upUserID, ItemID: 42, CurrentQty: 2,
	}), upEmail).Return(userprefs.UserItem{UserItemID: 7, UserID: upUserID, ItemID: 42, CurrentQty: 2}, nil)

	res, err := r.AdjustUserItem(upCtx(), struct {
		ItemID     graphql.ID
		Quantity   float64
		PurchaseAt *graphql.Time
	}{ItemID: "42", Quantity: 2})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("7"), res.ID())
	assert.False(t, res.IsFavorite())
}

func TestResolver_AdjustUserItem_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.AdjustUserItem(context.Background(), struct {
		ItemID     graphql.ID
		Quantity   float64
		PurchaseAt *graphql.Time
	}{ItemID: "42", Quantity: 1})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_AdjustUserItem_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).Return(nil, nil)
	up.EXPECT().UpsertUserItem(gomock.Any(), gomock.Any(), upEmail).Return(userprefs.UserItem{}, upErrBoom)

	res, err := r.AdjustUserItem(upCtx(), struct {
		ItemID     graphql.ID
		Quantity   float64
		PurchaseAt *graphql.Time
	}{ItemID: "42", Quantity: 1})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, upErrBoom)
}

func TestResolver_SetItemFavorite_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	existing := userprefs.UserItem{UserItemID: 5, UserID: upUserID, ItemID: 42, CurrentQty: 3, Notes: "n"}
	wantArg := userprefs.UserItem{
		UserID: upUserID, ItemID: 42, CurrentQty: 3, Notes: "n", IsFavorite: true,
	}
	updated := wantArg
	updated.UserItemID = 5

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).Return([]userprefs.UserItem{existing}, nil)
	up.EXPECT().UpsertUserItem(gomock.Any(), gomock.Eq(wantArg), upEmail).Return(updated, nil)

	res, err := r.SetItemFavorite(upCtx(), struct {
		ItemID     graphql.ID
		IsFavorite bool
	}{ItemID: "42", IsFavorite: true})
	require.NoError(t, err)
	assert.True(t, res.IsFavorite())
	assert.Equal(t, 3.0, res.CurrentQty())
}

func TestResolver_SetItemFavorite_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.SetItemFavorite(context.Background(), struct {
		ItemID     graphql.ID
		IsFavorite bool
	}{ItemID: "42", IsFavorite: true})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_DeleteUserItem_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).
		Return([]userprefs.UserItem{{UserItemID: 5, UserID: upUserID, ItemID: 42}}, nil)
	up.EXPECT().DeleteUserItem(gomock.Any(), int64(5), upUserID).Return(nil)

	ok, err := r.DeleteUserItem(upCtx(), struct{ ItemID graphql.ID }{ItemID: "42"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_DeleteUserItem_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	// ItemID 99 not present -> returns false without calling DeleteUserItem.
	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).
		Return([]userprefs.UserItem{{UserItemID: 5, UserID: upUserID, ItemID: 42}}, nil)

	ok, err := r.DeleteUserItem(upCtx(), struct{ ItemID graphql.ID }{ItemID: "99"})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestResolver_DeleteUserItem_Unauthorized(t *testing.T) {
	r := &Resolver{}
	ok, err := r.DeleteUserItem(context.Background(), struct{ ItemID graphql.ID }{ItemID: "42"})
	assert.False(t, ok)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_DeleteUserItem_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserItems(gomock.Any(), upUserID, int32(100_000), int32(0)).
		Return([]userprefs.UserItem{{UserItemID: 5, UserID: upUserID, ItemID: 42}}, nil)
	up.EXPECT().DeleteUserItem(gomock.Any(), int64(5), upUserID).Return(upErrBoom)

	ok, err := r.DeleteUserItem(upCtx(), struct{ ItemID graphql.ID }{ItemID: "42"})
	assert.False(t, ok)
	assert.ErrorIs(t, err, upErrBoom)
}

func TestResolver_AdjustUserBottle_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	bottleNum := int32(1)
	price := 30.0
	existing := userprefs.UserBottle{
		UserBottleID: 30, UserID: upUserID, BottleID: 88, BottleNumber: &bottleNum,
		Quantity: 2, PurchasePrice: &price, Location: "rack", IsFavorite: true,
	}
	wantArg := userprefs.UserBottle{
		UserID: upUserID, BottleID: 88, BottleNumber: &bottleNum, Quantity: 5,
		PurchasePrice: &price, Location: "rack", IsFavorite: true,
	}
	updated := wantArg
	updated.UserBottleID = 30

	up.EXPECT().ListUserBottles(gomock.Any(), upUserID, int32(100_000), int32(0)).Return([]userprefs.UserBottle{existing}, nil)
	up.EXPECT().UpsertUserBottle(gomock.Any(), gomock.Eq(wantArg), upEmail).Return(updated, nil)

	res, err := r.AdjustUserBottle(upCtx(), struct {
		BottleID graphql.ID
		Quantity int32
	}{BottleID: "88", Quantity: 5})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("30"), res.ID())
	assert.Equal(t, int32(5), res.Quantity())
	assert.True(t, res.IsFavorite())
	assert.Equal(t, "rack", *res.Location())
}

func TestResolver_AdjustUserBottle_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.AdjustUserBottle(context.Background(), struct {
		BottleID graphql.ID
		Quantity int32
	}{BottleID: "88", Quantity: 1})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_AdjustUserBottle_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().ListUserBottles(gomock.Any(), upUserID, int32(100_000), int32(0)).Return(nil, nil)
	up.EXPECT().UpsertUserBottle(gomock.Any(), gomock.Any(), upEmail).Return(userprefs.UserBottle{}, upErrBoom)

	res, err := r.AdjustUserBottle(upCtx(), struct {
		BottleID graphql.ID
		Quantity int32
	}{BottleID: "88", Quantity: 1})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, upErrBoom)
}

func TestResolver_SetBottleFavorite_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	existing := userprefs.UserBottle{UserBottleID: 30, UserID: upUserID, BottleID: 88, Quantity: 4, Notes: "nice"}
	wantArg := userprefs.UserBottle{
		UserID: upUserID, BottleID: 88, Quantity: 4, Notes: "nice", IsFavorite: true,
	}
	updated := wantArg
	updated.UserBottleID = 30

	up.EXPECT().ListUserBottles(gomock.Any(), upUserID, int32(100_000), int32(0)).Return([]userprefs.UserBottle{existing}, nil)
	up.EXPECT().UpsertUserBottle(gomock.Any(), gomock.Eq(wantArg), upEmail).Return(updated, nil)

	res, err := r.SetBottleFavorite(upCtx(), struct {
		BottleID   graphql.ID
		IsFavorite bool
	}{BottleID: "88", IsFavorite: true})
	require.NoError(t, err)
	assert.True(t, res.IsFavorite())
	assert.Equal(t, int32(4), res.Quantity())
}

func TestResolver_SetBottleFavorite_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.SetBottleFavorite(context.Background(), struct {
		BottleID   graphql.ID
		IsFavorite bool
	}{BottleID: "88", IsFavorite: true})
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_SetRecipeFavorite_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	// userID comes from context, not args.
	up.EXPECT().SetRecipeFavorite(gomock.Any(), upUserID, int64(77), true, upEmail).
		Return(userprefs.RecipeFavorite{UserID: upUserID, RecipeID: 77, IsFavorite: true}, nil)

	ok, err := r.SetRecipeFavorite(upCtx(), struct {
		RecipeID   graphql.ID
		IsFavorite bool
	}{RecipeID: "77", IsFavorite: true})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_SetRecipeFavorite_Unauthorized(t *testing.T) {
	r := &Resolver{}
	ok, err := r.SetRecipeFavorite(context.Background(), struct {
		RecipeID   graphql.ID
		IsFavorite bool
	}{RecipeID: "77", IsFavorite: true})
	assert.False(t, ok)
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_SetRecipeFavorite_InvalidID(t *testing.T) {
	r := &Resolver{}
	ok, err := r.SetRecipeFavorite(upCtx(), struct {
		RecipeID   graphql.ID
		IsFavorite bool
	}{RecipeID: "bad", IsFavorite: true})
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestResolver_SetRecipeFavorite_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{UserPrefsService: up}

	up.EXPECT().SetRecipeFavorite(gomock.Any(), upUserID, int64(77), false, upEmail).
		Return(userprefs.RecipeFavorite{}, upErrBoom)

	ok, err := r.SetRecipeFavorite(upCtx(), struct {
		RecipeID   graphql.ID
		IsFavorite bool
	}{RecipeID: "77", IsFavorite: false})
	assert.False(t, ok)
	assert.ErrorIs(t, err, upErrBoom)
}
