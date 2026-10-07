// Package household owns the household schema: the households themselves
// and the invite lifecycle between users. Shared-data scoping (pantry,
// cellar, meal plans, grocery lists) stays in the domain services; this
// package only manages membership and invitations.
package household

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/household/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Status is the persisted invite lifecycle state on household.invites.
type Status string

// Invite statuses. Only pending invites may transition; concluded invites
// are immutable history.
const (
	StatusPending   Status = "pending"
	StatusAccepted  Status = "accepted"
	StatusDeclined  Status = "declined"
	StatusCancelled Status = "cancelled"
)

// Household is a group of users sharing operational data.
type Household struct {
	HouseholdID int64
	Name        *string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

// NotificationKind identifies the household event a notification records.
type NotificationKind string

// UserDirected reports whether the notification's recipient relationship
// is to the event itself rather than to the household — the target is not
// (or no longer) a member: received/cancelled invites and removal notices.
// These rows are written with a NULL household_id so active-household feed
// scoping (LEN-26) can never hide a notice addressed to the user.
func (k NotificationKind) UserDirected() bool {
	switch k {
	case KindInviteReceived, KindInviteCancelled, KindMemberRemoved:
		return true
	default:
		return false
	}
}

// Notification kinds written inside the transaction that produces them.
const (
	KindInviteReceived   NotificationKind = "invite_received"
	KindInviteAccepted   NotificationKind = "invite_accepted"
	KindInviteDeclined   NotificationKind = "invite_declined"
	KindInviteCancelled  NotificationKind = "invite_cancelled"
	KindMemberJoined     NotificationKind = "member_joined"
	KindMemberLeft       NotificationKind = "member_left"
	KindMemberRemoved    NotificationKind = "member_removed"
	KindRoleChanged      NotificationKind = "role_changed"
	KindHouseholdRenamed NotificationKind = "household_renamed"
	KindEventCreated     NotificationKind = "event_created"
	KindEventUpdated     NotificationKind = "event_updated"
	KindEventDeleted     NotificationKind = "event_deleted"
)

// Notification is an in-app household event surfaced to a user via the
// unread badge and notification feed.
type Notification struct {
	NotificationID int64
	UserID         int64
	HouseholdID    *int64
	Kind           NotificationKind
	ActorUserID    *int64
	InviteID       *int64
	// FoodEventID deep-links event notifications to the event; null for
	// non-event kinds or after the event row is deleted (SET NULL).
	FoodEventID *int64
	// Title/Body carry server-rendered feed text for scheduled reminders;
	// event-driven rows leave them null and clients render from kind.
	Title *string
	Body  *string
	// RecipeID/ItemID deep-link reminder notifications to their subject.
	RecipeID  *int64
	ItemID    *int64
	ReadAt    *time.Time
	CreatedAt time.Time
}

// Membership ties a user to a household with a role (LEN-26). The
// household_member table is the source of truth for membership; the
// user's household_id/household_role columns mirror the ACTIVE
// membership so household-scoped queries keep working.
type Membership struct {
	HouseholdID int64
	UserID      int64
	Role        string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedBy   *string
	UpdatedAt   *time.Time
}

// MyHousehold pairs a household the user belongs to with their role in it.
type MyHousehold struct {
	Household Household
	Role      string
}

// Invite is a pending or concluded household invitation.
type Invite struct {
	InviteID    int64
	FromUserID  int64
	ToUserID    int64
	HouseholdID int64
	Status      Status
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedBy   *string
	UpdatedAt   *time.Time
}

// NotifyGate decides at write time whether a notification of a given kind
// may be delivered to a user — the notifier service implements it so
// per-category opt-outs and mute windows apply to event-driven writes too.
// Feed and push are independent channels with separate opt-ins.
type NotifyGate interface {
	Allowed(ctx context.Context, userID int64, kind string) (bool, error)
	PushAllowed(ctx context.Context, userID int64, kind string) (bool, error)
}

// Service provides household and invite operations backed by Postgres.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// gate suppresses notifications the recipient opted out of; nil means
	// deliver everything (tests, pre-wiring startup paths).
	gate NotifyGate
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// WithNotifyGate installs the write-time suppression gate.
func (s *Service) WithNotifyGate(g NotifyGate) *Service {
	s.gate = g
	return s
}

// NewService creates a household Service using the given connection pool.
// The querier resolves a ctx-carried transaction first (see
// dbtx.ContextExecer) so calls made inside a UnitOfWork join that
// transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "household")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "household"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "household")) }
	}
	c := *s
	c.q = newQ(tx)
	c.tx = tx
	return &c
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. If the service is already bound to a
// transaction, or ctx already carries a UnitOfWork transaction, fn runs in
// that transaction instead of starting a new one.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	if s.tx != nil || dbtx.HasTx(ctx) {
		return fn(s)
	}
	if s.pool == nil {
		return fmt.Errorf("household: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// CreateHousehold inserts a new (typically single-person) household.
func (s *Service) CreateHousehold(ctx context.Context, by string) (Household, error) {
	row, err := s.q.CreateHousehold(ctx, by)
	if err != nil {
		return Household{}, fmt.Errorf("create household: %w", domainerr.FromStorage(err))
	}
	return toHousehold(row), nil
}

// GetHouseholdByID returns a household by primary key.
func (s *Service) GetHouseholdByID(ctx context.Context, householdID int64) (Household, error) {
	row, err := s.q.GetHouseholdByID(ctx, householdID)
	if err != nil {
		return Household{}, fmt.Errorf("get household: %w", domainerr.FromStorage(err))
	}
	return toHousehold(row), nil
}

// CreateInvite records a pending invitation. A duplicate pending invite
// between the same pair surfaces as domainerr.ErrConflict via the partial
// unique index.
func (s *Service) CreateInvite(ctx context.Context, fromUserID, toUserID, householdID int64, by string) (Invite, error) {
	row, err := s.q.CreateInvite(ctx, sqlc.CreateInviteParams{
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		HouseholdID: householdID,
		CreatedBy:   by,
	})
	if err != nil {
		return Invite{}, fmt.Errorf("create invite: %w", domainerr.FromStorage(err))
	}
	return toInvite(row), nil
}

// GetInviteByID returns an invite by primary key.
func (s *Service) GetInviteByID(ctx context.Context, inviteID int64) (Invite, error) {
	row, err := s.q.GetInviteByID(ctx, inviteID)
	if err != nil {
		return Invite{}, fmt.Errorf("get invite: %w", domainerr.FromStorage(err))
	}
	return toInvite(row), nil
}

// ListPendingInvitesForUser returns invites awaiting the user's decision.
func (s *Service) ListPendingInvitesForUser(ctx context.Context, userID int64) ([]Invite, error) {
	rows, err := s.q.ListPendingInvitesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list pending invites: %w", domainerr.FromStorage(err))
	}
	return toInvites(rows), nil
}

// ListSentInvitesForUser returns pending invites the user sent for the
// given household (the caller's active one — sent invites scope to the
// active household; received invites aggregate).
func (s *Service) ListSentInvitesForUser(ctx context.Context, userID, householdID int64) ([]Invite, error) {
	rows, err := s.q.ListSentInvitesForUser(ctx, sqlc.ListSentInvitesForUserParams{
		FromUserID:  userID,
		HouseholdID: householdID,
	})
	if err != nil {
		return nil, fmt.Errorf("list sent invites: %w", domainerr.FromStorage(err))
	}
	return toInvites(rows), nil
}

// TransitionInvite moves a pending invite to a concluded status. The update
// is status-guarded: a non-pending invite (already resolved, or lost a
// concurrent race) yields zero rows and surfaces as domainerr.ErrConflict.
// Callers resolve the invite and check actor rules before transitioning.
func (s *Service) TransitionInvite(ctx context.Context, inviteID int64, to Status, by string) (Invite, error) {
	switch to {
	case StatusAccepted, StatusDeclined, StatusCancelled:
	default:
		return Invite{}, &domainerr.ValidationError{Field: "status", Msg: "invite can only transition to a concluded status"}
	}
	row, err := s.q.TransitionInvite(ctx, sqlc.TransitionInviteParams{
		InviteID:  inviteID,
		Status:    string(to),
		UpdatedBy: pgtype.Text{String: by, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invite{}, fmt.Errorf("transition invite: %w", domainerr.ErrConflict)
		}
		return Invite{}, fmt.Errorf("transition invite: %w", domainerr.FromStorage(err))
	}
	return toInvite(row), nil
}

// RenameHousehold sets the household's display name; empty or whitespace
// clears it back to NULL so clients fall back to a member-derived label.
func (s *Service) RenameHousehold(ctx context.Context, householdID int64, name, by string) (Household, error) {
	trimmed := strings.TrimSpace(name)
	n := pgtype.Text{}
	if trimmed != "" {
		n = pgtype.Text{String: trimmed, Valid: true}
	}
	row, err := s.q.RenameHousehold(ctx, sqlc.RenameHouseholdParams{
		HouseholdID: householdID,
		Name:        n,
		UpdatedBy:   pgtype.Text{String: by, Valid: true},
	})
	if err != nil {
		return Household{}, fmt.Errorf("rename household: %w", domainerr.FromStorage(err))
	}
	return toHousehold(row), nil
}

// LockHousehold fetches a household with SELECT ... FOR UPDATE inside the
// caller's transaction. Membership mutations that must serialize against
// each other — accept, leave, remove — lock the row before checking
// member counts or roles.
func (s *Service) LockHousehold(ctx context.Context, householdID int64) (Household, error) {
	row, err := s.q.GetHouseholdByIDForUpdate(ctx, householdID)
	if err != nil {
		return Household{}, fmt.Errorf("lock household: %w", domainerr.FromStorage(err))
	}
	return toHousehold(row), nil
}

// CreateNotification records a household event for a member and prunes
// their read backlog; call it inside the producing operation's
// transaction so the notification can never outlive a rolled-back change.
func (s *Service) CreateNotification(ctx context.Context, userID int64, kind NotificationKind, householdID *int64, actorUserID *int64, inviteID *int64, foodEventID *int64) error {
	feedOK, pushOK := s.gateDecisions(ctx, userID, kind)
	if !feedOK && !pushOK {
		return nil
	}
	hh := pgtype.Int8{}
	if householdID != nil {
		hh = pgtype.Int8{Int64: *householdID, Valid: true}
	}
	actor := pgtype.Int8{}
	if actorUserID != nil {
		actor = pgtype.Int8{Int64: *actorUserID, Valid: true}
	}
	inv := pgtype.Int8{}
	if inviteID != nil {
		inv = pgtype.Int8{Int64: *inviteID, Valid: true}
	}
	ev := pgtype.Int8{}
	if foodEventID != nil {
		ev = pgtype.Int8{Int64: *foodEventID, Valid: true}
	}
	if feedOK {
		if _, err := s.q.CreateNotification(ctx, sqlc.CreateNotificationParams{
			UserID:      userID,
			HouseholdID: hh,
			Kind:        string(kind),
			ActorUserID: actor,
			InviteID:    inv,
			FoodEventID: ev,
		}); err != nil {
			return fmt.Errorf("create notification: %w", domainerr.FromStorage(err))
		}
		if err := s.q.PruneReadNotifications(ctx, userID); err != nil {
			return err
		}
	}
	if pushOK {
		// The delivery worker claims the row and fans out to the user's
		// device tokens; the outbox row rides this transaction so a
		// rollback can never promise a push for an unmade change.
		if _, err := s.q.InsertPushDelivery(ctx, sqlc.InsertPushDeliveryParams{
			UserID:      userID,
			Kind:        string(kind),
			HouseholdID: hh,
			ActorUserID: actor,
			InviteID:    inv,
			FoodEventID: ev,
		}); err != nil {
			return fmt.Errorf("insert push delivery: %w", domainerr.FromStorage(err))
		}
	}
	return nil
}

// gateDecisions evaluates the feed and push channels independently: a
// feed gate error fails open (deliver — an in-app row can't spam a
// phone), while a push gate error fails closed (don't risk an errant
// push on a prefs outage).
func (s *Service) gateDecisions(ctx context.Context, userID int64, kind NotificationKind) (feedOK, pushOK bool) {
	feedOK = true
	if s.gate == nil {
		return feedOK, false
	}
	if ok, err := s.gate.Allowed(ctx, userID, string(kind)); err != nil {
		slog.Warn("notification suppression check failed, delivering anyway", "user", userID, "kind", kind, "error", err)
	} else {
		feedOK = ok
	}
	if ok, err := s.gate.PushAllowed(ctx, userID, string(kind)); err != nil {
		slog.Warn("push suppression check failed, skipping push", "user", userID, "kind", kind, "error", err)
	} else {
		pushOK = ok
	}
	return feedOK, pushOK
}

// ListNotificationsForUser returns the user's notifications for the
// active household plus user-global (household-less) ones, newest-first.
func (s *Service) ListNotificationsForUser(ctx context.Context, userID, householdID int64, limit int32) ([]Notification, error) {
	rows, err := s.q.ListNotificationsForUser(ctx, sqlc.ListNotificationsForUserParams{
		UserID:      userID,
		HouseholdID: pgtype.Int8{Int64: householdID, Valid: householdID != 0},
		Limit:       limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", domainerr.FromStorage(err))
	}
	out := make([]Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, toNotification(r))
	}
	return out, nil
}

// CountUnreadNotifications backs the nav badge; scoped to the active
// household plus user-global notifications.
func (s *Service) CountUnreadNotifications(ctx context.Context, userID, householdID int64) (int64, error) {
	n, err := s.q.CountUnreadNotifications(ctx, sqlc.CountUnreadNotificationsParams{
		UserID:      userID,
		HouseholdID: pgtype.Int8{Int64: householdID, Valid: householdID != 0},
	})
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", domainerr.FromStorage(err))
	}
	return n, nil
}

// MarkAllNotificationsRead clears the user's unread set for the active
// household plus user-global notifications.
func (s *Service) MarkAllNotificationsRead(ctx context.Context, userID, householdID int64) error {
	if _, err := s.q.MarkAllNotificationsRead(ctx, sqlc.MarkAllNotificationsReadParams{
		UserID:      userID,
		HouseholdID: pgtype.Int8{Int64: householdID, Valid: householdID != 0},
	}); err != nil {
		return fmt.Errorf("mark notifications read: %w", domainerr.FromStorage(err))
	}
	return nil
}

// CancelPendingInvitesFrom cancels pending invites a departing member
// sent for the given household and returns them — callers fan out
// invite_cancelled notifications per recipient.
func (s *Service) CancelPendingInvitesFrom(ctx context.Context, fromUserID, householdID int64, by string) ([]Invite, error) {
	rows, err := s.q.CancelPendingInvitesFrom(ctx, sqlc.CancelPendingInvitesFromParams{
		FromUserID:  fromUserID,
		HouseholdID: householdID,
		UpdatedBy:   pgtype.Text{String: by, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("cancel pending invites: %w", domainerr.FromStorage(err))
	}
	return toInvites(rows), nil
}

// ---------- membership (LEN-26) ----------

// GetMembership returns the user's membership row for a household, or
// domainerr.ErrNotFound when they don't belong.
func (s *Service) GetMembership(ctx context.Context, householdID, userID int64) (Membership, error) {
	row, err := s.q.GetMembership(ctx, sqlc.GetMembershipParams{
		HouseholdID: householdID,
		UserID:      userID,
	})
	if err != nil {
		return Membership{}, fmt.Errorf("get membership: %w", domainerr.FromStorage(err))
	}
	return toMembership(row), nil
}

// ListMembersByHousehold returns every membership row for a household,
// join order (oldest first).
func (s *Service) ListMembersByHousehold(ctx context.Context, householdID int64) ([]Membership, error) {
	rows, err := s.q.ListMembersByHousehold(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", domainerr.FromStorage(err))
	}
	return toMemberships(rows), nil
}

// ListMembershipsByUser returns every household the user belongs to,
// join order.
func (s *Service) ListMembershipsByUser(ctx context.Context, userID int64) ([]Membership, error) {
	rows, err := s.q.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", domainerr.FromStorage(err))
	}
	return toMemberships(rows), nil
}

// ListMyHouseholds returns the user's households with their role in
// each — backs the active-household switcher.
func (s *Service) ListMyHouseholds(ctx context.Context, userID int64) ([]MyHousehold, error) {
	rows, err := s.q.ListHouseholdsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list my households: %w", domainerr.FromStorage(err))
	}
	out := make([]MyHousehold, 0, len(rows))
	for _, r := range rows {
		out = append(out, MyHousehold{
			Household: toHousehold(sqlc.HouseholdHousehold{
				HouseholdID: r.HouseholdID,
				Name:        r.Name,
				CreatedBy:   r.CreatedBy,
				CreatedAt:   r.CreatedAt,
				UpdatedBy:   r.UpdatedBy,
				UpdatedAt:   r.UpdatedAt,
			}),
			Role: r.MemberRole,
		})
	}
	return out, nil
}

// CountMembers returns the household's member count — drives the cap and
// last-owner checks.
func (s *Service) CountMembers(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountMembers(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("count members: %w", domainerr.FromStorage(err))
	}
	return n, nil
}

// JoinHousehold writes (or refreshes) a membership row. Callers sync the
// user's active-household pointer via the identity service inside the
// same ambient transaction — this method deliberately does not touch
// identity.users.
func (s *Service) JoinHousehold(ctx context.Context, householdID, userID int64, role, by string) (Membership, error) {
	row, err := s.q.UpsertMembership(ctx, sqlc.UpsertMembershipParams{
		HouseholdID: householdID,
		UserID:      userID,
		Role:        role,
		CreatedBy:   by,
	})
	if err != nil {
		return Membership{}, fmt.Errorf("join household: %w", domainerr.FromStorage(err))
	}
	return toMembership(row), nil
}

// RemoveMembership deletes a membership row; zero rows means the user
// was not a member and surfaces as domainerr.ErrNotFound.
func (s *Service) RemoveMembership(ctx context.Context, householdID, userID int64) error {
	n, err := s.q.DeleteMembership(ctx, sqlc.DeleteMembershipParams{
		HouseholdID: householdID,
		UserID:      userID,
	})
	if err != nil {
		return fmt.Errorf("remove membership: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("remove membership: %w", domainerr.ErrNotFound)
	}
	return nil
}

// DeleteHousehold removes a household row (merge-dissolve). Callers must
// reassign RESTRICT-FK data and move member pointers first; membership and
// other references cascade. Zero rows surfaces as domainerr.ErrNotFound.
func (s *Service) DeleteHousehold(ctx context.Context, householdID int64) error {
	n, err := s.q.DeleteHousehold(ctx, householdID)
	if err != nil {
		return fmt.Errorf("delete household: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("delete household: %w", domainerr.ErrNotFound)
	}
	return nil
}

func toMembership(row sqlc.HouseholdHouseholdMember) Membership {
	m := Membership{
		HouseholdID: row.HouseholdID,
		UserID:      row.UserID,
		Role:        row.Role,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
	}
	if row.UpdatedBy.Valid {
		s := row.UpdatedBy.String
		m.UpdatedBy = &s
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		m.UpdatedAt = &t
	}
	return m
}

func toMemberships(rows []sqlc.HouseholdHouseholdMember) []Membership {
	out := make([]Membership, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMembership(r))
	}
	return out
}

func toNotification(row sqlc.HouseholdNotification) Notification {
	n := Notification{
		NotificationID: row.NotificationID,
		UserID:         row.UserID,
		Kind:           NotificationKind(row.Kind),
		CreatedAt:      row.CreatedAt,
	}
	if row.HouseholdID.Valid {
		id := row.HouseholdID.Int64
		n.HouseholdID = &id
	}
	if row.ActorUserID.Valid {
		id := row.ActorUserID.Int64
		n.ActorUserID = &id
	}
	if row.InviteID.Valid {
		id := row.InviteID.Int64
		n.InviteID = &id
	}
	if row.FoodEventID.Valid {
		id := row.FoodEventID.Int64
		n.FoodEventID = &id
	}
	if row.Title.Valid {
		n.Title = &row.Title.String
	}
	if row.Body.Valid {
		n.Body = &row.Body.String
	}
	if row.RecipeID.Valid {
		id := row.RecipeID.Int64
		n.RecipeID = &id
	}
	if row.ItemID.Valid {
		id := row.ItemID.Int64
		n.ItemID = &id
	}
	if row.ReadAt.Valid {
		t := row.ReadAt.Time
		n.ReadAt = &t
	}
	return n
}

func toHousehold(row sqlc.HouseholdHousehold) Household {
	h := Household{
		HouseholdID: row.HouseholdID,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
	}
	if row.Name.Valid {
		s := row.Name.String
		h.Name = &s
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		h.UpdatedAt = &t
	}
	return h
}

func toInvite(row sqlc.HouseholdInvite) Invite {
	i := Invite{
		InviteID:    row.InviteID,
		FromUserID:  row.FromUserID,
		ToUserID:    row.ToUserID,
		HouseholdID: row.HouseholdID,
		Status:      Status(row.Status),
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
	}
	if row.UpdatedBy.Valid {
		s := row.UpdatedBy.String
		i.UpdatedBy = &s
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		i.UpdatedAt = &t
	}
	return i
}

func toInvites(rows []sqlc.HouseholdInvite) []Invite {
	out := make([]Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, toInvite(r))
	}
	return out
}
