// Package household owns the household schema: the households themselves
// and the invite lifecycle between users. Shared-data scoping (pantry,
// cellar, meal plans, grocery lists) stays in the domain services; this
// package only manages membership and invitations.
package household

import (
	"context"
	"errors"
	"fmt"
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
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
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

func toHousehold(row sqlc.HouseholdHousehold) Household {
	h := Household{
		HouseholdID: row.HouseholdID,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
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
