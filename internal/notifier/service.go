// Package notifier owns the Notification Manager: the notification-type
// registry (kind -> opt-out category), per-user notification preferences,
// the suppression gate consulted by every notification writer, and the
// hourly sweep that materializes scheduled reminders (protein defrost,
// advance prep, expiring pantry items) as dedup-keyed rows for every
// household member.
package notifier

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JRAdams472/LENA2/internal/notifier/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Scheduled reminder kinds. Event-driven kinds stay declared in the
// household package, which writes them.
const (
	KindProteinDefrost  = "protein_defrost"
	KindMealPrepAdvance = "meal_prep_advance"
	KindItemExpiring    = "item_expiring"
)

// CategoryAll is the pseudo-category implementing the global mute — a
// notification_pref row with this category mutes every kind until the
// timestamp passes.
const CategoryAll = "_all"

// categoryLabels gives each opt-out bucket a display name for the
// preferences UI. Categories themselves live on notification_type rows;
// a new seeded category needs a label entry here to render.
var categoryLabels = map[string]string{
	"household":      "Household activity",
	"events":         "Food events",
	"meal_reminders": "Meal reminders",
	"expiry":         "Expiring pantry items",
	CategoryAll:      "All notifications",
}

// Config tunes the sweep cadence and reminder windows.
type Config struct {
	// SweepInterval is how often the reminder sweep runs.
	SweepInterval time.Duration
	// NotifyHour is the server-local hour at which date-based reminders
	// become due (meal slots are date-granular).
	NotifyHour int
	// ExpiryDays is how far ahead of an item's expires_at the expiry
	// reminder fires.
	ExpiryDays int
}

// CategoryPreference is one opt-out bucket's effective state for a user:
// registry categories plus the _all global mute row.
type CategoryPreference struct {
	Category    string
	Label       string
	Enabled     bool
	PushEnabled bool
	MutedUntil  *time.Time
}

// Service owns notification preferences, the suppression gate, and the
// reminder sweep.
type Service struct {
	q        sqlc.Querier
	cfg      Config
	stop     context.CancelFunc
	wg       sync.WaitGroup
	delivery *DeliveryWorker
}

// NewService builds the notifier. Like the other domain services it joins
// a ctx-carried transaction first (see dbtx.ContextExecer) so admin-test
// sweeps can run inside a UnitOfWork.
func NewService(pool *pgxpool.Pool, cfg Config) *Service {
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = time.Hour
	}
	if cfg.NotifyHour <= 0 || cfg.NotifyHour > 23 {
		cfg.NotifyHour = 8
	}
	if cfg.ExpiryDays <= 0 {
		cfg.ExpiryDays = 3
	}
	return &Service{q: sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "notifier")), cfg: cfg}
}

// ---------- Preferences ----------

// ListCategoryPreferences returns every opt-out bucket (registry categories
// plus _all) with the user's effective enabled/muted state. Missing pref
// rows mean defaults: enabled, not muted.
func (s *Service) ListCategoryPreferences(ctx context.Context, userID int64) ([]CategoryPreference, error) {
	types, err := s.q.ListActiveNotificationTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list notification types: %w", domainerr.FromStorage(err))
	}
	prefs, err := s.q.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification prefs: %w", domainerr.FromStorage(err))
	}
	byCat := make(map[string]sqlc.UserprefsNotificationPref, len(prefs))
	for _, p := range prefs {
		byCat[p.Category] = p
	}
	seen := map[string]bool{}
	out := []CategoryPreference{prefFor(CategoryAll, byCat[CategoryAll])}
	seen[CategoryAll] = true
	for _, t := range types {
		if seen[t.Category] {
			continue
		}
		seen[t.Category] = true
		out = append(out, prefFor(t.Category, byCat[t.Category]))
	}
	return out, nil
}

// prefFor overlays a stored row onto the defaults for one category.
func prefFor(category string, p sqlc.UserprefsNotificationPref) CategoryPreference {
	cp := CategoryPreference{Category: category, Label: categoryLabels[category], Enabled: true}
	if cp.Label == "" {
		cp.Label = category
	}
	if p.Category != "" {
		cp.Enabled = p.Enabled
		cp.PushEnabled = p.PushEnabled
		if p.MutedUntil.Valid {
			t := p.MutedUntil.Time
			cp.MutedUntil = &t
		}
	}
	return cp
}

// SetCategoryEnabled switches one opt-out bucket (or _all) without
// disturbing an active mute window.
func (s *Service) SetCategoryEnabled(ctx context.Context, userID int64, category string, enabled bool) error {
	if err := s.checkCategory(ctx, category); err != nil {
		return err
	}
	muted, err := s.currentMutedUntil(ctx, userID, category)
	if err != nil {
		return err
	}
	return s.upsertPref(ctx, userID, category, enabled, muted)
}

// MuteCategory sets a mute-until on one bucket — or every kind when the
// category is _all.
func (s *Service) MuteCategory(ctx context.Context, userID int64, category string, until time.Time) error {
	if err := s.checkCategory(ctx, category); err != nil {
		return err
	}
	if !until.After(time.Now()) {
		return &domainerr.ValidationError{Msg: "mute-until must be in the future"}
	}
	enabled, err := s.currentEnabled(ctx, userID, category)
	if err != nil {
		return err
	}
	return s.upsertPref(ctx, userID, category, enabled, &until)
}

// ClearMute removes a category (or _all) mute window, preserving enabled.
func (s *Service) ClearMute(ctx context.Context, userID int64, category string) error {
	if err := s.checkCategory(ctx, category); err != nil {
		return err
	}
	enabled, err := s.currentEnabled(ctx, userID, category)
	if err != nil {
		return err
	}
	return s.upsertPref(ctx, userID, category, enabled, nil)
}

// SetCategoryPushEnabled switches push delivery for one bucket (or _all)
// without disturbing enabled or an active mute window. Push is independent
// of the feed: a member can want a category pushed but not in feed.
func (s *Service) SetCategoryPushEnabled(ctx context.Context, userID int64, category string, enabled bool) error {
	if err := s.checkCategory(ctx, category); err != nil {
		return err
	}
	if err := s.q.UpsertNotificationPrefPushEnabled(ctx, sqlc.UpsertNotificationPrefPushEnabledParams{
		UserID: userID, Category: category, PushEnabled: enabled,
	}); err != nil {
		return fmt.Errorf("upsert notification pref push: %w", domainerr.FromStorage(err))
	}
	return nil
}

func (s *Service) upsertPref(ctx context.Context, userID int64, category string, enabled bool, mutedUntil *time.Time) error {
	var mu pgtype.Timestamptz
	if mutedUntil != nil {
		mu = pgtype.Timestamptz{Time: *mutedUntil, Valid: true}
	}
	if err := s.q.UpsertNotificationPref(ctx, sqlc.UpsertNotificationPrefParams{
		UserID: userID, Category: category, Enabled: enabled, MutedUntil: mu,
	}); err != nil {
		return fmt.Errorf("upsert notification pref: %w", domainerr.FromStorage(err))
	}
	return nil
}

func (s *Service) currentMutedUntil(ctx context.Context, userID int64, category string) (*time.Time, error) {
	rows, err := s.q.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification prefs: %w", domainerr.FromStorage(err))
	}
	for _, p := range rows {
		if p.Category == category && p.MutedUntil.Valid {
			t := p.MutedUntil.Time
			return &t, nil
		}
	}
	return nil, nil
}

func (s *Service) currentEnabled(ctx context.Context, userID int64, category string) (bool, error) {
	rows, err := s.q.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("list notification prefs: %w", domainerr.FromStorage(err))
	}
	for _, p := range rows {
		if p.Category == category {
			return p.Enabled, nil
		}
	}
	return true, nil
}

// checkCategory validates a preference target: _all or a category that at
// least one active kind maps to.
func (s *Service) checkCategory(ctx context.Context, category string) error {
	if category == CategoryAll {
		return nil
	}
	types, err := s.q.ListActiveNotificationTypes(ctx)
	if err != nil {
		return fmt.Errorf("list notification types: %w", domainerr.FromStorage(err))
	}
	for _, t := range types {
		if t.Category == category {
			return nil
		}
	}
	return &domainerr.ValidationError{Field: "category", Msg: "unknown notification category"}
}

// ---------- Suppression gate ----------

// Allowed reports whether a notification of the given kind may be created
// for the user right now. household.Service consults this gate before every
// event-driven write, and the sweep consults it per member, so opt-outs
// apply uniformly. Unknown/inactive kinds fail open — a missing seed row
// should not silently eat notifications.
func (s *Service) Allowed(ctx context.Context, userID int64, kind string) (bool, error) {
	category, err := s.q.GetNotificationTypeCategory(ctx, kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return true, fmt.Errorf("notification kind lookup: %w", domainerr.FromStorage(err))
	}
	return s.categoryAllowed(ctx, userID, category)
}

// categoryAllowed applies the suppression rules for one bucket: _all mute
// wins, then per-category opt-out, then per-category mute.
func (s *Service) categoryAllowed(ctx context.Context, userID int64, category string) (bool, error) {
	prefs, err := s.q.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return true, fmt.Errorf("list notification prefs: %w", domainerr.FromStorage(err))
	}
	now := time.Now()
	if prefMuted(prefs, CategoryAll, now) || prefMuted(prefs, category, now) {
		return false, nil
	}
	return true, nil
}

// prefMuted reports whether any pref row in the category suppresses
// delivery — disabled outright or muted until after now.
func prefMuted(prefs []sqlc.UserprefsNotificationPref, category string, now time.Time) bool {
	for _, p := range prefs {
		if p.Category != category {
			continue
		}
		if !p.Enabled || (p.MutedUntil.Valid && p.MutedUntil.Time.After(now)) {
			return true
		}
	}
	return false
}

// PushAllowed reports whether a notification of the given kind may also be
// pushed to the user's devices. Like Allowed it resolves the kind's
// category, but the gates differ: mutes are channel-agnostic (a mute means
// silence), while 'enabled' gates only the feed — push requires an
// explicit push_enabled opt-in on the category or on _all.
func (s *Service) PushAllowed(ctx context.Context, userID int64, kind string) (bool, error) {
	category, err := s.q.GetNotificationTypeCategory(ctx, kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return true, fmt.Errorf("notification kind lookup: %w", domainerr.FromStorage(err))
	}
	return s.categoryPushAllowed(ctx, userID, category)
}

func (s *Service) categoryPushAllowed(ctx context.Context, userID int64, category string) (bool, error) {
	prefs, err := s.q.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return true, fmt.Errorf("list notification prefs: %w", domainerr.FromStorage(err))
	}
	now := time.Now()
	if prefSilenced(prefs, CategoryAll, now) || prefSilenced(prefs, category, now) {
		return false, nil
	}
	return prefPushOn(prefs, CategoryAll) || prefPushOn(prefs, category), nil
}

// prefSilenced reports whether an active mute window suppresses the
// category — unlike prefMuted it ignores 'enabled', which gates the feed
// channel only.
func prefSilenced(prefs []sqlc.UserprefsNotificationPref, category string, now time.Time) bool {
	for _, p := range prefs {
		if p.Category == category && p.MutedUntil.Valid && p.MutedUntil.Time.After(now) {
			return true
		}
	}
	return false
}

func prefPushOn(prefs []sqlc.UserprefsNotificationPref, category string) bool {
	for _, p := range prefs {
		if p.Category == category {
			return p.PushEnabled
		}
	}
	return false
}

// ---------- Device tokens ----------

// RegisterDeviceToken records or refreshes a provider push token for the
// user. Register is idempotent per token and reassigns ownership to the
// caller — the token follows the most recent login on a shared device.
func (s *Service) RegisterDeviceToken(ctx context.Context, userID int64, platform, token string) error {
	switch platform {
	case "android", "ios", "web":
	default:
		return &domainerr.ValidationError{Field: "platform", Msg: "unknown device platform"}
	}
	if token == "" || len(token) > 512 {
		return &domainerr.ValidationError{Field: "token", Msg: "device token must be 1-512 characters"}
	}
	if err := s.q.UpsertDeviceToken(ctx, sqlc.UpsertDeviceTokenParams{
		UserID: userID, Platform: platform, Token: token,
	}); err != nil {
		return fmt.Errorf("register device token: %w", domainerr.FromStorage(err))
	}
	return nil
}

// UnregisterDeviceToken removes one of the caller's own tokens (logout).
// Removing someone else's token is a no-op, not an error — the caller can
// only ever see and revoke their own registrations.
func (s *Service) UnregisterDeviceToken(ctx context.Context, userID int64, token string) error {
	if err := s.q.DeleteDeviceTokenForUser(ctx, sqlc.DeleteDeviceTokenForUserParams{
		Token: token, UserID: userID,
	}); err != nil {
		return fmt.Errorf("unregister device token: %w", domainerr.FromStorage(err))
	}
	return nil
}
