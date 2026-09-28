package bff

import (
	"context"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/notifier"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func newNotifierMock(t *testing.T) *mock.MockNotifierService {
	return mock.NewMockNotifierService(gomock.NewController(t))
}

func TestResolver_MyNotificationPreferences(t *testing.T) {
	n := newNotifierMock(t)
	muted := time.Now().Add(time.Hour)
	n.EXPECT().ListCategoryPreferences(gomock.Any(), int64(7)).Return([]notifier.CategoryPreference{
		{Category: notifier.CategoryAll, Label: "All notifications", Enabled: true},
		{Category: "expiry", Label: "Expiring pantry items", Enabled: false, MutedUntil: &muted},
	}, nil)
	r := &Resolver{NotifierService: n}

	got, err := r.MyNotificationPreferences(testutil.WithUser(context.Background(), 7, "u@example.com"))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, notifier.CategoryAll, got[0].Category())
	assert.True(t, got[0].Enabled())
	assert.Nil(t, got[0].MutedUntil())
	assert.Equal(t, "expiry", got[1].Category())
	assert.False(t, got[1].Enabled())
	require.NotNil(t, got[1].MutedUntil())

	_, err = r.MyNotificationPreferences(context.Background())
	require.ErrorContains(t, err, "unauthorized")
}

func TestResolver_SetNotificationCategoryEnabled(t *testing.T) {
	n := newNotifierMock(t)
	n.EXPECT().SetCategoryEnabled(gomock.Any(), int64(7), "expiry", false).Return(nil)
	r := &Resolver{NotifierService: n}

	ok, err := r.SetNotificationCategoryEnabled(testutil.WithUser(context.Background(), 7, "u@example.com"), struct {
		Category string
		Enabled  bool
	}{Category: "expiry", Enabled: false})
	require.NoError(t, err)
	assert.True(t, ok)

	// _all is reserved for time-bounded global mutes.
	_, err = r.SetNotificationCategoryEnabled(testutil.WithUser(context.Background(), 7, "u@example.com"), struct {
		Category string
		Enabled  bool
	}{Category: notifier.CategoryAll, Enabled: false})
	require.Error(t, err)
}

func TestResolver_MuteNotifications(t *testing.T) {
	n := newNotifierMock(t)
	until := time.Now().Add(2 * time.Hour)
	// Null category -> global mute.
	n.EXPECT().MuteCategory(gomock.Any(), int64(7), notifier.CategoryAll, gomock.Any()).Return(nil)
	// Named category passes through.
	n.EXPECT().MuteCategory(gomock.Any(), int64(7), "meal_reminders", gomock.Any()).Return(nil)
	r := &Resolver{NotifierService: n}

	cat := "meal_reminders"
	ctx := testutil.WithUser(context.Background(), 7, "u@example.com")
	_, err := r.MuteNotifications(ctx, struct {
		Category *string
		Until    graphql.Time
	}{Until: graphql.Time{Time: until}})
	require.NoError(t, err)
	_, err = r.MuteNotifications(ctx, struct {
		Category *string
		Until    graphql.Time
	}{Category: &cat, Until: graphql.Time{Time: until}})
	require.NoError(t, err)
}

func TestResolver_ClearNotificationMute(t *testing.T) {
	n := newNotifierMock(t)
	n.EXPECT().ClearMute(gomock.Any(), int64(7), "expiry").Return(nil)
	r := &Resolver{NotifierService: n}

	cat := "expiry"
	_, err := r.ClearNotificationMute(testutil.WithUser(context.Background(), 7, "u@example.com"), struct {
		Category *string
	}{Category: &cat})
	require.NoError(t, err)
}

func TestResolver_AddItemToCurrentGroceryList(t *testing.T) {
	g := mock.NewMockGroceryService(gomock.NewController(t))
	itemID := int64(9)
	g.EXPECT().ListGroceryLists(gomock.Any(), int64(7), int32(1), int32(0)).Return([]grocery.GroceryList{
		{GroceryListID: 3, HouseholdID: 7},
	}, nil)
	g.EXPECT().AddGroceryListItem(gomock.Any(), gomock.Cond(func(a grocery.GroceryListItem) bool {
		return a.GroceryListID == 3 && a.ItemID != nil && *a.ItemID == 9 &&
			a.Source == "manual" && a.QuantityNeeded == 1
	}), int64(7), "u@example.com").Return(grocery.GroceryListItem{
		GroceryListItemID: 55, GroceryListID: 3, ItemID: &itemID, Source: "manual",
	}, nil)
	r := &Resolver{GroceryService: g}

	res, err := r.AddItemToCurrentGroceryList(testutil.WithUser(context.Background(), 7, "u@example.com"), struct {
		ItemID graphql.ID
	}{ItemID: "9"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("55"), res.ID())
}

func TestResolver_AddItemToCurrentGroceryList_NoList(t *testing.T) {
	g := mock.NewMockGroceryService(gomock.NewController(t))
	g.EXPECT().ListGroceryLists(gomock.Any(), int64(7), int32(1), int32(0)).Return(nil, nil)
	r := &Resolver{GroceryService: g}

	_, err := r.AddItemToCurrentGroceryList(testutil.WithUser(context.Background(), 7, "u@example.com"), struct {
		ItemID graphql.ID
	}{ItemID: "9"})
	require.ErrorContains(t, err, "no grocery list")
}

func TestResolver_TriggerNotificationSweep_AdminOnly(t *testing.T) {
	n := newNotifierMock(t)
	n.EXPECT().Sweep(gomock.Any(), gomock.Any()).Return(5, nil)
	r := &Resolver{NotifierService: n}

	got, err := r.TriggerNotificationSweep(testutil.WithAdmin(context.Background(), 1, "admin@example.com"))
	require.NoError(t, err)
	assert.Equal(t, int32(5), got)

	// Plain member -> rejected before Sweep runs.
	_, err = r.TriggerNotificationSweep(testutil.WithUser(context.Background(), 7, "u@example.com"))
	require.Error(t, err)

	// Unauthenticated -> rejected.
	_, err = r.TriggerNotificationSweep(context.Background())
	require.Error(t, err)
}
