package bff

import (
	"context"
	"errors"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

// fakeIdentityService is an in-memory IdentityService for resolver tests.
type fakeIdentityService struct {
	users      map[int64]identity.User
	protected  map[string]bool
	roleErr    error
	activeErr  error
	profileErr error
}

func newFakeIdentityService(users ...identity.User) *fakeIdentityService {
	f := &fakeIdentityService{users: make(map[int64]identity.User), protected: map[string]bool{}}
	for _, u := range users {
		f.users[u.UserID] = u
	}
	return f
}

func (f *fakeIdentityService) GetByID(_ context.Context, userID int64) (identity.User, error) {
	u, ok := f.users[userID]
	if !ok {
		return identity.User{}, errors.New("not found")
	}
	return u, nil
}

func (f *fakeIdentityService) ListUsers(_ context.Context, limit, offset int32) ([]identity.User, error) {
	out := make([]identity.User, 0, len(f.users))
	for i := int64(1); i <= int64(len(f.users)); i++ {
		if u, ok := f.users[i]; ok {
			out = append(out, u)
		}
	}
	if int(offset) >= len(out) {
		return nil, nil
	}
	out = out[offset:]
	if int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeIdentityService) CountUsers(context.Context) (int64, error) {
	return int64(len(f.users)), nil
}

func (f *fakeIdentityService) IsProtected(email string) bool {
	return f.protected[email]
}

func (f *fakeIdentityService) AdminSetRole(_ context.Context, _, targetID int64, role string) error {
	if f.roleErr != nil {
		return f.roleErr
	}
	u := f.users[targetID]
	u.Role = role
	f.users[targetID] = u
	return nil
}

func (f *fakeIdentityService) AdminSetActive(_ context.Context, _, targetID int64, active bool, _ string) error {
	if f.activeErr != nil {
		return f.activeErr
	}
	u := f.users[targetID]
	u.IsActive = active
	f.users[targetID] = u
	return nil
}

func (f *fakeIdentityService) UpdateProfile(_ context.Context, userID int64, first, last, backup, _ string) error {
	if f.profileErr != nil {
		return f.profileErr
	}
	u := f.users[userID]
	u.FirstName, u.LastName, u.BackupEmail = first, last, backup
	f.users[userID] = u
	return nil
}

func adminCtx() context.Context {
	return currentuser.WithUser(context.Background(), currentuser.User{
		UserID: 1, Email: "admin@example.com", IsAdmin: true,
	})
}

func TestResolver_Users_AdminOnly(t *testing.T) {
	r := &Resolver{IdentityService: newFakeIdentityService(
		identity.User{UserID: 1, Email: "admin@example.com", Role: identity.RoleAdmin, IsActive: true},
	)}

	// Unauthenticated.
	_, err := r.Users(context.Background(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 25})
	assert.EqualError(t, err, "unauthorized")

	// Non-admin member.
	_, err = r.Users(testenv.WithUser(context.Background(), 2, "m@example.com"), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 25})
	assert.EqualError(t, err, "forbidden: admin role required")

	// Admin.
	page, err := r.Users(adminCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 25})
	require.NoError(t, err)
	require.Len(t, page.Items(), 1)
	assert.Equal(t, int32(1), page.PageInfo().TotalCount())
}

func TestResolver_SetUserRole_Guards(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "admin@example.com", Role: identity.RoleAdmin, IsActive: true},
		identity.User{UserID: 2, Email: "member@example.com", Role: identity.RoleMember, IsActive: true},
	)
	r := &Resolver{IdentityService: svc}
	args := func(id string, role string) struct {
		UserID graphql.ID
		Role   string
	} {
		return struct {
			UserID graphql.ID
			Role   string
		}{UserID: graphql.ID(id), Role: role}
	}

	// Non-admin is forbidden.
	_, err := r.SetUserRole(testenv.WithUser(context.Background(), 3, "x@x.com"), args("2", "admin"))
	assert.EqualError(t, err, "forbidden: admin role required")

	// Invalid role is a client error.
	_, err = r.SetUserRole(adminCtx(), args("2", "superuser"))
	assert.ErrorContains(t, err, "invalid role")

	// Guard errors map to FORBIDDEN.
	svc.roleErr = identity.ErrProtectedUser
	_, err = r.SetUserRole(adminCtx(), args("2", "admin"))
	assert.ErrorContains(t, err, "protected admin")

	svc.roleErr = identity.ErrSelfModification
	_, err = r.SetUserRole(adminCtx(), args("1", "member"))
	assert.ErrorContains(t, err, "your own role")

	svc.roleErr = identity.ErrLastAdmin
	_, err = r.SetUserRole(adminCtx(), args("2", "admin"))
	assert.ErrorContains(t, err, "last active admin")

	// Happy path promotes and returns the updated user.
	svc.roleErr = nil
	got, err := r.SetUserRole(adminCtx(), args("2", "admin"))
	require.NoError(t, err)
	assert.Equal(t, "admin", got.Role())
	assert.Equal(t, "member@example.com", got.Email())
}

func TestResolver_SetUserActive(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "admin@example.com", Role: identity.RoleAdmin, IsActive: true},
		identity.User{UserID: 2, Email: "member@example.com", Role: identity.RoleMember, IsActive: true},
	)
	r := &Resolver{IdentityService: svc}
	args := func(id string, active bool) struct {
		UserID   graphql.ID
		IsActive bool
	} {
		return struct {
			UserID   graphql.ID
			IsActive bool
		}{UserID: graphql.ID(id), IsActive: active}
	}

	got, err := r.SetUserActive(adminCtx(), args("2", false))
	require.NoError(t, err)
	assert.False(t, got.IsActive())

	svc.activeErr = identity.ErrProtectedUser
	_, err = r.SetUserActive(adminCtx(), args("2", true))
	assert.ErrorContains(t, err, "protected admin")
}

func TestResolver_UpdateMyProfile(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "me@example.com", Role: identity.RoleMember, IsActive: true},
	)
	r := &Resolver{IdentityService: svc}
	ctx := testenv.WithUser(context.Background(), 1, "me@example.com")
	str := func(s string) *string { return &s }
	input := func(first, last, backup *string) struct{ Input updateProfileInput } {
		return struct{ Input updateProfileInput }{Input: updateProfileInput{FirstName: first, LastName: last, BackupEmail: backup}}
	}

	// Full update.
	got, err := r.UpdateMyProfile(ctx, input(str("Ada"), str("Lovelace"), str("alt@example.com")))
	require.NoError(t, err)
	assert.Equal(t, "Ada", *got.FirstName())
	assert.Equal(t, "Lovelace", *got.LastName())
	assert.Equal(t, "alt@example.com", *got.BackupEmail())

	// Nil fields leave stored values untouched; empty clears.
	got, err = r.UpdateMyProfile(ctx, input(nil, nil, str("")))
	require.NoError(t, err)
	assert.Equal(t, "Ada", *got.FirstName())
	assert.Nil(t, got.BackupEmail())

	// Validation.
	_, err = r.UpdateMyProfile(ctx, input(nil, nil, str("not-an-email")))
	assert.ErrorContains(t, err, "backupEmail")

	long := string(make([]byte, 101))
	_, err = r.UpdateMyProfile(ctx, input(str(long), nil, nil))
	assert.ErrorContains(t, err, "firstName")

	// Unauthenticated.
	_, err = r.UpdateMyProfile(context.Background(), input(str("x"), nil, nil))
	assert.EqualError(t, err, "unauthorized")
}

func TestResolver_Me_UsesIdentityService(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "me@example.com", Role: identity.RoleAdmin, IsActive: true, FirstName: "Ada"},
	)
	svc.protected["me@example.com"] = true
	r := &Resolver{IdentityService: svc}

	res, err := r.Me(adminCtx())
	require.NoError(t, err)
	assert.Equal(t, "admin", res.Role())
	assert.True(t, res.IsProtected())
	assert.Equal(t, "Ada", *res.FirstName())
}
