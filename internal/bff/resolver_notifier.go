package bff

import (
	"context"
	"math"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/notifier"
)

// MyNotificationPreferences returns every opt-out bucket (registry
// categories plus the _all global mute) with the caller's effective state.
func (r *Resolver) MyNotificationPreferences(ctx context.Context) ([]*notificationPrefResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	prefs, err := r.NotifierService.ListCategoryPreferences(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]*notificationPrefResolver, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, &notificationPrefResolver{p: p})
	}
	return out, nil
}

// SetNotificationCategoryEnabled turns one opt-out bucket on or off. "_all"
// is rejected here — global muting is a time-bounded action by definition.
func (r *Resolver) SetNotificationCategoryEnabled(ctx context.Context, args struct {
	Category string
	Enabled  bool
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	if args.Category == notifier.CategoryAll {
		return false, badInputf("use muteNotifications to pause all notifications")
	}
	if err := r.NotifierService.SetCategoryEnabled(ctx, u.UserID, args.Category, args.Enabled); err != nil {
		return false, err
	}
	return true, nil
}

// MuteNotifications mutes one category — or every kind when the category is
// null or "_all" — until the given time.
func (r *Resolver) MuteNotifications(ctx context.Context, args struct {
	Category *string
	Until    graphql.Time
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	category := notifier.CategoryAll
	if args.Category != nil && *args.Category != "" {
		category = *args.Category
	}
	if err := r.NotifierService.MuteCategory(ctx, u.UserID, category, args.Until.Time); err != nil {
		return false, err
	}
	return true, nil
}

// ClearNotificationMute lifts a category (or global) mute early.
func (r *Resolver) ClearNotificationMute(ctx context.Context, args struct {
	Category *string
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	category := notifier.CategoryAll
	if args.Category != nil && *args.Category != "" {
		category = *args.Category
	}
	if err := r.NotifierService.ClearMute(ctx, u.UserID, category); err != nil {
		return false, err
	}
	return true, nil
}

// AddItemToCurrentGroceryList drops an expiring pantry item onto the
// household's most recent grocery list so a replacement gets bought. The
// row is source 'manual' so regenerating the list keeps it.
func (r *Resolver) AddItemToCurrentGroceryList(ctx context.Context, args struct {
	ItemID graphql.ID
}) (*groceryListItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	lists, err := r.GroceryService.ListGroceryLists(ctx, u.HouseholdID, 1, 0)
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, badInputf("no grocery list yet — generate one from the meal plan first")
	}
	it, err := r.GroceryService.AddGroceryListItem(ctx, grocery.GroceryListItem{
		GroceryListID:  lists[0].GroceryListID,
		ItemID:         &itemID,
		QuantityNeeded: 1,
		Source:         "manual",
	}, u.HouseholdID, u.Email)
	if err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: it}, nil
}

// TriggerNotificationSweep runs the reminder sweep immediately so admins
// (and E2E tests) can verify reminders without waiting for the hourly tick.
func (r *Resolver) TriggerNotificationSweep(ctx context.Context) (int32, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return 0, err
	}
	n, err := r.NotifierService.Sweep(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	//nolint:gosec // n is saturated at MaxInt32 immediately above.
	return int32(min(n, math.MaxInt32)), nil
}

type notificationPrefResolver struct {
	p notifier.CategoryPreference
}

func (r *notificationPrefResolver) Category() string { return r.p.Category }
func (r *notificationPrefResolver) Label() string    { return r.p.Label }
func (r *notificationPrefResolver) Enabled() bool    { return r.p.Enabled }

func (r *notificationPrefResolver) MutedUntil() *graphql.Time {
	if r.p.MutedUntil == nil {
		return nil
	}
	return timeToGraphQL(r.p.MutedUntil)
}
