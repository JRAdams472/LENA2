// Package identity owns the identity.users table: upserting users on
// authenticated requests and looking them up by ID or by provider identity.
package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/identity/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
)

// Service provides identity operations backed by Postgres.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	// protected holds lowercased emails that can never be demoted from
	// admin or deactivated (LENA_PROTECTED_EMAILS).
	protected map[string]bool
}

// NewService creates an identity Service using the given connection pool.
func NewService(pool dbtx.Pool) *Service {
	return &Service{q: sqlc.New(dbtx.NewTimedExecer(pool, "identity")), pool: pool}
}

// WithProtectedEmails returns a copy of the service treating the given
// emails (case-insensitive) as protected admins: their role and active
// flag can never be reduced by AdminSetRole/AdminSetActive.
func (s *Service) WithProtectedEmails(emails []string) *Service {
	c := *s
	c.protected = make(map[string]bool, len(emails))
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			c.protected[e] = true
		}
	}
	return &c
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	c := *s
	c.q = sqlc.New(dbtx.NewTimedExecer(tx, "identity"))
	return &c
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// RoleMember and RoleAdmin are the persisted authorization roles on
// identity.users. Membership is seeded via the LENA_ADMIN_EMAILS config
// list: on each authenticated request the BFF promotes a matching user.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// Guard errors returned by the admin mutation methods; the BFF maps these
// to forbidden responses.
var (
	ErrSelfModification = errors.New("cannot change your own role or active status")
	ErrProtectedUser    = errors.New("user is a protected admin")
	ErrLastAdmin        = errors.New("cannot remove the last active admin")
)

// User is the identity module's view of a user row.
type User struct {
	UserID          int64
	Provider        string
	ExternalSubject string
	Email           string
	DisplayName     string
	FirstName       string
	LastName        string
	BackupEmail     string
	IsActive        bool
	Role            string
	LastLoginAt     *time.Time
	CreatedAt       time.Time
}

// IsAdmin reports whether the user holds the admin role.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// IsProtected reports whether the user's email is in the protected list:
// protected admins can never be demoted or deactivated.
func (s *Service) IsProtected(email string) bool {
	return s.protected[strings.ToLower(email)]
}

// UpsertUser creates the user on first sign-in or refreshes email/display
// name/last-login on subsequent sign-ins, keyed by (provider, subject).
func (s *Service) UpsertUser(ctx context.Context, provider, subject, email, displayName string) (User, error) {
	row, err := s.q.UpsertUser(ctx, sqlc.UpsertUserParams{
		Provider:        provider,
		ExternalSubject: subject,
		Email:           email,
		DisplayName:     textOrNull(displayName),
		CreatedBy:       email,
		UpdatedBy:       textOrNull(email),
	})
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return toUser(row), nil
}

// SetUserRole updates a user's persisted role.
func (s *Service) SetUserRole(ctx context.Context, userID int64, role string) error {
	return s.q.SetUserRole(ctx, sqlc.SetUserRoleParams{UserID: userID, Role: role})
}

// GetByID looks up a user by their primary key.
func (s *Service) GetByID(ctx context.Context, userID int64) (User, error) {
	row, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return toUser(row), nil
}

// ListUsers returns a page of users ordered by creation date.
func (s *Service) ListUsers(ctx context.Context, limit, offset int32) ([]User, error) {
	rows, err := s.q.ListUsers(ctx, sqlc.ListUsersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := make([]User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toUser(row))
	}
	return users, nil
}

// CountUsers returns the total number of user rows.
func (s *Service) CountUsers(ctx context.Context) (int64, error) {
	n, err := s.q.CountUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

// checkAdminMutation enforces the invariants for privileged changes: an
// admin may not modify their own record, protected admins are immutable,
// and the last active admin cannot be removed.
func (s *Service) checkAdminMutation(ctx context.Context, actorID int64, target User, removesAdmin bool) error {
	if target.UserID == actorID {
		return ErrSelfModification
	}
	if s.IsProtected(target.Email) {
		return ErrProtectedUser
	}
	if removesAdmin {
		n, err := s.q.CountActiveAdmins(ctx)
		if err != nil {
			return fmt.Errorf("count active admins: %w", err)
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	return nil
}

// AdminSetRole changes target's role on behalf of actorID, enforcing the
// self/protected/last-admin guards. Callers must already be admins.
func (s *Service) AdminSetRole(ctx context.Context, actorID, targetID int64, role string) error {
	if targetID == actorID {
		return ErrSelfModification
	}
	target, err := s.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if err := s.checkAdminMutation(ctx, actorID, target, target.IsAdmin() && role != RoleAdmin); err != nil {
		return err
	}
	return s.q.SetUserRole(ctx, sqlc.SetUserRoleParams{UserID: targetID, Role: role})
}

// AdminSetActive activates or deactivates (bans) target on behalf of
// actorID, enforcing the self/protected/last-admin guards.
func (s *Service) AdminSetActive(ctx context.Context, actorID, targetID int64, active bool, by string) error {
	if targetID == actorID {
		return ErrSelfModification
	}
	target, err := s.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if err := s.checkAdminMutation(ctx, actorID, target, target.IsAdmin() && !active); err != nil {
		return err
	}
	return s.q.SetUserActive(ctx, sqlc.SetUserActiveParams{
		UserID:    targetID,
		IsActive:  active,
		UpdatedBy: textOrNull(by),
	})
}

// UpdateProfile stores the user's editable profile fields. Names are
// already validated for length by the caller; backupEmail may be empty to
// clear the field.
func (s *Service) UpdateProfile(ctx context.Context, userID int64, firstName, lastName, backupEmail, by string) error {
	return s.q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
		UserID:      userID,
		FirstName:   textOrNull(firstName),
		LastName:    textOrNull(lastName),
		BackupEmail: textOrNull(backupEmail),
		UpdatedBy:   textOrNull(by),
	})
}

func toUser(row sqlc.IdentityUser) User {
	u := User{
		UserID:          row.UserID,
		Provider:        row.Provider,
		ExternalSubject: row.ExternalSubject,
		Email:           row.Email,
		DisplayName:     row.DisplayName.String,
		FirstName:       row.FirstName.String,
		LastName:        row.LastName.String,
		BackupEmail:     row.BackupEmail.String,
		IsActive:        row.IsActive,
		Role:            row.Role,
		CreatedAt:       row.CreatedAt,
	}
	if row.LastLoginAt.Valid {
		t := row.LastLoginAt.Time
		u.LastLoginAt = &t
	}
	return u
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
