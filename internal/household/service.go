// Package household owns the household schema: the households themselves
// and the invite lifecycle between users. Shared-data scoping (pantry,
// cellar, meal plans, grocery lists) stays in the domain services; this
// package only manages membership and invitations.
package household

import (
	"context"
	"errors"
	"fmt"
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
	ReadAt      *time.Time
	CreatedAt   time.Time
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

// Service provides household and invite operations backed by Postgres.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
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

// ListSentInvitesForUser returns pending invites the user sent.
func (s *Service) ListSentInvitesForUser(ctx context.Context, userID int64) ([]Invite, error) {
	rows, err := s.q.ListSentInvitesForUser(ctx, userID)
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
	return s.q.PruneReadNotifications(ctx, userID)
}

// ListNotificationsForUser returns the user's notifications newest-first.
func (s *Service) ListNotificationsForUser(ctx context.Context, userID int64, limit int32) ([]Notification, error) {
	rows, err := s.q.ListNotificationsForUser(ctx, sqlc.ListNotificationsForUserParams{
		UserID: userID,
		Limit:  limit,
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

// CountUnreadNotifications backs the nav badge.
func (s *Service) CountUnreadNotifications(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountUnreadNotifications(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", domainerr.FromStorage(err))
	}
	return n, nil
}

// MarkAllNotificationsRead clears the user's unread set.
func (s *Service) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	if _, err := s.q.MarkAllNotificationsRead(ctx, userID); err != nil {
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
