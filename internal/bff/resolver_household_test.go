package bff

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

const (
	hhUserID = int64(7)
	hhEmail  = "hh-caller@example.com"
)

func hhCtx() context.Context {
	return testutil.WithUser(context.Background(), hhUserID, hhEmail)
}

func TestResolver_MyHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	created := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID, CreatedAt: created}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).Return([]household.Membership{
		{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleOwner},
		{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleMember},
	}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{hhUserID, 9})).Return([]identity.User{
		{UserID: hhUserID, Email: hhEmail, DisplayName: "Caller"},
		{UserID: 9, Email: "mate@example.com", DisplayName: "Mate"},
	}, nil)

	res, err := r.MyHousehold(hhCtx())
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("7"), res.ID())
	assert.Equal(t, "OWNER", res.MyRole())
	assert.True(t, res.IsActive())
	require.Len(t, res.Members(), 2)
	assert.Equal(t, "Caller", *res.Members()[0].User().DisplayName())
	assert.Equal(t, "OWNER", res.Members()[0].Role())
	assert.True(t, res.Members()[0].IsMe())
	// Restricted projection carries no email/role surface.
	assert.Equal(t, graphql.ID("9"), res.Members()[1].User().ID())
	assert.Equal(t, "MEMBER", res.Members()[1].Role())
	assert.False(t, res.Members()[1].IsMe())
}

func TestResolver_MyHousehold_None(t *testing.T) {
	r := &Resolver{}
	ctx := testutil.WithHousehold(context.Background(), hhUserID, 0, hhEmail)
	res, err := r.MyHousehold(ctx)
	require.NoError(t, err)
	assert.Nil(t, res)
}

func TestResolver_HouseholdInvites(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	h.EXPECT().ListPendingInvitesForUser(gomock.Any(), hhUserID).Return([]household.Invite{
		{InviteID: 21, FromUserID: 9, ToUserID: hhUserID, HouseholdID: 42, Status: household.StatusPending},
	}, nil)
	h.EXPECT().ListSentInvitesForUser(gomock.Any(), hhUserID, hhUserID).Return([]household.Invite{
		{InviteID: 22, FromUserID: hhUserID, ToUserID: 11, HouseholdID: hhUserID, Status: household.StatusPending},
	}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{9, hhUserID, 11})).Return([]identity.User{
		{UserID: 9, DisplayName: "Inviter"},
		{UserID: hhUserID, DisplayName: "Caller"},
		{UserID: 11, DisplayName: "Target"},
	}, nil)

	out, err := r.HouseholdInvites(hhCtx())
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "PENDING", out[0].Status())
	assert.Equal(t, "Inviter", *out[0].FromUser().DisplayName())
	assert.Equal(t, "Target", *out[1].ToUser().DisplayName())
}

func TestResolver_SearchHouseholdUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{IdentityService: idSvc}

	idSvc.EXPECT().SearchUsers(gomock.Any(), "ali", hhUserID, hhUserID, int32(50)).
		Return([]identity.User{{UserID: 42, DisplayName: "Alice"}}, nil)

	out, err := r.SearchHouseholdUsers(hhCtx(), struct {
		Term  string
		Limit int32
	}{Term: " ali ", Limit: 500})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, graphql.ID("42"), out[0].ID())
}

func TestResolver_SearchHouseholdUsers_ShortTerm(t *testing.T) {
	r := &Resolver{}
	_, err := r.SearchHouseholdUsers(hhCtx(), struct {
		Term  string
		Limit int32
	}{Term: "x", Limit: 20})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 characters")
}

func TestResolver_InviteHouseholdMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	otherHH := int64(50)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: true, IsSearchable: true}, nil)
	// Not already a mate — membership check, not pointer equality.
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{}, domainerr.ErrNotFound)
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), hhUserID).Return(int64(2), nil)
	h.EXPECT().CreateInvite(gomock.Any(), hhUserID, int64(9), hhUserID, hhEmail).
		Return(household.Invite{InviteID: 30, FromUserID: hhUserID, ToUserID: 9, HouseholdID: hhUserID, Status: household.StatusPending}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteReceived, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{hhUserID, 9})).
		Return([]identity.User{{UserID: hhUserID}, {UserID: 9}}, nil)

	res, err := r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("30"), res.ID())
}

func TestResolver_InviteHouseholdMember_Guards(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	// Self-invite is rejected before any store call.
	_, err := r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "7"})
	assert.Error(t, err)

	// Already a member of the caller's household — even though their
	// active pointer is elsewhere, the membership check still catches it.
	otherHH := int64(50)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: true, IsSearchable: true}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: "member"}, nil)
	_, err = r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.ErrorContains(t, err, "cannot invite this user")
}

func TestResolver_AcceptHouseholdInvite(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	prefs := mock.NewMockUserPrefsService(ctrl)
	mp := mock.NewMockMealPlanService(ctrl)
	groc := mock.NewMockGroceryService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{
		HouseholdService: h, IdentityService: idSvc, UserPrefsService: prefs,
		MealPlanService: mp, GroceryService: groc, AuthInvalidator: auth,
	}

	inviterHH := int64(42)
	inv := household.Invite{InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending}

	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(inv, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	h.EXPECT().CountMembers(gomock.Any(), inviterHH).Return(int64(1), nil)
	// The inviter must still be a member of the invited household.
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, int64(9)).
		Return(household.Membership{HouseholdID: inviterHH, UserID: 9, Role: "owner"}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusAccepted, hhEmail).
		Return(household.Invite{InviteID: 55, Status: household.StatusAccepted}, nil)
	// Not already a member → join as member. The caller's old membership
	// stays — accept is add-membership, not a move.
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, hhUserID).
		Return(household.Membership{}, domainerr.ErrNotFound)
	h.EXPECT().JoinHousehold(gomock.Any(), inviterHH, hhUserID, identity.HouseholdRoleMember, hhEmail).
		Return(household.Membership{HouseholdID: inviterHH, UserID: hhUserID, Role: "member"}, nil)
	// No merge arg → no merge calls, and the old household is kept.
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, inviterHH, hhEmail).Return(nil)
	// invite_accepted to the inviter; member_joined to other members.
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteAccepted, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	// Once inside acceptInvite for member_joined fan-out, once to build
	// the returned Household.
	h.EXPECT().ListMembersByHousehold(gomock.Any(), inviterHH).
		Return([]household.Membership{
			{HouseholdID: inviterHH, UserID: 9, Role: "owner"},
			{HouseholdID: inviterHH, UserID: hhUserID, Role: "member"},
		}, nil).Times(2)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{9, hhUserID})).
		Return([]identity.User{{UserID: 9}, {UserID: hhUserID}}, nil)

	res, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("42"), res.ID())
	assert.True(t, res.IsActive())
	assert.Len(t, res.Members(), 2)
}

func TestResolver_AcceptHouseholdInvite_NotParty(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	r := &Resolver{HouseholdService: h}

	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: 99, Status: household.StatusPending,
	}, nil)

	_, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55"})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestResolver_AcceptHouseholdInvite_StaleInviter(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	inviterHH := int64(42)
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending,
	}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	h.EXPECT().CountMembers(gomock.Any(), inviterHH).Return(int64(1), nil)
	// The inviter left the household the invite points at — accept
	// conflicts even though they may be active elsewhere.
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, int64(9)).
		Return(household.Membership{}, domainerr.ErrNotFound)

	_, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55"})
	assert.ErrorIs(t, err, domainerr.ErrConflict)
}

func TestResolver_DeclineAndCancelHouseholdInvite(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	// Decline: caller is the invitee.
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: hhUserID, Status: household.StatusPending,
	}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusDeclined, hhEmail).
		Return(household.Invite{InviteID: 55, FromUserID: 9, ToUserID: hhUserID, Status: household.StatusDeclined}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteDeclined, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{9, hhUserID})).
		Return([]identity.User{{UserID: 9}, {UserID: hhUserID}}, nil)

	res, err := r.DeclineHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "55"})
	require.NoError(t, err)
	assert.Equal(t, "DECLINED", res.Status())

	// Cancel: caller is the inviter.
	h.EXPECT().GetInviteByID(gomock.Any(), int64(56)).Return(household.Invite{
		InviteID: 56, FromUserID: hhUserID, ToUserID: 9, Status: household.StatusPending,
	}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(56), household.StatusCancelled, hhEmail).
		Return(household.Invite{InviteID: 56, FromUserID: hhUserID, ToUserID: 9, Status: household.StatusCancelled}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteCancelled, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{hhUserID, 9})).
		Return([]identity.User{{UserID: hhUserID}, {UserID: 9}}, nil)

	res, err = r.CancelHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "56"})
	require.NoError(t, err)
	assert.Equal(t, "CANCELLED", res.Status())

	// Wrong party cannot decline an invite addressed to someone else.
	h.EXPECT().GetInviteByID(gomock.Any(), int64(57)).Return(household.Invite{
		InviteID: 57, FromUserID: 9, ToUserID: 11, Status: household.StatusPending,
	}, nil)
	_, err = r.DeclineHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "57"})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestResolver_LeaveHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	currentHH := int64(42)
	ctx := testutil.WithHousehold(context.Background(), hhUserID, currentHH, hhEmail)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &currentHH, HouseholdRole: identity.HouseholdRoleOwner}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), currentHH).
		Return(household.Household{HouseholdID: currentHH}, nil)
	h.EXPECT().GetMembership(gomock.Any(), currentHH, hhUserID).
		Return(household.Membership{HouseholdID: currentHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}, nil)
	// Sole member leaving: no ownership transfer.
	h.EXPECT().ListMembersByHousehold(gomock.Any(), currentHH).
		Return([]household.Membership{{HouseholdID: currentHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), hhUserID, currentHH, hhEmail).
		Return(nil, nil)
	h.EXPECT().RemoveMembership(gomock.Any(), currentHH, hhUserID).Return(nil)
	// The active household was left and no memberships remain → fresh
	// solo household, activated.
	h.EXPECT().ListMembershipsByUser(gomock.Any(), hhUserID).Return(nil, nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), int64(90), hhUserID, identity.HouseholdRoleOwner, hhEmail).
		Return(household.Membership{HouseholdID: 90, UserID: hhUserID, Role: "owner"}, nil)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, int64(90), hhEmail).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")

	ok, err := r.LeaveHousehold(ctx, struct{ HouseholdID *graphql.ID }{})
	require.NoError(t, err)
	assert.True(t, ok)
}

// Leaving a non-active household removes only that membership — the
// active pointer never moves, so no fallback activation runs.
func TestResolver_LeaveHousehold_Inactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	activeHH, otherHH := int64(42), int64(60)
	ctx := testutil.WithHousehold(context.Background(), hhUserID, activeHH, hhEmail)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &activeHH}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), otherHH).
		Return(household.Household{HouseholdID: otherHH}, nil)
	h.EXPECT().GetMembership(gomock.Any(), otherHH, hhUserID).
		Return(household.Membership{HouseholdID: otherHH, UserID: hhUserID, Role: identity.HouseholdRoleMember}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), otherHH).
		Return([]household.Membership{
			{HouseholdID: otherHH, UserID: 9, Role: identity.HouseholdRoleOwner},
			{HouseholdID: otherHH, UserID: hhUserID, Role: identity.HouseholdRoleMember},
		}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), hhUserID, otherHH, hhEmail).Return(nil, nil)
	h.EXPECT().RemoveMembership(gomock.Any(), otherHH, hhUserID).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberLeft, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")

	leaveID := graphql.ID("60")
	ok, err := r.LeaveHousehold(ctx, struct{ HouseholdID *graphql.ID }{HouseholdID: &leaveID})
	require.NoError(t, err)
	assert.True(t, ok)
}

// Leaving the active household while another membership remains activates
// the earliest remaining one — no fresh household is created.
func TestResolver_LeaveHousehold_ActivatesEarliestRemaining(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	activeHH := int64(42)
	ctx := testutil.WithHousehold(context.Background(), hhUserID, activeHH, hhEmail)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &activeHH}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), activeHH).
		Return(household.Household{HouseholdID: activeHH}, nil)
	h.EXPECT().GetMembership(gomock.Any(), activeHH, hhUserID).
		Return(household.Membership{HouseholdID: activeHH, UserID: hhUserID, Role: identity.HouseholdRoleMember}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), activeHH).
		Return([]household.Membership{
			{HouseholdID: activeHH, UserID: 9, Role: identity.HouseholdRoleOwner},
			{HouseholdID: activeHH, UserID: hhUserID, Role: identity.HouseholdRoleMember},
		}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), hhUserID, activeHH, hhEmail).Return(nil, nil)
	h.EXPECT().RemoveMembership(gomock.Any(), activeHH, hhUserID).Return(nil)
	h.EXPECT().ListMembershipsByUser(gomock.Any(), hhUserID).
		Return([]household.Membership{{HouseholdID: 60, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, int64(60), hhEmail).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberLeft, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")

	ok, err := r.LeaveHousehold(ctx, struct{ HouseholdID *graphql.ID }{})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_InviteHouseholdMember_RoleGate(t *testing.T) {
	r := &Resolver{}
	// A plain member cannot invite.
	memberCtx := testutil.WithHouseholdRole(context.Background(), hhUserID, hhUserID, "member", hhEmail)
	_, err := r.InviteHouseholdMember(memberCtx, struct{ UserID graphql.ID }{UserID: "9"})
	assert.Error(t, err)

	// An admin can proceed to the store layer.
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r = &Resolver{HouseholdService: h, IdentityService: idSvc}
	adminCtx := testutil.WithHouseholdRole(context.Background(), hhUserID, hhUserID, "admin", hhEmail)
	otherHH := int64(50)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: true, IsSearchable: true}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{}, domainerr.ErrNotFound)
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), hhUserID).Return(int64(2), nil)
	h.EXPECT().CreateInvite(gomock.Any(), hhUserID, int64(9), hhUserID, hhEmail).
		Return(household.Invite{InviteID: 30, FromUserID: hhUserID, ToUserID: 9, HouseholdID: hhUserID, Status: household.StatusPending}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteReceived, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
	_, err = r.InviteHouseholdMember(adminCtx, struct{ UserID graphql.ID }{UserID: "9"})
	require.NoError(t, err)
}

func TestResolver_InviteHouseholdMember_Full(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	otherHH := int64(50)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: true, IsSearchable: true}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{}, domainerr.ErrNotFound)
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), hhUserID).
		Return(int64(maxHouseholdMembers), nil)

	_, err := r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.ErrorContains(t, err, "household is full")
}

func TestResolver_InviteHouseholdMember_GenericTargetErrors(t *testing.T) {
	// Every target-side rejection returns the identical generic error so
	// the endpoint cannot enumerate accounts or discoverability state
	// (LEN-29 finding 3).
	otherHH := int64(50)
	cases := []struct {
		name  string
		setup func(idSvc *mock.MockIdentityService, h *mock.MockHouseholdService)
	}{
		{"unknown user", func(idSvc *mock.MockIdentityService, _ *mock.MockHouseholdService) {
			idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
				Return(identity.User{}, fmt.Errorf("get user by id: %w", domainerr.ErrNotFound))
		}},
		{"inactive user", func(idSvc *mock.MockIdentityService, _ *mock.MockHouseholdService) {
			idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
				Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: false, IsSearchable: true}, nil)
		}},
		{"unsearchable user", func(idSvc *mock.MockIdentityService, _ *mock.MockHouseholdService) {
			idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
				Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: true, IsSearchable: false}, nil)
		}},
		{"duplicate pending invite", func(idSvc *mock.MockIdentityService, h *mock.MockHouseholdService) {
			idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
				Return(identity.User{UserID: 9, HouseholdID: &otherHH, IsActive: true, IsSearchable: true}, nil)
			h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
				Return(household.Membership{}, domainerr.ErrNotFound)
			idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), hhUserID).Return(int64(2), nil)
			h.EXPECT().CreateInvite(gomock.Any(), hhUserID, int64(9), hhUserID, hhEmail).
				Return(household.Invite{}, fmt.Errorf("create invite: %w", domainerr.ErrConflict))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := mock.NewMockHouseholdService(ctrl)
			idSvc := mock.NewMockIdentityService(ctrl)
			r := &Resolver{HouseholdService: h, IdentityService: idSvc}
			tc.setup(idSvc, h)
			_, err := r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
			assert.EqualError(t, err, "cannot invite this user")
		})
	}
}

func TestResolver_InviteHouseholdMember_RateLimited(t *testing.T) {
	r := &Resolver{invites: newUserRateLimiter(1)}
	// Drain the single token; the next invite attempt is throttled before
	// any store call, so no mocks are needed.
	r.inviteLimiter().allow(hhUserID)
	_, err := r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.ErrorContains(t, err, "too many invites")
}

func TestResolver_AcceptHouseholdInvite_Full(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	inviterHH := int64(42)
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending,
	}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	h.EXPECT().CountMembers(gomock.Any(), inviterHH).
		Return(int64(maxHouseholdMembers), nil)

	_, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55"})
	assert.ErrorContains(t, err, "household is full")
}

func TestResolver_RenameHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	name := "The Smiths"
	h.EXPECT().RenameHousehold(gomock.Any(), hhUserID, name, hhEmail).
		Return(household.Household{HouseholdID: hhUserID, Name: &name}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).
		Return([]household.Membership{
			{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleOwner},
			{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleMember},
		}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindHouseholdRenamed, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID, Name: &name}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).
		Return([]household.Membership{{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{hhUserID}).
		Return([]identity.User{{UserID: hhUserID}}, nil)

	res, err := r.RenameHousehold(hhCtx(), struct{ Name string }{Name: name})
	require.NoError(t, err)
	assert.Equal(t, name, *res.Name())
}

func TestResolver_RenameHousehold_MemberForbidden(t *testing.T) {
	r := &Resolver{}
	memberCtx := testutil.WithHouseholdRole(context.Background(), hhUserID, hhUserID, "member", hhEmail)
	_, err := r.RenameHousehold(memberCtx, struct{ Name string }{Name: "x"})
	assert.Error(t, err)
}

func TestResolver_SetHouseholdRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleMember}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0]}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), hhUserID, int64(9), identity.HouseholdRoleAdmin, hhEmail).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: "admin"}, nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), int64(9), hhUserID, identity.HouseholdRoleAdmin, hhEmail).
		Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).
		Return([]household.Membership{
			{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleOwner},
			{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleAdmin},
		}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{hhUserID, 9})).
		Return([]identity.User{{UserID: hhUserID}, {UserID: 9}}, nil)

	res, err := r.SetHouseholdRole(hhCtx(), struct {
		UserID graphql.ID
		Role   string
	}{UserID: "9", Role: "ADMIN"})
	require.NoError(t, err)
	assert.Equal(t, "ADMIN", res.Members()[1].Role())
}

func TestResolver_SetHouseholdRole_Guards(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	// OWNER is rejected — use transferHouseholdOwnership.
	_, err := r.SetHouseholdRole(hhCtx(), struct {
		UserID graphql.ID
		Role   string
	}{UserID: "9", Role: "OWNER"})
	assert.ErrorContains(t, err, "transferHouseholdOwnership")

	// Self is rejected.
	_, err = r.SetHouseholdRole(hhCtx(), struct {
		UserID graphql.ID
		Role   string
	}{UserID: "7", Role: "MEMBER"})
	assert.ErrorContains(t, err, "your own role")

	// A stranger gets NOT_FOUND — household membership is never leaked.
	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{}, domainerr.ErrNotFound)
	_, err = r.SetHouseholdRole(hhCtx(), struct {
		UserID graphql.ID
		Role   string
	}{UserID: "9", Role: "ADMIN"})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestResolver_TransferHouseholdOwnership(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleMember}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0]}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), hhUserID, int64(9), identity.HouseholdRoleOwner, hhEmail).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: "owner"}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), hhUserID, hhUserID, identity.HouseholdRoleMember, hhEmail).
		Return(household.Membership{HouseholdID: hhUserID, UserID: hhUserID, Role: "member"}, nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), int64(9), hhUserID, identity.HouseholdRoleOwner, hhEmail).Return(nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), hhUserID, hhUserID, identity.HouseholdRoleMember, hhEmail).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).
		Return([]household.Membership{
			{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleMember},
			{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleOwner},
		}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{hhUserID, 9})).
		Return([]identity.User{{UserID: hhUserID}, {UserID: 9}}, nil)

	res, err := r.TransferHouseholdOwnership(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	require.NoError(t, err)
	assert.Equal(t, "MEMBER", res.MyRole())
}

func TestResolver_RemoveHouseholdMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleMember}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0]}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), int64(9), hhUserID, hhEmail).Return(nil, nil)
	h.EXPECT().RemoveMembership(gomock.Any(), hhUserID, int64(9)).Return(nil)
	// The removed member's active pointer was on this household and no
	// memberships remain → fresh solo household, activated.
	h.EXPECT().ListMembershipsByUser(gomock.Any(), int64(9)).Return(nil, nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), int64(90), int64(9), identity.HouseholdRoleOwner, hhEmail).
		Return(household.Membership{HouseholdID: 90, UserID: 9, Role: "owner"}, nil)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), int64(9), int64(90), hhEmail).Return(nil)
	// member_removed to the target; member_left to the remaining members.
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberRemoved, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).
		Return([]household.Membership{{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil).Times(2)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{hhUserID}).
		Return([]identity.User{{UserID: hhUserID}}, nil)
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)

	res, err := r.RemoveHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	require.NoError(t, err)
	assert.Len(t, res.Members(), 1)
}

func TestResolver_RemoveHouseholdMember_Guards(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	// The owner cannot be removed — the membership role, not the pointer.
	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleOwner}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0]}, nil)
	_, err := r.RemoveHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.Error(t, err)

	// An admin cannot remove another admin.
	adminCtx := testutil.WithHouseholdRole(context.Background(), hhUserID, hhUserID, "admin", hhEmail)
	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleAdmin}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0]}, nil)
	_, err = r.RemoveHouseholdMember(adminCtx, struct{ UserID graphql.ID }{UserID: "9"})
	assert.Error(t, err)

	// A plain member cannot remove anyone.
	memberCtx := testutil.WithHouseholdRole(context.Background(), hhUserID, hhUserID, "member", hhEmail)
	_, err = r.RemoveHouseholdMember(memberCtx, struct{ UserID graphql.ID }{UserID: "9"})
	assert.Error(t, err)
}

func TestResolver_LeaveHousehold_OwnerTransfer(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	currentHH := int64(42)
	ctx := testutil.WithHousehold(context.Background(), hhUserID, currentHH, hhEmail)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &currentHH, HouseholdRole: identity.HouseholdRoleOwner}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), currentHH).
		Return(household.Household{HouseholdID: currentHH}, nil)
	h.EXPECT().GetMembership(gomock.Any(), currentHH, hhUserID).
		Return(household.Membership{HouseholdID: currentHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}, nil)
	// Owner leaves with members remaining: the earliest admin is promoted.
	h.EXPECT().ListMembersByHousehold(gomock.Any(), currentHH).
		Return([]household.Membership{
			{HouseholdID: currentHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner},
			{HouseholdID: currentHH, UserID: 9, Role: identity.HouseholdRoleMember},
			{HouseholdID: currentHH, UserID: 10, Role: identity.HouseholdRoleAdmin},
		}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), currentHH, int64(10), identity.HouseholdRoleOwner, hhEmail).
		Return(household.Membership{HouseholdID: currentHH, UserID: 10, Role: "owner"}, nil)
	// The successor's active pointer is on this household → role syncs.
	idSvc.EXPECT().GetByID(gomock.Any(), int64(10)).
		Return(identity.User{UserID: 10, HouseholdID: &currentHH}, nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), int64(10), currentHH, identity.HouseholdRoleOwner, hhEmail).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(10), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	// A pending invite the leaver sent for this household is cancelled.
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), hhUserID, currentHH, hhEmail).
		Return([]household.Invite{{InviteID: 77, FromUserID: hhUserID, ToUserID: 11, HouseholdID: currentHH}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(11), household.KindInviteCancelled, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().RemoveMembership(gomock.Any(), currentHH, hhUserID).Return(nil)
	h.EXPECT().ListMembershipsByUser(gomock.Any(), hhUserID).Return(nil, nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), int64(90), hhUserID, identity.HouseholdRoleOwner, hhEmail).
		Return(household.Membership{HouseholdID: 90, UserID: hhUserID, Role: "owner"}, nil)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, int64(90), hhEmail).Return(nil)
	// member_left goes to each of the two remaining members.
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberLeft, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(10), household.KindMemberLeft, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(10))

	ok, err := r.LeaveHousehold(ctx, struct{ HouseholdID *graphql.ID }{})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_MyNotifications(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	actor := int64(9)
	// Direct resolver calls bypass the schema default — the limit clamps to 1.
	h.EXPECT().ListNotificationsForUser(gomock.Any(), hhUserID, hhUserID, int32(1)).
		Return([]household.Notification{
			{NotificationID: 5, UserID: hhUserID, Kind: household.KindInviteReceived, ActorUserID: &actor},
			{NotificationID: 4, UserID: hhUserID, Kind: household.KindMemberJoined},
		}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{9}).
		Return([]identity.User{{UserID: 9, DisplayName: "Inviter"}}, nil)

	out, err := r.MyNotifications(hhCtx(), struct{ Limit int32 }{Limit: 0})
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "INVITE_RECEIVED", out[0].Kind())
	assert.Equal(t, "Inviter", *out[0].Actor().DisplayName())
	assert.Equal(t, "MEMBER_JOINED", out[1].Kind())
	assert.Nil(t, out[1].Actor())
}

func TestResolver_NotificationCountAndMarkRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	r := &Resolver{HouseholdService: h}

	h.EXPECT().CountUnreadNotifications(gomock.Any(), hhUserID, hhUserID).Return(int64(3), nil)
	n, err := r.UnreadNotificationCount(hhCtx())
	require.NoError(t, err)
	assert.Equal(t, int32(3), n)

	h.EXPECT().MarkAllNotificationsRead(gomock.Any(), hhUserID, hhUserID).Return(nil)
	ok, err := r.MarkAllNotificationsRead(hhCtx())
	require.NoError(t, err)
	assert.True(t, ok)
}

// ---------- LEN-26: multi-household surface ----------

func TestResolver_MyHouseholds(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	activeHH, otherHH := hhUserID, int64(60)
	h.EXPECT().ListMyHouseholds(gomock.Any(), hhUserID).Return([]household.MyHousehold{
		{Household: household.Household{HouseholdID: activeHH}, Role: identity.HouseholdRoleOwner},
		{Household: household.Household{HouseholdID: otherHH}, Role: identity.HouseholdRoleMember},
	}, nil)
	h.EXPECT().GetHouseholdByID(gomock.Any(), activeHH).
		Return(household.Household{HouseholdID: activeHH}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), activeHH).
		Return([]household.Membership{{HouseholdID: activeHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{hhUserID}).
		Return([]identity.User{{UserID: hhUserID}}, nil)
	h.EXPECT().GetHouseholdByID(gomock.Any(), otherHH).
		Return(household.Household{HouseholdID: otherHH}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), otherHH).
		Return([]household.Membership{
			{HouseholdID: otherHH, UserID: 9, Role: identity.HouseholdRoleOwner},
			{HouseholdID: otherHH, UserID: hhUserID, Role: identity.HouseholdRoleMember},
		}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{9, hhUserID})).
		Return([]identity.User{{UserID: 9}, {UserID: hhUserID}}, nil)

	out, err := r.MyHouseholds(hhCtx())
	require.NoError(t, err)
	require.Len(t, out, 2)
	// The caller's role is per-household, from the membership rows.
	assert.Equal(t, "OWNER", out[0].MyRole())
	assert.True(t, out[0].IsActive())
	assert.Equal(t, "MEMBER", out[1].MyRole())
	assert.False(t, out[1].IsActive())
	// Member role comes from the membership row — the caller's active
	// pointer role (owner) must not leak into the other household.
	assert.Equal(t, "MEMBER", out[1].Members()[1].Role())
	assert.True(t, out[1].Members()[1].IsMe())
}

func TestResolver_SetActiveHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	otherHH := int64(60)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, otherHH, hhEmail).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	h.EXPECT().GetHouseholdByID(gomock.Any(), otherHH).
		Return(household.Household{HouseholdID: otherHH}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), otherHH).
		Return([]household.Membership{{HouseholdID: otherHH, UserID: hhUserID, Role: identity.HouseholdRoleMember}}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{hhUserID}).
		Return([]identity.User{{UserID: hhUserID}}, nil)

	res, err := r.SetActiveHousehold(hhCtx(), struct{ HouseholdID graphql.ID }{HouseholdID: "60"})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("60"), res.ID())
	assert.True(t, res.IsActive())
	assert.Equal(t, "MEMBER", res.MyRole())
}

// A household the caller does not belong to resolves as NOT_FOUND — the
// membership guard in identity maps to conflict internally but must not
// leak household existence to non-members.
func TestResolver_SetActiveHousehold_NonMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, int64(60), hhEmail).
		Return(fmt.Errorf("set active household: %w", domainerr.ErrConflict))

	_, err := r.SetActiveHousehold(hhCtx(), struct{ HouseholdID graphql.ID }{HouseholdID: "60"})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestResolver_CreateHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	name := "  Beach House  "
	newHH := int64(77)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: newHH}, nil)
	h.EXPECT().JoinHousehold(gomock.Any(), newHH, hhUserID, identity.HouseholdRoleOwner, hhEmail).
		Return(household.Membership{HouseholdID: newHH, UserID: hhUserID, Role: "owner"}, nil)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, newHH, hhEmail).Return(nil)
	h.EXPECT().RenameHousehold(gomock.Any(), newHH, "Beach House", hhEmail).
		Return(household.Household{HouseholdID: newHH, Name: &name}, nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	h.EXPECT().GetHouseholdByID(gomock.Any(), newHH).
		Return(household.Household{HouseholdID: newHH, Name: &name}, nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), newHH).
		Return([]household.Membership{{HouseholdID: newHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{hhUserID}).
		Return([]identity.User{{UserID: hhUserID}}, nil)

	res, err := r.CreateHousehold(hhCtx(), struct{ Name *string }{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("77"), res.ID())
	assert.Equal(t, name, *res.Name())
	assert.True(t, res.IsActive())
	assert.Equal(t, "OWNER", res.MyRole())
}

// Accepting with mergeFromHouseholdId moves the sole-owned household's
// data into the joined household and dissolves the source.
func TestResolver_AcceptHouseholdInvite_WithMerge(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	prefs := mock.NewMockUserPrefsService(ctrl)
	mp := mock.NewMockMealPlanService(ctrl)
	groc := mock.NewMockGroceryService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{
		HouseholdService: h, IdentityService: idSvc, UserPrefsService: prefs,
		MealPlanService: mp, GroceryService: groc, AuthInvalidator: auth,
	}

	inviterHH, sourceHH := int64(42), int64(70)
	inv := household.Invite{InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending}
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(inv, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	h.EXPECT().CountMembers(gomock.Any(), inviterHH).Return(int64(1), nil)
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, int64(9)).
		Return(household.Membership{HouseholdID: inviterHH, UserID: 9, Role: "owner"}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusAccepted, hhEmail).
		Return(household.Invite{InviteID: 55, Status: household.StatusAccepted}, nil)
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, hhUserID).
		Return(household.Membership{}, domainerr.ErrNotFound)
	h.EXPECT().JoinHousehold(gomock.Any(), inviterHH, hhUserID, identity.HouseholdRoleMember, hhEmail).
		Return(household.Membership{HouseholdID: inviterHH, UserID: hhUserID, Role: "member"}, nil)
	// Merge guard: caller is the source's sole member AND owner.
	h.EXPECT().GetMembership(gomock.Any(), sourceHH, hhUserID).
		Return(household.Membership{HouseholdID: sourceHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}, nil)
	h.EXPECT().CountMembers(gomock.Any(), sourceHH).Return(int64(1), nil)
	prefs.EXPECT().MergeHouseholdStock(gomock.Any(), sourceHH, inviterHH, hhEmail).Return(nil)
	mp.EXPECT().ReassignHousehold(gomock.Any(), sourceHH, inviterHH, hhEmail).Return(nil)
	groc.EXPECT().ReassignHousehold(gomock.Any(), sourceHH, inviterHH, hhEmail).Return(nil)
	idSvc.EXPECT().SetActiveHousehold(gomock.Any(), hhUserID, inviterHH, hhEmail).Return(nil)
	h.EXPECT().DeleteHousehold(gomock.Any(), sourceHH).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteAccepted, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), inviterHH).
		Return([]household.Membership{
			{HouseholdID: inviterHH, UserID: 9, Role: "owner"},
			{HouseholdID: inviterHH, UserID: hhUserID, Role: "member"},
		}, nil).Times(2)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), gomock.InAnyOrder([]int64{9, hhUserID})).
		Return([]identity.User{{UserID: 9}, {UserID: hhUserID}}, nil)

	mergeFrom := graphql.ID("70")
	res, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55", MergeFromHouseholdID: &mergeFrom})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("42"), res.ID())
}

// Merge is rejected when the source household is shared — the other
// members' data must never be dissolved by someone else's accept.
func TestResolver_AcceptHouseholdInvite_MergeSharedSourceRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	inviterHH, sourceHH := int64(42), int64(70)
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending,
	}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	h.EXPECT().CountMembers(gomock.Any(), inviterHH).Return(int64(1), nil)
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, int64(9)).
		Return(household.Membership{HouseholdID: inviterHH, UserID: 9, Role: "owner"}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusAccepted, hhEmail).
		Return(household.Invite{InviteID: 55, Status: household.StatusAccepted}, nil)
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, hhUserID).
		Return(household.Membership{}, domainerr.ErrNotFound)
	h.EXPECT().JoinHousehold(gomock.Any(), inviterHH, hhUserID, identity.HouseholdRoleMember, hhEmail).
		Return(household.Membership{HouseholdID: inviterHH, UserID: hhUserID, Role: "member"}, nil)
	h.EXPECT().GetMembership(gomock.Any(), sourceHH, hhUserID).
		Return(household.Membership{HouseholdID: sourceHH, UserID: hhUserID, Role: identity.HouseholdRoleOwner}, nil)
	// Shared household — the merge must be rejected and the tx rolled
	// back (invite transition included).
	h.EXPECT().CountMembers(gomock.Any(), sourceHH).Return(int64(3), nil)

	mergeFrom := graphql.ID("70")
	_, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55", MergeFromHouseholdID: &mergeFrom})
	assert.ErrorContains(t, err, "alone belong to")
}

// Merge is also rejected when the caller isn't a member of the source —
// NOT_FOUND, never a leak.
func TestResolver_AcceptHouseholdInvite_MergeStrangerSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	inviterHH, sourceHH := int64(42), int64(70)
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending,
	}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	h.EXPECT().CountMembers(gomock.Any(), inviterHH).Return(int64(1), nil)
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, int64(9)).
		Return(household.Membership{HouseholdID: inviterHH, UserID: 9, Role: "owner"}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusAccepted, hhEmail).
		Return(household.Invite{InviteID: 55, Status: household.StatusAccepted}, nil)
	h.EXPECT().GetMembership(gomock.Any(), inviterHH, hhUserID).
		Return(household.Membership{}, domainerr.ErrNotFound)
	h.EXPECT().JoinHousehold(gomock.Any(), inviterHH, hhUserID, identity.HouseholdRoleMember, hhEmail).
		Return(household.Membership{HouseholdID: inviterHH, UserID: hhUserID, Role: "member"}, nil)
	h.EXPECT().GetMembership(gomock.Any(), sourceHH, hhUserID).
		Return(household.Membership{}, domainerr.ErrNotFound)

	mergeFrom := graphql.ID("70")
	_, err := r.AcceptHouseholdInvite(hhCtx(), struct {
		InviteID             graphql.ID
		MergeFromHouseholdID *graphql.ID
	}{InviteID: "55", MergeFromHouseholdID: &mergeFrom})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

// Removing a member whose active pointer is elsewhere drops only the
// membership — no fallback activation runs.
func TestResolver_RemoveHouseholdMember_PointerElsewhere(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	auth := mock.NewMockAuthInvalidator(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc, AuthInvalidator: auth}

	otherHH := int64(60)
	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	h.EXPECT().GetMembership(gomock.Any(), hhUserID, int64(9)).
		Return(household.Membership{HouseholdID: hhUserID, UserID: 9, Role: identity.HouseholdRoleMember}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &otherHH}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), int64(9), hhUserID, hhEmail).Return(nil, nil)
	h.EXPECT().RemoveMembership(gomock.Any(), hhUserID, int64(9)).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberRemoved, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().ListMembersByHousehold(gomock.Any(), hhUserID).
		Return([]household.Membership{{HouseholdID: hhUserID, UserID: hhUserID, Role: identity.HouseholdRoleOwner}}, nil).Times(2)
	idSvc.EXPECT().ListUsersByIDs(gomock.Any(), []int64{hhUserID}).
		Return([]identity.User{{UserID: hhUserID}}, nil)
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)

	res, err := r.RemoveHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	require.NoError(t, err)
	assert.Len(t, res.Members(), 1)
}
