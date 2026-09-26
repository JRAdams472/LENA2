package bff

import (
	"context"
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
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), hhUserID).Return([]identity.User{
		{UserID: hhUserID, Email: hhEmail, DisplayName: "Caller", HouseholdRole: identity.HouseholdRoleOwner},
		{UserID: 9, Email: "mate@example.com", DisplayName: "Mate", HouseholdRole: identity.HouseholdRoleMember},
	}, nil)

	res, err := r.MyHousehold(hhCtx())
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("7"), res.ID())
	assert.Equal(t, "OWNER", res.MyRole())
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
	h.EXPECT().ListSentInvitesForUser(gomock.Any(), hhUserID).Return([]household.Invite{
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
		Return(identity.User{UserID: 9, HouseholdID: &otherHH}, nil)
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

	// Already in the caller's household.
	sameHH := hhUserID
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &sameHH}, nil)
	_, err = r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.ErrorContains(t, err, "already in your household")
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
	targetHH := hhUserID
	inv := household.Invite{InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending}

	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(inv, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), inviterHH).Return(int64(1), nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &inviterHH}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &targetHH}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusAccepted, hhEmail).
		Return(household.Invite{InviteID: 55, Status: household.StatusAccepted}, nil)
	prefs.EXPECT().MergeHouseholdStock(gomock.Any(), targetHH, inviterHH, hhEmail).Return(nil)
	mp.EXPECT().ReassignHousehold(gomock.Any(), targetHH, inviterHH, hhEmail).Return(nil)
	groc.EXPECT().ReassignHousehold(gomock.Any(), targetHH, inviterHH, hhEmail).Return(nil)
	idSvc.EXPECT().SetUserHousehold(gomock.Any(), hhUserID, inviterHH, identity.HouseholdRoleMember, &targetHH).Return(nil)
	// invite_accepted to the inviter; member_joined to other members.
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindInviteAccepted, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	// Once inside acceptInvite for member_joined fan-out, once to build
	// the returned Household.
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), inviterHH).
		Return([]identity.User{{UserID: 9}, {UserID: hhUserID}}, nil).Times(2)

	res, err := r.AcceptHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "55"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("42"), res.ID())
	assert.Len(t, res.Members(), 2)
}

func TestResolver_AcceptHouseholdInvite_NotParty(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	r := &Resolver{HouseholdService: h}

	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: 99, Status: household.StatusPending,
	}, nil)

	_, err := r.AcceptHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "55"})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestResolver_AcceptHouseholdInvite_StaleInviter(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{HouseholdService: h, IdentityService: idSvc}

	inviterHH := int64(42)
	movedHH := int64(77)
	h.EXPECT().GetInviteByID(gomock.Any(), int64(55)).Return(household.Invite{
		InviteID: 55, FromUserID: 9, ToUserID: hhUserID, HouseholdID: inviterHH, Status: household.StatusPending,
	}, nil)
	h.EXPECT().LockHousehold(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), inviterHH).Return(int64(1), nil)
	// The inviter left the household the invite points at — accept conflicts.
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &movedHH}, nil)

	_, err := r.AcceptHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "55"})
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
	h.EXPECT().LockHousehold(gomock.Any(), currentHH).
		Return(household.Household{HouseholdID: currentHH}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &currentHH, HouseholdRole: identity.HouseholdRoleOwner}, nil)
	// Sole member leaving: no ownership transfer.
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), currentHH).
		Return([]identity.User{{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleOwner}}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), hhUserID, currentHH, hhEmail).
		Return(nil, nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	idSvc.EXPECT().SetUserHousehold(gomock.Any(), hhUserID, int64(90), identity.HouseholdRoleOwner, &currentHH).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")

	ok, err := r.LeaveHousehold(hhCtx())
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
		Return(identity.User{UserID: 9, HouseholdID: &otherHH}, nil)
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
		Return(identity.User{UserID: 9, HouseholdID: &otherHH}, nil)
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), hhUserID).
		Return(int64(maxHouseholdMembers), nil)

	_, err := r.InviteHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.ErrorContains(t, err, "household is full")
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
	idSvc.EXPECT().CountUsersByHousehold(gomock.Any(), inviterHH).
		Return(int64(maxHouseholdMembers), nil)

	_, err := r.AcceptHouseholdInvite(hhCtx(), struct{ InviteID graphql.ID }{InviteID: "55"})
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
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), hhUserID).
		Return([]identity.User{
			{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleOwner},
			{UserID: 9, HouseholdRole: identity.HouseholdRoleMember},
		}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindHouseholdRenamed, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID, Name: &name}, nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), hhUserID).
		Return([]identity.User{{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleOwner}}, nil)

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
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0], HouseholdRole: identity.HouseholdRoleMember}, nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), int64(9), hhUserID, identity.HouseholdRoleAdmin, hhEmail).
		Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), hhUserID).
		Return([]identity.User{
			{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleOwner},
			{UserID: 9, HouseholdRole: identity.HouseholdRoleAdmin},
		}, nil)

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
	strangerHH := int64(50)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &strangerHH}, nil)
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
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0], HouseholdRole: identity.HouseholdRoleMember}, nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), int64(9), hhUserID, identity.HouseholdRoleOwner, hhEmail).Return(nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), hhUserID, hhUserID, identity.HouseholdRoleMember, hhEmail).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), hhUserID).
		Return([]identity.User{
			{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleMember},
			{UserID: 9, HouseholdRole: identity.HouseholdRoleOwner},
		}, nil)

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
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0], HouseholdRole: identity.HouseholdRoleMember}, nil)
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), int64(9), hhUserID, hhEmail).Return(nil, nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	expectedHH := int64(hhUserID)
	idSvc.EXPECT().SetUserHousehold(gomock.Any(), int64(9), int64(90), identity.HouseholdRoleOwner, &expectedHH).Return(nil)
	// member_removed to the target; member_left to the remaining members.
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberRemoved, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), hhUserID).
		Return([]identity.User{{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleOwner}}, nil).Times(2)
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

	// The owner cannot be removed.
	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0], HouseholdRole: identity.HouseholdRoleOwner}, nil)
	_, err := r.RemoveHouseholdMember(hhCtx(), struct{ UserID graphql.ID }{UserID: "9"})
	assert.Error(t, err)

	// An admin cannot remove another admin.
	adminCtx := testutil.WithHouseholdRole(context.Background(), hhUserID, hhUserID, "admin", hhEmail)
	h.EXPECT().LockHousehold(gomock.Any(), hhUserID).
		Return(household.Household{HouseholdID: hhUserID}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &[]int64{hhUserID}[0], HouseholdRole: identity.HouseholdRoleAdmin}, nil)
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
	// Owner leaves with members remaining: the earliest admin is promoted.
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), currentHH).
		Return([]identity.User{
			{UserID: hhUserID, HouseholdRole: identity.HouseholdRoleOwner},
			{UserID: 9, HouseholdRole: identity.HouseholdRoleMember},
			{UserID: 10, HouseholdRole: identity.HouseholdRoleAdmin},
		}, nil)
	idSvc.EXPECT().SetUserHouseholdRole(gomock.Any(), int64(10), currentHH, identity.HouseholdRoleOwner, hhEmail).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(10), household.KindRoleChanged, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	// A pending invite the leaver sent for this household is cancelled.
	h.EXPECT().CancelPendingInvitesFrom(gomock.Any(), hhUserID, currentHH, hhEmail).
		Return([]household.Invite{{InviteID: 77, FromUserID: hhUserID, ToUserID: 11, HouseholdID: currentHH}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(11), household.KindInviteCancelled, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	idSvc.EXPECT().SetUserHousehold(gomock.Any(), hhUserID, int64(90), identity.HouseholdRoleOwner, &currentHH).Return(nil)
	// member_left goes to each of the two remaining members.
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindMemberLeft, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(10), household.KindMemberLeft, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(10))

	ok, err := r.LeaveHousehold(ctx)
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
	h.EXPECT().ListNotificationsForUser(gomock.Any(), hhUserID, int32(1)).
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

	h.EXPECT().CountUnreadNotifications(gomock.Any(), hhUserID).Return(int64(3), nil)
	n, err := r.UnreadNotificationCount(hhCtx())
	require.NoError(t, err)
	assert.Equal(t, int32(3), n)

	h.EXPECT().MarkAllNotificationsRead(gomock.Any(), hhUserID).Return(nil)
	ok, err := r.MarkAllNotificationsRead(hhCtx())
	require.NoError(t, err)
	assert.True(t, ok)
}
