package household

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/household/sqlc"
	"github.com/JRAdams472/LENA2/internal/household/sqlc/mock"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

var errDB = errors.New("db error")

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq}, mq
}

func TestCreateHousehold(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateHousehold(ctx, "a@b.com").Return(
			sqlc.HouseholdHousehold{HouseholdID: 7, CreatedBy: "a@b.com"}, nil)

		got, err := svc.CreateHousehold(ctx, "a@b.com")
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.HouseholdID)
		assert.Equal(t, "a@b.com", got.CreatedBy)
	})

	t.Run("storage error wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateHousehold(ctx, gomock.Any()).Return(sqlc.HouseholdHousehold{}, errDB)
		_, err := svc.CreateHousehold(ctx, "a@b.com")
		require.ErrorIs(t, err, errDB)
	})
}

func TestGetHouseholdByID(t *testing.T) {
	ctx := context.Background()

	svc, mq := newService(t)
	mq.EXPECT().GetHouseholdByID(ctx, int64(9)).Return(sqlc.HouseholdHousehold{}, pgx.ErrNoRows)
	_, err := svc.GetHouseholdByID(ctx, 9)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestCreateInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateInvite(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.CreateInviteParams) (sqlc.HouseholdInvite, error) {
				assert.Equal(t, int64(1), arg.FromUserID)
				assert.Equal(t, int64(2), arg.ToUserID)
				assert.Equal(t, int64(11), arg.HouseholdID)
				return sqlc.HouseholdInvite{
					InviteID: 5, FromUserID: arg.FromUserID, ToUserID: arg.ToUserID,
					HouseholdID: arg.HouseholdID, Status: "pending",
				}, nil
			})

		got, err := svc.CreateInvite(ctx, 1, 2, 11, "a@b.com")
		require.NoError(t, err)
		assert.Equal(t, StatusPending, got.Status)
	})

	t.Run("duplicate pending invite is a conflict", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateInvite(ctx, gomock.Any()).Return(sqlc.HouseholdInvite{},
			&pgconn.PgError{Code: "23505", ConstraintName: "idx_invites_pending_unique"})
		_, err := svc.CreateInvite(ctx, 1, 2, 11, "a@b.com")
		assert.ErrorIs(t, err, domainerr.ErrConflict)
	})
}

func TestGetInviteByID(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().GetInviteByID(gomock.Any(), int64(3)).Return(sqlc.HouseholdInvite{}, pgx.ErrNoRows)
	_, err := svc.GetInviteByID(context.Background(), 3)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestListInvites(t *testing.T) {
	ctx := context.Background()

	svc, mq := newService(t)
	mq.EXPECT().ListPendingInvitesForUser(ctx, int64(2)).Return([]sqlc.HouseholdInvite{
		{InviteID: 1, FromUserID: 9, ToUserID: 2, Status: "pending"},
	}, nil)
	mq.EXPECT().ListSentInvitesForUser(ctx, int64(9)).Return([]sqlc.HouseholdInvite{
		{InviteID: 1, FromUserID: 9, ToUserID: 2, Status: "pending"},
	}, nil)

	pending, err := svc.ListPendingInvitesForUser(ctx, 2)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(9), pending[0].FromUserID)

	sent, err := svc.ListSentInvitesForUser(ctx, 9)
	require.NoError(t, err)
	require.Len(t, sent, 1)
	assert.Equal(t, int64(2), sent[0].ToUserID)
}

func TestTransitionInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("pending to accepted", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().TransitionInvite(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.TransitionInviteParams) (sqlc.HouseholdInvite, error) {
				assert.Equal(t, int64(4), arg.InviteID)
				assert.Equal(t, "accepted", arg.Status)
				return sqlc.HouseholdInvite{InviteID: 4, Status: "accepted"}, nil
			})
		got, err := svc.TransitionInvite(ctx, 4, StatusAccepted, "b@b.com")
		require.NoError(t, err)
		assert.Equal(t, StatusAccepted, got.Status)
	})

	t.Run("non-pending invite is a conflict", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().TransitionInvite(ctx, gomock.Any()).Return(sqlc.HouseholdInvite{}, pgx.ErrNoRows)
		_, err := svc.TransitionInvite(ctx, 4, StatusAccepted, "b@b.com")
		assert.ErrorIs(t, err, domainerr.ErrConflict)
	})

	t.Run("pending is not a valid target", func(t *testing.T) {
		svc, _ := newService(t)
		_, err := svc.TransitionInvite(ctx, 4, StatusPending, "b@b.com")
		assert.ErrorIs(t, err, domainerr.ErrValidation)
	})
}
