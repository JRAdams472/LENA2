package bff

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

// fakeIdentityService is an in-memory IdentityService for resolver tests.
type fakeIdentityService struct {
	users       map[int64]identity.User
	protected   map[string]bool
	roleErr     error
	activeErr   error
	profileErr  error
	approvalErr error
	deleteErr   error
	// memberships mirrors household_member for LEN-26: user → household →
	// role. When nil, a user's household_id counts as their only membership
	// (the P1 1:1 invariant).
	memberships map[int64]map[int64]string
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

func (f *fakeIdentityService) IsProtected(_ string, email string) bool {
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

func (f *fakeIdentityService) ListPendingUsers(_ context.Context, limit, offset int32) ([]identity.User, error) {
	var out []identity.User
	for _, u := range f.users {
		if u.IsPending() {
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

func (f *fakeIdentityService) CountPendingUsers(context.Context) (int64, error) {
	var n int64
	for _, u := range f.users {
		if u.IsPending() {
			n++
		}
	}
	return n, nil
}

func (f *fakeIdentityService) AdminSetApproval(_ context.Context, _, targetID int64, status, _ string) error {
	if f.approvalErr != nil {
		return f.approvalErr
	}
	u := f.users[targetID]
	u.ApprovalStatus = status
	f.users[targetID] = u
	return nil
}

func (f *fakeIdentityService) AcceptDisclaimer(_ context.Context, userID int64, _ string) error {
	u, ok := f.users[userID]
	if !ok {
		return errors.New("not found")
	}
	now := time.Now()
	u.DisclaimerAcceptedAt = &now
	f.users[userID] = u
	return nil
}

func (f *fakeIdentityService) UpdateProfile(_ context.Context, userID int64, displayName, first, last, backup string, birthdate *time.Time, _ string) error {
	if f.profileErr != nil {
		return f.profileErr
	}
	u := f.users[userID]
	u.DisplayName, u.FirstName, u.LastName, u.BackupEmail, u.Birthdate = displayName, first, last, backup, birthdate
	f.users[userID] = u
	return nil
}

func (f *fakeIdentityService) SetUserHousehold(_ context.Context, userID, householdID int64, role string, _ *int64) error {
	u, ok := f.users[userID]
	if !ok {
		return errors.New("not found")
	}
	u.HouseholdID = &householdID
	u.HouseholdRole = role
	f.users[userID] = u
	return nil
}

func (f *fakeIdentityService) SetUserHouseholdRole(_ context.Context, userID, householdID int64, role, _ string) error {
	u, ok := f.users[userID]
	if !ok || u.HouseholdID == nil || *u.HouseholdID != householdID {
		return domainerr.ErrConflict
	}
	u.HouseholdRole = role
	f.users[userID] = u
	if f.memberships != nil && f.memberships[userID] != nil {
		f.memberships[userID][householdID] = role
	}
	return nil
}

func (f *fakeIdentityService) SetActiveHousehold(_ context.Context, userID, householdID int64, _ string) error {
	u, ok := f.users[userID]
	if !ok {
		return errors.New("not found")
	}
	role := ""
	if f.memberships != nil {
		role = f.memberships[userID][householdID]
	}
	if role == "" && u.HouseholdID != nil && *u.HouseholdID == householdID {
		role = u.HouseholdRole
	}
	if role == "" {
		return domainerr.ErrConflict
	}
	u.HouseholdID = &householdID
	u.HouseholdRole = role
	f.users[userID] = u
	return nil
}

func (f *fakeIdentityService) CountUsersByHousehold(_ context.Context, householdID int64) (int64, error) {
	var n int64
	for _, u := range f.users {
		if u.HouseholdID != nil && *u.HouseholdID == householdID {
			n++
		}
	}
	return n, nil
}

func (f *fakeIdentityService) SetUserSearchable(_ context.Context, userID int64, searchable bool, _ string) error {
	u, ok := f.users[userID]
	if !ok {
		return errors.New("not found")
	}
	u.IsSearchable = searchable
	f.users[userID] = u
	return nil
}

func (f *fakeIdentityService) ListUsersByHousehold(_ context.Context, householdID int64) ([]identity.User, error) {
	var out []identity.User
	for _, u := range f.users {
		if u.HouseholdID != nil && *u.HouseholdID == householdID {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeIdentityService) ListUsersByIDs(_ context.Context, ids []int64) ([]identity.User, error) {
	out := make([]identity.User, 0, len(ids))
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeIdentityService) DeleteUser(_ context.Context, userID int64) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.users[userID]; !ok {
		return domainerr.ErrNotFound
	}
	delete(f.users, userID)
	return nil
}

func (f *fakeIdentityService) SearchUsers(_ context.Context, term string, excludeUserID, excludeHouseholdID int64, limit int32) ([]identity.User, error) {
	term = strings.ToLower(term)
	var out []identity.User
	for _, u := range f.users {
		if u.UserID == excludeUserID || !u.IsActive || !u.IsSearchable {
			continue
		}
		if u.HouseholdID != nil && *u.HouseholdID == excludeHouseholdID {
			continue
		}
		hay := strings.ToLower(u.Email + " " + u.DisplayName + " " + u.FirstName + " " + u.LastName)
		if strings.Contains(hay, term) {
			out = append(out, u)
		}
	}
	if int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
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
	_, err = r.Users(testutil.WithUser(context.Background(), 2, "m@example.com"), struct {
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
	_, err := r.SetUserRole(testutil.WithUser(context.Background(), 3, "x@x.com"), args("2", "admin"))
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
	auth := &fakeAuthInvalidator{}
	r := &Resolver{IdentityService: svc, AuthInvalidator: auth}
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
	// The ban evicts the target's cached identity so it applies immediately.
	assert.Equal(t, []int64{2}, auth.userIDs)

	svc.activeErr = identity.ErrProtectedUser
	_, err = r.SetUserActive(adminCtx(), args("2", true))
	assert.ErrorContains(t, err, "protected admin")
}

func TestResolver_UpdateMyProfile(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "me@example.com", Role: identity.RoleMember, IsActive: true},
	)
	r := &Resolver{IdentityService: svc}
	ctx := testutil.WithUser(context.Background(), 1, "me@example.com")
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

	// displayName persists through the same input and stays put when unset.
	got, err = r.UpdateMyProfile(ctx, struct{ Input updateProfileInput }{
		Input: updateProfileInput{DisplayName: str("Ada L.")},
	})
	require.NoError(t, err)
	assert.Equal(t, "Ada L.", *got.DisplayName())
	assert.Equal(t, "Ada", *got.FirstName())
	got, err = r.UpdateMyProfile(ctx, input(nil, nil, nil))
	require.NoError(t, err)
	assert.Equal(t, "Ada L.", *got.DisplayName())

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

// fakeAuthInvalidator records auth-cache evictions for assertions.
type fakeAuthInvalidator struct {
	users   []string
	userIDs []int64
}

func (f *fakeAuthInvalidator) InvalidateUser(provider, subject string) {
	f.users = append(f.users, provider+"|"+subject)
}

func (f *fakeAuthInvalidator) InvalidateUserID(_ context.Context, userID int64) {
	f.userIDs = append(f.userIDs, userID)
}

func TestResolver_UpdateMyProfile_IsSearchable(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "me@example.com", Role: identity.RoleMember, IsActive: true, IsSearchable: true},
	)
	auth := &fakeAuthInvalidator{}
	r := &Resolver{IdentityService: svc, AuthInvalidator: auth}
	ctx := testutil.WithUser(context.Background(), 1, "me@example.com")
	off := false

	got, err := r.UpdateMyProfile(ctx, struct{ Input updateProfileInput }{
		Input: updateProfileInput{IsSearchable: &off},
	})
	require.NoError(t, err)
	assert.False(t, got.IsSearchable())
	// A change evicts the cached identity resolution.
	require.Len(t, auth.users, 1)

	// Resubmitting the same value does not invalidate again.
	_, err = r.UpdateMyProfile(ctx, struct{ Input updateProfileInput }{
		Input: updateProfileInput{IsSearchable: &off},
	})
	require.NoError(t, err)
	assert.Len(t, auth.users, 1)
}

func TestResolver_User_HouseholdSelfGated(t *testing.T) {
	hhID := int64(42)
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "admin@example.com", Role: identity.RoleAdmin, IsActive: true, HouseholdID: &hhID},
		identity.User{UserID: 2, Email: "me@example.com", Role: identity.RoleMember, IsActive: true, HouseholdID: &hhID},
	)
	h := mock.NewMockHouseholdService(gomock.NewController(t))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhID).
		Return(household.Household{HouseholdID: hhID}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhID).
		Return([]household.Membership{{HouseholdID: hhID, UserID: 2, Role: identity.HouseholdRoleMember}}, nil)
	r := &Resolver{IdentityService: svc, HouseholdService: h}

	// Caller resolving their own user sees the household.
	self, err := r.userByID(testutil.WithUser(context.Background(), 2, "me@example.com"), 2)
	require.NoError(t, err)
	hh, err := self.Household(testutil.WithUser(context.Background(), 2, "me@example.com"))
	require.NoError(t, err)
	require.NotNil(t, hh)
	assert.Equal(t, graphql.ID("42"), hh.ID())

	// A different caller resolving user 2 sees no household — the admin
	// list path must not leak membership.
	other, err := r.userByID(adminCtx(), 2)
	require.NoError(t, err)
	hh, err = other.Household(adminCtx())
	require.NoError(t, err)
	assert.Nil(t, hh)
}

func TestResolver_PendingUsers_AdminOnly(t *testing.T) {
	r := &Resolver{IdentityService: newFakeIdentityService(
		identity.User{UserID: 1, Email: "admin@example.com", Role: identity.RoleAdmin, IsActive: true},
		identity.User{UserID: 2, Email: "new@example.com", Role: identity.RoleMember, IsActive: true, ApprovalStatus: identity.StatusPending},
		identity.User{UserID: 3, Email: "old@example.com", Role: identity.RoleMember, IsActive: true, ApprovalStatus: identity.StatusApproved},
	)}
	args := func() struct {
		Page     int32
		PageSize int32
	} {
		return struct {
			Page     int32
			PageSize int32
		}{Page: 1, PageSize: 25}
	}

	// Unauthenticated and non-admin are rejected like the users list.
	_, err := r.PendingUsers(context.Background(), args())
	assert.EqualError(t, err, "unauthorized")
	_, err = r.PendingUsers(testutil.WithUser(context.Background(), 2, "m@example.com"), args())
	assert.EqualError(t, err, "forbidden: admin role required")

	page, err := r.PendingUsers(adminCtx(), args())
	require.NoError(t, err)
	require.Len(t, page.Items(), 1)
	assert.Equal(t, "new@example.com", page.Items()[0].Email())
	assert.Equal(t, "PENDING", page.Items()[0].ApprovalStatus())
	assert.Equal(t, int32(1), page.PageInfo().TotalCount())
}

func TestResolver_ApproveDenyUser(t *testing.T) {
	svc := newFakeIdentityService(
		identity.User{UserID: 1, Email: "admin@example.com", Role: identity.RoleAdmin, IsActive: true},
		identity.User{UserID: 2, Email: "new@example.com", Role: identity.RoleMember, IsActive: true, ApprovalStatus: identity.StatusPending},
	)
	auth := &fakeAuthInvalidator{}
	r := &Resolver{IdentityService: svc, AuthInvalidator: auth}
	args := func(id string) struct{ UserID graphql.ID } {
		return struct{ UserID graphql.ID }{UserID: graphql.ID(id)}
	}

	// Non-admin cannot approve.
	_, err := r.ApproveUser(testutil.WithUser(context.Background(), 3, "x@x.com"), args("2"))
	assert.EqualError(t, err, "forbidden: admin role required")

	// Approve flips the status and evicts the cached identity.
	got, err := r.ApproveUser(adminCtx(), args("2"))
	require.NoError(t, err)
	assert.Equal(t, "APPROVED", got.ApprovalStatus())
	assert.Equal(t, []int64{2}, auth.userIDs)

	// Deny records the durable status and evicts again.
	got, err = r.DenyUser(adminCtx(), args("2"))
	require.NoError(t, err)
	assert.Equal(t, "DENIED", got.ApprovalStatus())
	assert.Equal(t, []int64{2, 2}, auth.userIDs)

	// Guard errors map to FORBIDDEN.
	svc.approvalErr = identity.ErrProtectedUser
	_, err = r.DenyUser(adminCtx(), args("2"))
	assert.ErrorContains(t, err, "protected admin")

	svc.approvalErr = identity.ErrSelfModification
	_, err = r.ApproveUser(adminCtx(), args("1"))
	assert.ErrorContains(t, err, "your own role")
}

func TestResolver_DeleteMyAccount(t *testing.T) {
	newMocks := func(t *testing.T) (*mock.MockHouseholdService, *mock.MockUserPrefsService, *mock.MockMealPlanService, *mock.MockGroceryService) {
		ctrl := gomock.NewController(t)
		return mock.NewMockHouseholdService(ctrl), mock.NewMockUserPrefsService(ctrl),
			mock.NewMockMealPlanService(ctrl), mock.NewMockGroceryService(ctrl)
	}
	ctx := testutil.WithUser(context.Background(), 7, "me@example.com")
	membership := household.Membership{HouseholdID: 42, UserID: 7, Role: identity.HouseholdRoleOwner}
	mate := household.Membership{HouseholdID: 42, UserID: 9, Role: identity.HouseholdRoleMember}

	t.Run("unauthenticated", func(t *testing.T) {
		r := &Resolver{}
		ok, err := r.DeleteMyAccount(context.Background())
		assert.False(t, ok)
		assert.EqualError(t, err, "unauthorized")
	})

	t.Run("sole household dissolved", func(t *testing.T) {
		svc := newFakeIdentityService(identity.User{UserID: 7, Email: "me@example.com", Role: identity.RoleMember, IsActive: true})
		hh, prefs, plans, grocery := newMocks(t)
		auth := &fakeAuthInvalidator{}
		gomock.InOrder(
			hh.EXPECT().ListMembershipsByUser(gomock.Any(), int64(7)).Return([]household.Membership{membership}, nil),
			hh.EXPECT().LockHousehold(gomock.Any(), int64(42)).Return(household.Household{HouseholdID: 42}, nil),
			hh.EXPECT().ListMembersByHousehold(gomock.Any(), int64(42)).Return([]household.Membership{membership}, nil),
			prefs.EXPECT().DeleteHouseholdStock(gomock.Any(), int64(42)).Return(nil),
			plans.EXPECT().DeleteHouseholdPlans(gomock.Any(), int64(42)).Return(nil),
			grocery.EXPECT().DeleteHouseholdLists(gomock.Any(), int64(42)).Return(nil),
			hh.EXPECT().DeleteHousehold(gomock.Any(), int64(42)).Return(nil),
		)
		r := &Resolver{
			IdentityService: svc, HouseholdService: hh, UserPrefsService: prefs,
			MealPlanService: plans, GroceryService: grocery, AuthInvalidator: auth,
		}
		ok, err := r.DeleteMyAccount(ctx)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Empty(t, svc.users)
		assert.Len(t, auth.users, 1)
	})

	t.Run("shared household kept", func(t *testing.T) {
		svc := newFakeIdentityService(identity.User{UserID: 7, Email: "me@example.com", Role: identity.RoleMember, IsActive: true})
		hh, prefs, plans, grocery := newMocks(t)
		memberSeat := household.Membership{HouseholdID: 42, UserID: 7, Role: identity.HouseholdRoleMember}
		gomock.InOrder(
			hh.EXPECT().ListMembershipsByUser(gomock.Any(), int64(7)).Return([]household.Membership{memberSeat}, nil),
			hh.EXPECT().LockHousehold(gomock.Any(), int64(42)).Return(household.Household{HouseholdID: 42}, nil),
			hh.EXPECT().ListMembersByHousehold(gomock.Any(), int64(42)).Return([]household.Membership{
				{HouseholdID: 42, UserID: 8, Role: identity.HouseholdRoleOwner},
				memberSeat,
				mate,
			}, nil),
		)
		// No stock/plans/lists/household deletes — the household outlives the member.
		r := &Resolver{
			IdentityService: svc, HouseholdService: hh, UserPrefsService: prefs,
			MealPlanService: plans, GroceryService: grocery,
		}
		ok, err := r.DeleteMyAccount(ctx)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("shared household owner promotes a successor", func(t *testing.T) {
		svc := newFakeIdentityService(
			identity.User{UserID: 7, Email: "me@example.com", Role: identity.RoleMember, IsActive: true},
			identity.User{UserID: 9, Email: "mate@example.com", Role: identity.RoleMember, IsActive: true},
		)
		hh, prefs, plans, grocery := newMocks(t)
		auth := &fakeAuthInvalidator{}
		hh.EXPECT().ListMembershipsByUser(gomock.Any(), int64(7)).Return([]household.Membership{membership}, nil)
		hh.EXPECT().LockHousehold(gomock.Any(), int64(42)).Return(household.Household{HouseholdID: 42}, nil)
		hh.EXPECT().ListMembersByHousehold(gomock.Any(), int64(42)).Return([]household.Membership{membership, mate}, nil)
		// Ownership passes to the earliest remaining member before the
		// caller's row (and owner seat) is deleted.
		hh.EXPECT().JoinHousehold(gomock.Any(), int64(42), int64(9), identity.HouseholdRoleOwner, "me@example.com").Return(household.Membership{}, nil)
		hh.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		r := &Resolver{
			IdentityService: svc, HouseholdService: hh, UserPrefsService: prefs,
			MealPlanService: plans, GroceryService: grocery, AuthInvalidator: auth,
		}
		ok, err := r.DeleteMyAccount(ctx)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Contains(t, auth.userIDs, int64(9))
	})

	t.Run("no memberships", func(t *testing.T) {
		svc := newFakeIdentityService(identity.User{UserID: 7, Email: "me@example.com", Role: identity.RoleMember, IsActive: true})
		hh, prefs, plans, grocery := newMocks(t)
		hh.EXPECT().ListMembershipsByUser(gomock.Any(), int64(7)).Return(nil, nil)
		r := &Resolver{
			IdentityService: svc, HouseholdService: hh, UserPrefsService: prefs,
			MealPlanService: plans, GroceryService: grocery,
		}
		ok, err := r.DeleteMyAccount(ctx)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("last admin refused", func(t *testing.T) {
		svc := newFakeIdentityService(identity.User{UserID: 7, Email: "me@example.com", Role: identity.RoleAdmin, IsActive: true})
		svc.deleteErr = domainerr.ErrLastAdmin
		hh, prefs, plans, grocery := newMocks(t)
		hh.EXPECT().ListMembershipsByUser(gomock.Any(), int64(7)).Return(nil, nil)
		r := &Resolver{
			IdentityService: svc, HouseholdService: hh, UserPrefsService: prefs,
			MealPlanService: plans, GroceryService: grocery,
		}
		ok, err := r.DeleteMyAccount(ctx)
		assert.False(t, ok)
		assert.ErrorContains(t, err, "last active admin")
	})
}

// acceptDisclaimer stamps acceptance on the caller and reflects on the
// reloaded user; missing users surface the service error.
func TestAcceptDisclaimer(t *testing.T) {
	ctx := testutil.WithUser(context.Background(), 7, "me@example.com")

	t.Run("stamps and reflects", func(t *testing.T) {
		svc := newFakeIdentityService(identity.User{UserID: 7, Email: "me@example.com", IsActive: true})
		r := &Resolver{IdentityService: svc}
		ok, err := r.AcceptDisclaimer(ctx)
		require.NoError(t, err)
		assert.True(t, ok)

		u, err := svc.GetByID(ctx, 7)
		require.NoError(t, err)
		require.NotNil(t, u.DisclaimerAcceptedAt)
		assert.True(t, (&userResolver{u: u}).DisclaimerAccepted())

		// Idempotent — a second accept re-stamps without error.
		ok, err = r.AcceptDisclaimer(ctx)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unaccepted resolves false", func(t *testing.T) {
		u := identity.User{UserID: 7}
		assert.False(t, (&userResolver{u: u}).DisclaimerAccepted())
	})
}
