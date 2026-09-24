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
		{UserID: hhUserID, Email: hhEmail, DisplayName: "Caller"},
		{UserID: 9, Email: "mate@example.com", DisplayName: "Mate"},
	}, nil)

	res, err := r.MyHousehold(hhCtx())
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("7"), res.ID())
	require.Len(t, res.Members(), 2)
	assert.Equal(t, "Caller", *res.Members()[0].DisplayName())
	// Restricted projection carries no email/role surface.
	assert.Equal(t, graphql.ID("9"), res.Members()[1].ID())
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
	h.EXPECT().CreateInvite(gomock.Any(), hhUserID, int64(9), hhUserID, hhEmail).
		Return(household.Invite{InviteID: 30, FromUserID: hhUserID, ToUserID: 9, HouseholdID: hhUserID, Status: household.StatusPending}, nil)
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
	idSvc.EXPECT().GetByID(gomock.Any(), int64(9)).
		Return(identity.User{UserID: 9, HouseholdID: &inviterHH}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &targetHH}, nil)
	h.EXPECT().TransitionInvite(gomock.Any(), int64(55), household.StatusAccepted, hhEmail).
		Return(household.Invite{InviteID: 55, Status: household.StatusAccepted}, nil)
	prefs.EXPECT().MergeHouseholdStock(gomock.Any(), targetHH, inviterHH, hhEmail).Return(nil)
	mp.EXPECT().ReassignHousehold(gomock.Any(), targetHH, inviterHH, hhEmail).Return(nil)
	groc.EXPECT().ReassignHousehold(gomock.Any(), targetHH, inviterHH, hhEmail).Return(nil)
	idSvc.EXPECT().SetUserHousehold(gomock.Any(), hhUserID, inviterHH, &targetHH).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")
	auth.EXPECT().InvalidateUserID(gomock.Any(), int64(9))
	h.EXPECT().GetHouseholdByID(gomock.Any(), inviterHH).
		Return(household.Household{HouseholdID: inviterHH}, nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), inviterHH).
		Return([]identity.User{{UserID: 9}, {UserID: hhUserID}}, nil)

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
	idSvc.EXPECT().GetByID(gomock.Any(), hhUserID).
		Return(identity.User{UserID: hhUserID, HouseholdID: &currentHH}, nil)
	h.EXPECT().CreateHousehold(gomock.Any(), hhEmail).
		Return(household.Household{HouseholdID: 90}, nil)
	idSvc.EXPECT().SetUserHousehold(gomock.Any(), hhUserID, int64(90), &currentHH).Return(nil)
	auth.EXPECT().InvalidateUser("test-provider", "")

	ok, err := r.LeaveHousehold(hhCtx())
	require.NoError(t, err)
	assert.True(t, ok)
}
