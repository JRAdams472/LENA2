package notifier

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
	"github.com/JRAdams472/LENA2/internal/notifier/sqlc/mock"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq, cfg: Config{NotifyHour: 8, ExpiryDays: 3}}, mq
}

func num(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ---------- pure math ----------

func TestDefrostLeadDays(t *testing.T) {
	// Spec: 48h baseline up to and including 8 lb, then 24h per 4 lb up.
	for _, c := range []struct {
		lbs  float64
		want int
	}{
		{0.5, 2}, {4, 2}, {8, 2}, {8.1, 3}, {10, 3}, {16, 4}, {20, 5}, {40, 10},
	} {
		assert.Equal(t, c.want, defrostLeadDays(c.lbs), "lbs=%v", c.lbs)
	}
}

func TestSlotDate(t *testing.T) {
	mon := time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC) // a Monday
	row := sqlc.ListMealSlotsWithRecipesRow{
		WeekStartDate:      pgtype.Date{Time: mon, Valid: true},
		WeekStartDayOfWeek: 1, // weeks start Monday
	}
	row.DayOfWeek = 1
	assert.Equal(t, mon, slotDate(row))
	row.DayOfWeek = 3
	assert.Equal(t, mon.AddDate(0, 0, 2), slotDate(row))
	// Week wraps: Sunday (0) with Monday start is day 6.
	row.DayOfWeek = 0
	assert.Equal(t, mon.AddDate(0, 0, 6), slotDate(row))
}

func TestServingScale(t *testing.T) {
	row := sqlc.ListMealSlotsWithRecipesRow{
		Servings:       pgtype.Int4{Int32: 8, Valid: true},
		RecipeServings: pgtype.Int4{Int32: 4, Valid: true},
	}
	assert.InDelta(t, 2.0, servingScale(row), 1e-9)
	row.Servings.Valid = false
	assert.Equal(t, 1.0, servingScale(row))
	row.Servings.Valid = true
	row.RecipeServings = pgtype.Int4{Int32: 0, Valid: true}
	assert.Equal(t, 1.0, servingScale(row))
}

func TestProteinPounds(t *testing.T) {
	items := []sqlc.ListProteinItemsForRecipesRow{
		{Quantity: num(1), ToBaseFactor: num(453.592), UnitKind: pgtype.Text{String: "weight", Valid: true}},
		{Quantity: num(500), ToBaseFactor: num(1), UnitKind: pgtype.Text{String: "weight", Valid: true}},
		// count/volume units can't convert to weight — skipped.
		{Quantity: num(3), ToBaseFactor: num(1), UnitKind: pgtype.Text{String: "count", Valid: true}},
	}
	// (1 lb worth + 500 g) at 2x scale = (453.592 + 500) * 2 / 453.592 ~= 4.2 lb.
	assert.InDelta(t, (453.592+500)*2/453.592, proteinPounds(items, 2), 1e-6)
}

// ---------- suppression gate ----------

func TestAllowed_UnknownKindFailsOpen(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().GetNotificationTypeCategory(gomock.Any(), "ghost_kind").Return("", pgx.ErrNoRows)
	ok, err := svc.Allowed(context.Background(), 1, "ghost_kind")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestAllowed_SuppressionMatrix(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	for _, tc := range []struct {
		name  string
		prefs []sqlc.UserprefsNotificationPref
		want  bool
	}{
		{"no prefs -> allowed", nil, true},
		{"unrelated pref -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: "events", Enabled: false},
		}, true},
		{"category disabled -> blocked", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: false},
		}, false},
		{"category muted -> blocked", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: true, MutedUntil: ts(future)},
		}, false},
		{"expired mute -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: true, MutedUntil: ts(past)},
		}, true},
		{"global mute -> blocked", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, Enabled: true, MutedUntil: ts(future)},
		}, false},
		{"expired global mute -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, Enabled: true, MutedUntil: ts(past)},
		}, true},
		{"global disabled -> blocked", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, Enabled: false},
		}, false},
		{"global mute beats enabled category", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, Enabled: true, MutedUntil: ts(future)},
			{Category: "expiry", Enabled: true},
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, mq := newService(t)
			mq.EXPECT().GetNotificationTypeCategory(gomock.Any(), KindItemExpiring).Return("expiry", nil)
			mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return(tc.prefs, nil)
			ok, err := svc.Allowed(context.Background(), 1, KindItemExpiring)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)
		})
	}
}

// ---------- preferences ----------

func TestListCategoryPreferences_DefaultsAndOverlay(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
		{Kind: "item_expiring", Category: "expiry"},
		{Kind: "event_created", Category: "events"},
		{Kind: "event_updated", Category: "events"},
	}, nil)
	muted := time.Now().Add(time.Hour)
	mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return([]sqlc.UserprefsNotificationPref{
		{Category: "expiry", Enabled: false, MutedUntil: ts(muted)},
	}, nil)

	got, err := svc.ListCategoryPreferences(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got, 3) // _all + expiry + events (dedup'd)
	assert.Equal(t, CategoryAll, got[0].Category)
	assert.True(t, got[0].Enabled)

	var expiry, events *CategoryPreference
	for i := range got {
		switch got[i].Category {
		case "expiry":
			expiry = &got[i]
		case "events":
			events = &got[i]
		}
	}
	require.NotNil(t, expiry)
	assert.False(t, expiry.Enabled)
	require.NotNil(t, expiry.MutedUntil)
	assert.True(t, expiry.MutedUntil.Equal(muted))
	require.NotNil(t, events)
	assert.True(t, events.Enabled)
	assert.Nil(t, events.MutedUntil)
}

func TestSetCategoryEnabled_UpsertsPreservingMute(t *testing.T) {
	svc, mq := newService(t)
	muted := time.Now().Add(time.Hour)
	gomock.InOrder(
		mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
			{Kind: "item_expiring", Category: "expiry"},
		}, nil),
		mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return([]sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: true, MutedUntil: ts(muted)},
		}, nil),
		mq.EXPECT().UpsertNotificationPref(gomock.Any(), sqlc.UpsertNotificationPrefParams{
			UserID: 1, Category: "expiry", Enabled: false, MutedUntil: ts(muted),
		}).Return(nil),
	)
	require.NoError(t, svc.SetCategoryEnabled(context.Background(), 1, "expiry", false))
}

func TestSetCategoryEnabled_UnknownCategory(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
		{Kind: "item_expiring", Category: "expiry"},
	}, nil)
	err := svc.SetCategoryEnabled(context.Background(), 1, "bogus", false)
	var ve *domainerr.ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestMuteCategory(t *testing.T) {
	svc, mq := newService(t)
	until := time.Now().Add(2 * time.Hour)
	gomock.InOrder(
		// _all needs no registry lookup; checkCategory returns early.
		mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return(nil, nil),
		mq.EXPECT().UpsertNotificationPref(gomock.Any(), gomock.Cond(func(a sqlc.UpsertNotificationPrefParams) bool {
			return a.UserID == 1 && a.Category == CategoryAll && a.Enabled &&
				a.MutedUntil.Valid && a.MutedUntil.Time.Equal(until)
		})).Return(nil),
	)
	require.NoError(t, svc.MuteCategory(context.Background(), 1, CategoryAll, until))
}

func TestMuteCategory_PastTimeRejected(t *testing.T) {
	svc, _ := newService(t)
	// Fails before any query — _all skips the registry lookup and the
	// past-time check runs next.
	err := svc.MuteCategory(context.Background(), 1, CategoryAll, time.Now().Add(-time.Hour))
	var ve *domainerr.ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestClearMute_PreservesEnabled(t *testing.T) {
	svc, mq := newService(t)
	gomock.InOrder(
		mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
			{Kind: "item_expiring", Category: "expiry"},
		}, nil),
		mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return([]sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: false},
		}, nil),
		mq.EXPECT().UpsertNotificationPref(gomock.Any(), sqlc.UpsertNotificationPrefParams{
			UserID: 1, Category: "expiry", Enabled: false,
		}).Return(nil),
	)
	require.NoError(t, svc.ClearMute(context.Background(), 1, "expiry"))
}

// ---------- sweep ----------

func TestSweep_CreatesDedupedReminders(t *testing.T) {
	svc, mq := newService(t)
	now := time.Date(2025, 6, 2, 9, 0, 0, 0, time.Local) // Monday 9am
	mon := time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)

	slot := sqlc.ListMealSlotsWithRecipesRow{
		SlotID: 10, DayOfWeek: 4, // Thursday dinner this week
		Servings:           pgtype.Int4{Int32: 4, Valid: true},
		WeekStartDate:      pgtype.Date{Time: mon, Valid: true},
		WeekStartDayOfWeek: 1,
		HouseholdID:        7,
		RecipeID:           5, RecipeName: "Roast",
		RecipeServings: pgtype.Int4{Int32: 4, Valid: true},
	}
	mq.EXPECT().ListMealSlotsWithRecipes(gomock.Any()).Return([]sqlc.ListMealSlotsWithRecipesRow{slot}, nil)
	// 8 lb of protein -> 2 day lead -> due Thursday 8am - 2d = Tuesday; but
	// the meal is Thursday so no reminder yet at Monday 9am... except the
	// protein item is huge enough to fire: use 20 lb -> 5 day lead -> due
	// Sat; already past -> fires.
	mq.EXPECT().ListProteinItemsForRecipes(gomock.Any(), []int64{5}).Return([]sqlc.ListProteinItemsForRecipesRow{
		{RecipeID: 5, Quantity: num(20), ToBaseFactor: num(453.592), UnitKind: pgtype.Text{String: "weight", Valid: true}},
	}, nil)
	mq.EXPECT().ListLongStepsForRecipes(gomock.Any(), []int64{5}).Return([]sqlc.ListLongStepsForRecipesRow{
		// 3-day step -> 3 day lead -> due Monday 8am; sweep runs 9am -> fires.
		{RecipeID: 5, MaxDurationMinutes: 4320},
	}, nil)
	mq.EXPECT().ListMemberIDsForHouseholds(gomock.Any(), []int64{7}).Return([]sqlc.ListMemberIDsForHouseholdsRow{
		{UserID: 11, HouseholdID: pgtype.Int8{Int64: 7, Valid: true}},
		{UserID: 12, HouseholdID: pgtype.Int8{Int64: 7, Valid: true}},
	}, nil).Times(2) // once for meal reminders, once for expiry
	// Expiring item in window.
	mq.EXPECT().ListExpiringHouseholdItems(gomock.Any(), gomock.Any()).Return([]sqlc.ListExpiringHouseholdItemsRow{
		{HouseholdItemID: 3, HouseholdID: 7, ItemID: 9, ItemName: "Milk",
			ExpiresAt: ts(now.AddDate(0, 0, 2))},
	}, nil)
	// Members allowed for all kinds.
	for _, uid := range []int64{11, 12} {
		for _, kind := range []string{KindProteinDefrost, KindMealPrepAdvance, KindItemExpiring} {
			mq.EXPECT().GetNotificationTypeCategory(gomock.Any(), kind).Return("cat", nil)
			mq.EXPECT().ListNotificationPrefs(gomock.Any(), uid).Return(nil, nil)
		}
	}
	// 2 recipe reminders + 1 item reminder x 2 members = 6 inserts.
	mq.EXPECT().InsertRecipeReminderNotification(gomock.Any(), gomock.Any()).
		Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Times(4)
	mq.EXPECT().InsertItemReminderNotification(gomock.Any(), gomock.Any()).
		Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Times(2)
	mq.EXPECT().PruneReadNotifications(gomock.Any(), gomock.Any()).Return(nil).Times(6)

	n, err := svc.Sweep(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, 6, n)
}

func TestSweep_SuppressedMemberGetsNothing(t *testing.T) {
	svc, mq := newService(t)
	now := time.Date(2025, 6, 2, 9, 0, 0, 0, time.Local)
	mq.EXPECT().ListMealSlotsWithRecipes(gomock.Any()).Return(nil, nil)
	mq.EXPECT().ListExpiringHouseholdItems(gomock.Any(), gomock.Any()).Return([]sqlc.ListExpiringHouseholdItemsRow{
		{HouseholdItemID: 3, HouseholdID: 7, ItemID: 9, ItemName: "Milk",
			ExpiresAt: ts(now.AddDate(0, 0, 1))},
	}, nil)
	mq.EXPECT().ListMemberIDsForHouseholds(gomock.Any(), []int64{7}).Return([]sqlc.ListMemberIDsForHouseholdsRow{
		{UserID: 11, HouseholdID: pgtype.Int8{Int64: 7, Valid: true}},
	}, nil)
	mq.EXPECT().GetNotificationTypeCategory(gomock.Any(), KindItemExpiring).Return("expiry", nil)
	mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(11)).Return([]sqlc.UserprefsNotificationPref{
		{Category: "expiry", Enabled: false},
	}, nil)

	n, err := svc.Sweep(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// ---------- push gate ----------

func TestPushAllowed_UnknownKindFailsOpen(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().GetNotificationTypeCategory(gomock.Any(), "ghost_kind").Return("", pgx.ErrNoRows)
	ok, err := svc.PushAllowed(context.Background(), 1, "ghost_kind")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestPushAllowed_Matrix(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	for _, tc := range []struct {
		name  string
		prefs []sqlc.UserprefsNotificationPref
		want  bool
	}{
		// Push is opt-in: unlike the feed, defaults deny.
		{"no prefs -> denied", nil, false},
		{"unrelated pref -> denied", []sqlc.UserprefsNotificationPref{
			{Category: "events", PushEnabled: true},
		}, false},
		{"category push on -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", PushEnabled: true},
		}, true},
		{"master _all push on -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, PushEnabled: true},
		}, true},
		{"master on, category off -> still allowed", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, PushEnabled: true},
			{Category: "expiry", PushEnabled: false},
		}, true},
		{"category push on but muted -> denied", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", PushEnabled: true, MutedUntil: ts(future)},
		}, false},
		{"expired mute -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", PushEnabled: true, MutedUntil: ts(past)},
		}, true},
		{"_all muted -> denied despite category push", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, MutedUntil: ts(future)},
			{Category: "expiry", PushEnabled: true},
		}, false},
		// Channels are independent: disabling the feed does not kill push.
		{"feed disabled, push on -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: false, PushEnabled: true},
		}, true},
		{"_all feed disabled, category push on -> allowed", []sqlc.UserprefsNotificationPref{
			{Category: CategoryAll, Enabled: false},
			{Category: "expiry", PushEnabled: true},
		}, true},
		{"feed enabled alone -> denied", []sqlc.UserprefsNotificationPref{
			{Category: "expiry", Enabled: true},
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, mq := newService(t)
			mq.EXPECT().GetNotificationTypeCategory(gomock.Any(), KindItemExpiring).Return("expiry", nil)
			mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return(tc.prefs, nil)
			ok, err := svc.PushAllowed(context.Background(), 1, KindItemExpiring)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)
		})
	}
}

func TestSetCategoryPushEnabled(t *testing.T) {
	svc, mq := newService(t)
	gomock.InOrder(
		mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
			{Kind: "item_expiring", Category: "expiry"},
		}, nil),
		mq.EXPECT().UpsertNotificationPrefPushEnabled(gomock.Any(), sqlc.UpsertNotificationPrefPushEnabledParams{
			UserID: 1, Category: "expiry", PushEnabled: true,
		}).Return(nil),
	)
	require.NoError(t, svc.SetCategoryPushEnabled(context.Background(), 1, "expiry", true))
}

func TestSetCategoryPushEnabled_UnknownCategory(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
		{Kind: "item_expiring", Category: "expiry"},
	}, nil)
	err := svc.SetCategoryPushEnabled(context.Background(), 1, "bogus", true)
	var ve *domainerr.ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestListCategoryPreferences_ExposesPushFlag(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().ListActiveNotificationTypes(gomock.Any()).Return([]sqlc.ListActiveNotificationTypesRow{
		{Kind: "item_expiring", Category: "expiry"},
	}, nil)
	mq.EXPECT().ListNotificationPrefs(gomock.Any(), int64(1)).Return([]sqlc.UserprefsNotificationPref{
		{Category: "expiry", Enabled: true, PushEnabled: true},
	}, nil)

	got, err := svc.ListCategoryPreferences(context.Background(), 1)
	require.NoError(t, err)
	for _, p := range got {
		if p.Category == "expiry" {
			assert.True(t, p.PushEnabled)
			return
		}
	}
	t.Fatal("expiry pref missing")
}

// ---------- device tokens ----------

func TestRegisterDeviceToken_Validation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform string
		token    string
		wantErr  bool
	}{
		{"android ok", "android", "fcm-token-abc", false},
		{"ios ok", "ios", "apns-token", false},
		{"web ok", "web", "vapid-token", false},
		{"bad platform", "carrier-pigeon", "tok", true},
		{"empty token", "android", "", true},
		{"oversized token", "android", string(make([]byte, 513)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, mq := newService(t)
			if !tc.wantErr {
				mq.EXPECT().UpsertDeviceToken(gomock.Any(), sqlc.UpsertDeviceTokenParams{
					UserID: 1, Platform: tc.platform, Token: tc.token,
				}).Return(nil)
			}
			err := svc.RegisterDeviceToken(context.Background(), 1, tc.platform, tc.token)
			if tc.wantErr {
				var ve *domainerr.ValidationError
				require.ErrorAs(t, err, &ve)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnregisterDeviceToken_ScopedToCaller(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().DeleteDeviceTokenForUser(gomock.Any(), sqlc.DeleteDeviceTokenForUserParams{
		Token: "tok", UserID: 1,
	}).Return(nil)
	require.NoError(t, svc.UnregisterDeviceToken(context.Background(), 1, "tok"))
}
