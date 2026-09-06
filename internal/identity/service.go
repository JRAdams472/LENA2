// Package identity owns the identity.users table: upserting users on
// authenticated requests and looking them up by ID or by provider identity.
package identity

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JRAdams472/LENA2/internal/identity/sqlc"
)

// Service provides identity operations backed by Postgres.
type Service struct {
	q sqlc.Querier
}

// NewService creates an identity Service using the given connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

// RoleMember and RoleAdmin are the persisted authorization roles on
// identity.users. Membership is seeded via the LENA_ADMIN_EMAILS config
// list: on each authenticated request the BFF promotes a matching user.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// User is the identity module's view of a user row.
type User struct {
	UserID          int64
	Provider        string
	ExternalSubject string
	Email           string
	DisplayName     string
	IsActive        bool
	Role            string
}

// IsAdmin reports whether the user holds the admin role.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

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

func toUser(row sqlc.IdentityUser) User {
	return User{
		UserID:          row.UserID,
		Provider:        row.Provider,
		ExternalSubject: row.ExternalSubject,
		Email:           row.Email,
		DisplayName:     row.DisplayName.String,
		IsActive:        row.IsActive,
		Role:            row.Role,
	}
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
