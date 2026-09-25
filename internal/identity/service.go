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
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Service provides identity operations backed by Postgres.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
	// protected holds lowercased emails that can never be demoted from
	// admin or deactivated (LENA_PROTECTED_EMAILS).
	protected map[string]bool
}

// NewService creates an identity Service using the given connection pool.
// The querier resolves a ctx-carried transaction first (see
// dbtx.ContextExecer) so calls made inside a UnitOfWork join that
// transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "identity")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "identity"))
		},
	}
}

// WithProtectedEmails returns a copy of the service treating the given
// "issuer:email" entries (case-insensitive) as protected admins: their
// role and active flag can never be reduced by AdminSetRole/AdminSetActive.
// The last colon separates the issuer from the email. An unqualified bare
// email is stored unscoped and is only checked when no provider is given
// (mainly for test doubles).
func (s *Service) WithProtectedEmails(emails []string) *Service {
	c := *s
	c.protected = make(map[string]bool, len(emails))
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		issuer, email := splitIssuerAndEmail(e)
		key := strings.ToLower(issuer + ":" + email)
		c.protected[key] = true
	}
	return &c
}

// splitIssuerAndEmail splits an "issuer:email" string at the last colon.
// If no colon is present, both the issuer and the unmodified string are
// returned with an empty issuer.
func splitIssuerAndEmail(s string) (issuer, email string) {
	if i := strings.LastIndex(s, ":"); i > 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return "", s
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "identity")) }
	}
	c := *s
	c.q = newQ(tx)
	c.tx = tx
	c.newQ = newQ
	return &c
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise. If the service is already bound to a transaction, or
// ctx already carries a UnitOfWork transaction, fn runs in that transaction
// instead of starting a new one.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	if s.tx != nil || dbtx.HasTx(ctx) {
		return fn(s)
	}
	if s.pool == nil {
		return fmt.Errorf("identity: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// RoleMember and RoleAdmin are the persisted authorization roles on
// identity.users. Membership is seeded via the LENA_ADMIN_EMAILS config
// list: on each authenticated request the BFF promotes a matching user.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// HouseholdRole* are the per-household membership roles persisted on
// identity.users.household_role. They are unrelated to the site-wide
// Role above despite sharing the "admin"/"member" vocabulary.
const (
	HouseholdRoleOwner  = "owner"
	HouseholdRoleAdmin  = "admin"
	HouseholdRoleMember = "member"
)

// Guard errors returned by the admin mutation methods; the BFF maps these
// to forbidden responses.
var (
	ErrSelfModification = errors.New("cannot change your own role or active status")
	ErrProtectedUser    = errors.New("user is a protected admin")
	ErrLastAdmin        = domainerr.ErrLastAdmin
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
	// HouseholdID is nil for users created before the household feature
	// backfills or before the authenticator ensures a default household.
	HouseholdID *int64
	// HouseholdRole is the user's role within their household
	// (owner/admin/member); empty when HouseholdID is nil.
	HouseholdRole string
	IsSearchable  bool
	CreatedAt     time.Time
}

// IsAdmin reports whether the user holds the admin role.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// IsProtected reports whether the user is in the issuer-scoped protected
// list: protected admins can never be demoted or deactivated.
func (s *Service) IsProtected(provider, email string) bool {
	if provider == "" {
		return s.protected[strings.ToLower(":"+email)]
	}
	return s.protected[strings.ToLower(provider+":"+email)]
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
		return User{}, fmt.Errorf("get user by id: %w", domainerr.FromStorage(err))
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

// SetUserHousehold moves a user to householdID, guarded by the expected
// current value (nil for "no household yet"). A stale expectation — a
// concurrent accept or leave won the race — yields zero rows and surfaces
// as domainerr.ErrConflict; a missing user surfaces as ErrConflict too,
// since callers pre-fetch via GetByID when they need to distinguish.
func (s *Service) SetUserHousehold(ctx context.Context, userID, householdID int64, role string, expected *int64) error {
	exp := pgtype.Int8{}
	if expected != nil {
		exp = pgtype.Int8{Int64: *expected, Valid: true}
	}
	n, err := s.q.SetUserHousehold(ctx, sqlc.SetUserHouseholdParams{
		UserID:              userID,
		HouseholdID:         pgtype.Int8{Int64: householdID, Valid: true},
		HouseholdRole:       role,
		ExpectedHouseholdID: exp,
	})
	if err != nil {
		return fmt.Errorf("set user household: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set user household: %w", domainerr.ErrConflict)
	}
	return nil
}

// SetUserHouseholdRole changes a member's role within their current
// household. The household_id predicate makes the update conditional on
// the user still belonging to that household — a concurrent leave/remove
// yields zero rows and surfaces as domainerr.ErrConflict.
func (s *Service) SetUserHouseholdRole(ctx context.Context, userID, householdID int64, role, by string) error {
	n, err := s.q.SetUserHouseholdRole(ctx, sqlc.SetUserHouseholdRoleParams{
		UserID:        userID,
		HouseholdID:   pgtype.Int8{Int64: householdID, Valid: true},
		HouseholdRole: role,
		By:            textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set user household role: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set user household role: %w", domainerr.ErrConflict)
	}
	return nil
}

// CountUsersByHousehold returns the household's member count; the
// accept-path cap check runs it under a household row lock.
func (s *Service) CountUsersByHousehold(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountUsersByHousehold(ctx, pgtype.Int8{Int64: householdID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("count users by household: %w", domainerr.FromStorage(err))
	}
	return n, nil
}

// SetUserSearchable toggles whether the user appears in household-invite
// search results.
func (s *Service) SetUserSearchable(ctx context.Context, userID int64, searchable bool, by string) error {
	n, err := s.q.SetUserSearchable(ctx, sqlc.SetUserSearchableParams{
		UserID:       userID,
		IsSearchable: searchable,
		UpdatedBy:    textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set user searchable: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set user searchable: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ListUsersByHousehold returns the members of a household.
func (s *Service) ListUsersByHousehold(ctx context.Context, householdID int64) ([]User, error) {
	rows, err := s.q.ListUsersByHousehold(ctx, pgtype.Int8{Int64: householdID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list users by household: %w", domainerr.FromStorage(err))
	}
	return toUsers(rows), nil
}

// ListUsersByIDs batch-fetches users by primary key for invite/member
// hydration; ordering is not defined.
func (s *Service) ListUsersByIDs(ctx context.Context, ids []int64) ([]User, error) {
	if len(ids) == 0 {
		return []User{}, nil
	}
	rows, err := s.q.ListUsersByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list users by ids: %w", domainerr.FromStorage(err))
	}
	return toUsers(rows), nil
}

// SearchUsers finds household-invite candidates matching a name or email
// term: active, opted-in users excluding the caller and the caller's
// household members (pass excludeHouseholdID 0 when the caller has none).
func (s *Service) SearchUsers(ctx context.Context, term string, excludeUserID, excludeHouseholdID int64, limit int32) ([]User, error) {
	exclHH := pgtype.Int8{}
	if excludeHouseholdID > 0 {
		exclHH = pgtype.Int8{Int64: excludeHouseholdID, Valid: true}
	}
	rows, err := s.q.SearchUsers(ctx, sqlc.SearchUsersParams{
		ExcludeUserID:      excludeUserID,
		ExcludeHouseholdID: exclHH,
		Pattern:            pgtype.Text{String: "%" + escapeLike(term) + "%", Valid: true},
		RowLimit:           limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search users: %w", domainerr.FromStorage(err))
	}
	return toUsers(rows), nil
}

// escapeLike escapes LIKE wildcards in a user-supplied search term.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func toUsers(rows []sqlc.IdentityUser) []User {
	users := make([]User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toUser(row))
	}
	return users
}

// checkAdminMutation enforces the invariants for privileged changes: an
// admin may not modify their own record and protected admins are immutable.
// The last-active-admin guard is performed by the conditional update itself.
func (s *Service) checkAdminMutation(actorID int64, target User) error {
	if target.UserID == actorID {
		return ErrSelfModification
	}
	if s.IsProtected(target.Provider, target.Email) {
		return ErrProtectedUser
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
	if err := s.checkAdminMutation(actorID, target); err != nil {
		return err
	}
	n, err := s.q.ConditionalSetUserRole(ctx, sqlc.ConditionalSetUserRoleParams{UserID: targetID, Role: role})
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("set user role: %w", domainerr.ErrLastAdmin)
	}
	return nil
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
	if err := s.checkAdminMutation(actorID, target); err != nil {
		return err
	}
	n, err := s.q.ConditionalSetUserActive(ctx, sqlc.ConditionalSetUserActiveParams{
		UserID:    targetID,
		IsActive:  active,
		UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set user active: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("set user active: %w", domainerr.ErrLastAdmin)
	}
	return nil
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
		HouseholdRole:   row.HouseholdRole,
		IsSearchable:    row.IsSearchable,
		CreatedAt:       row.CreatedAt,
	}
	if row.HouseholdID.Valid {
		id := row.HouseholdID.Int64
		u.HouseholdID = &id
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
