package household

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
	mq.EXPECT().ListSentInvitesForUser(ctx, sqlc.ListSentInvitesForUserParams{
		FromUserID:  9,
		HouseholdID: 8,
	}).Return([]sqlc.HouseholdInvite{
		{InviteID: 1, FromUserID: 9, ToUserID: 2, Status: "pending"},
	}, nil)

	pending, err := svc.ListPendingInvitesForUser(ctx, 2)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(9), pending[0].FromUserID)

	sent, err := svc.ListSentInvitesForUser(ctx, 9, 8)
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

func TestRenameHousehold(t *testing.T) {
	ctx := context.Background()

	t.Run("trims and renames", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().RenameHousehold(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.RenameHouseholdParams) (sqlc.HouseholdHousehold, error) {
				assert.Equal(t, int64(7), arg.HouseholdID)
				assert.Equal(t, "Casa", arg.Name.String)
				assert.True(t, arg.Name.Valid)
				return sqlc.HouseholdHousehold{HouseholdID: 7, Name: arg.Name}, nil
			})
		got, err := svc.RenameHousehold(ctx, 7, "  Casa  ", "o@o.com")
		require.NoError(t, err)
		require.NotNil(t, got.Name)
		assert.Equal(t, "Casa", *got.Name)
	})

	t.Run("blank name clears", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().RenameHousehold(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.RenameHouseholdParams) (sqlc.HouseholdHousehold, error) {
				assert.False(t, arg.Name.Valid)
				return sqlc.HouseholdHousehold{HouseholdID: 7}, nil
			})
		got, err := svc.RenameHousehold(ctx, 7, "   ", "o@o.com")
		require.NoError(t, err)
		assert.Nil(t, got.Name)
	})

	t.Run("missing household is not found", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().RenameHousehold(ctx, gomock.Any()).Return(sqlc.HouseholdHousehold{}, pgx.ErrNoRows)
		_, err := svc.RenameHousehold(ctx, 7, "x", "o@o.com")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})
}

func TestLockHousehold(t *testing.T) {
	ctx := context.Background()

	svc, mq := newService(t)
	mq.EXPECT().GetHouseholdByIDForUpdate(ctx, int64(7)).Return(
		sqlc.HouseholdHousehold{HouseholdID: 7, Name: pgtype.Text{String: "H", Valid: true}}, nil)
	got, err := svc.LockHousehold(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.HouseholdID)
	require.NotNil(t, got.Name)
	assert.Equal(t, "H", *got.Name)
}

func TestCreateNotification(t *testing.T) {
	ctx := context.Background()

	svc, mq := newService(t)
	hh, actor, inv := int64(8), int64(2), int64(5)
	mq.EXPECT().CreateNotification(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, arg sqlc.CreateNotificationParams) (sqlc.HouseholdNotification, error) {
			assert.Equal(t, int64(3), arg.UserID)
			assert.Equal(t, hh, arg.HouseholdID.Int64)
			assert.Equal(t, "member_joined", arg.Kind)
			assert.Equal(t, actor, arg.ActorUserID.Int64)
			assert.Equal(t, inv, arg.InviteID.Int64)
			return sqlc.HouseholdNotification{}, nil
		})
	mq.EXPECT().PruneReadNotifications(ctx, int64(3)).Return(nil)
	err := svc.CreateNotification(ctx, 3, KindMemberJoined, &hh, &actor, &inv, nil)
	require.NoError(t, err)
}

type stubGate struct {
	allowed     bool
	pushAllowed bool
	err         error
	pushErr     error
	called      bool
}

func (g *stubGate) Allowed(_ context.Context, _ int64, _ string) (bool, error) {
	g.called = true
	return g.allowed, g.err
}

func (g *stubGate) PushAllowed(_ context.Context, _ int64, _ string) (bool, error) {
	return g.pushAllowed, g.pushErr
}

func TestCreateNotification_GateSuppressed(t *testing.T) {
	svc, _ := newService(t)
	gate := &stubGate{allowed: false}
	svc.WithNotifyGate(gate)

	// Suppressed recipients get nothing — no insert, no prune.
	require.NoError(t, svc.CreateNotification(context.Background(), 3, KindMemberJoined, nil, nil, nil, nil))
	assert.True(t, gate.called)
}

func TestCreateNotification_GateErrorDelivers(t *testing.T) {
	ctx := context.Background()
	svc, mq := newService(t)
	svc.WithNotifyGate(&stubGate{allowed: false, err: errDB})
	mq.EXPECT().CreateNotification(ctx, gomock.Any()).Return(sqlc.HouseholdNotification{}, nil)
	mq.EXPECT().PruneReadNotifications(ctx, int64(3)).Return(nil)

	// A gate failure fails open so a prefs outage can't eat notifications.
	require.NoError(t, svc.CreateNotification(ctx, 3, KindMemberJoined, nil, nil, nil, nil))
}

func TestCreateNotification_PushOnlySkipsFeed(t *testing.T) {
	ctx := context.Background()
	svc, mq := newService(t)
	// Feed suppressed, push opted in: outbox row written, no feed insert.
	svc.WithNotifyGate(&stubGate{allowed: false, pushAllowed: true})
	mq.EXPECT().InsertPushDelivery(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, arg sqlc.InsertPushDeliveryParams) (pgconn.CommandTag, error) {
			assert.Equal(t, int64(3), arg.UserID)
			assert.Equal(t, string(KindMemberJoined), arg.Kind)
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		})
	require.NoError(t, svc.CreateNotification(ctx, 3, KindMemberJoined, nil, nil, nil, nil))
}

func TestCreateNotification_BothChannels(t *testing.T) {
	ctx := context.Background()
	svc, mq := newService(t)
	svc.WithNotifyGate(&stubGate{allowed: true, pushAllowed: true})
	mq.EXPECT().CreateNotification(ctx, gomock.Any()).Return(sqlc.HouseholdNotification{}, nil)
	mq.EXPECT().PruneReadNotifications(ctx, int64(3)).Return(nil)
	mq.EXPECT().InsertPushDelivery(ctx, gomock.Any()).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)
	require.NoError(t, svc.CreateNotification(ctx, 3, KindMemberJoined, nil, nil, nil, nil))
}

func TestCreateNotification_PushGateErrorFailsClosed(t *testing.T) {
	ctx := context.Background()
	svc, mq := newService(t)
	// Feed gate fine, push gate errors: feed row still lands, no push —
	// an errant buzz is worse than a skipped one.
	svc.WithNotifyGate(&stubGate{allowed: true, pushErr: errDB})
	mq.EXPECT().CreateNotification(ctx, gomock.Any()).Return(sqlc.HouseholdNotification{}, nil)
	mq.EXPECT().PruneReadNotifications(ctx, int64(3)).Return(nil)
	require.NoError(t, svc.CreateNotification(ctx, 3, KindMemberJoined, nil, nil, nil, nil))
}

func TestCreateNotification_BothChannelsSuppressed(t *testing.T) {
	svc, _ := newService(t)
	svc.WithNotifyGate(&stubGate{allowed: false, pushAllowed: false})
	// Nothing written on either channel.
	require.NoError(t, svc.CreateNotification(context.Background(), 3, KindMemberJoined, nil, nil, nil, nil))
}

func TestListNotificationsForUser(t *testing.T) {
	ctx := context.Background()

	svc, mq := newService(t)
	now := time.Now()
	mq.EXPECT().ListNotificationsForUser(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, arg sqlc.ListNotificationsForUserParams) ([]sqlc.HouseholdNotification, error) {
			assert.Equal(t, int64(4), arg.UserID)
			assert.Equal(t, pgtype.Int8{Int64: 8, Valid: true}, arg.HouseholdID)
			assert.Equal(t, int32(10), arg.Limit)
			return []sqlc.HouseholdNotification{{
				NotificationID: 9,
				UserID:         4,
				Kind:           "invite_received",
				ReadAt:         pgtype.Timestamptz{Time: now, Valid: true},
				CreatedAt:      now,
			}}, nil
		})
	got, err := svc.ListNotificationsForUser(ctx, 4, 8, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(9), got[0].NotificationID)
	assert.Equal(t, KindInviteReceived, got[0].Kind)
	require.NotNil(t, got[0].ReadAt)
}

func TestNotificationCountsAndRead(t *testing.T) {
	ctx := context.Background()

	t.Run("unread count", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountUnreadNotifications(ctx, sqlc.CountUnreadNotificationsParams{
			UserID:      4,
			HouseholdID: pgtype.Int8{Int64: 8, Valid: true},
		}).Return(int64(3), nil)
		n, err := svc.CountUnreadNotifications(ctx, 4, 8)
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
	})

	t.Run("mark all read", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().MarkAllNotificationsRead(ctx, sqlc.MarkAllNotificationsReadParams{
			UserID:      4,
			HouseholdID: pgtype.Int8{Int64: 8, Valid: true},
		}).Return(int64(2), nil)
		require.NoError(t, svc.MarkAllNotificationsRead(ctx, 4, 8))
	})
}

func TestCancelPendingInvitesFrom(t *testing.T) {
	ctx := context.Background()

	svc, mq := newService(t)
	mq.EXPECT().CancelPendingInvitesFrom(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, arg sqlc.CancelPendingInvitesFromParams) ([]sqlc.HouseholdInvite, error) {
			assert.Equal(t, int64(4), arg.FromUserID)
			assert.Equal(t, int64(8), arg.HouseholdID)
			return []sqlc.HouseholdInvite{{InviteID: 12, ToUserID: 9}}, nil
		})
	got, err := svc.CancelPendingInvitesFrom(ctx, 4, 8, "a@a.com")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(12), got[0].InviteID)
}

func TestMembershipQueries(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	row := sqlc.HouseholdHouseholdMember{
		HouseholdID: 8, UserID: 4, Role: "owner",
		CreatedBy: "a@a.com", CreatedAt: now,
		UpdatedBy: pgtype.Text{String: "b@b.com", Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}

	t.Run("get membership maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetMembership(ctx, sqlc.GetMembershipParams{HouseholdID: 8, UserID: 4}).
			Return(row, nil)
		m, err := svc.GetMembership(ctx, 8, 4)
		require.NoError(t, err)
		assert.Equal(t, int64(8), m.HouseholdID)
		assert.Equal(t, "owner", m.Role)
		require.NotNil(t, m.UpdatedBy)
		assert.Equal(t, "b@b.com", *m.UpdatedBy)
	})

	t.Run("get membership missing is not found", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetMembership(ctx, gomock.Any()).Return(sqlc.HouseholdHouseholdMember{}, pgx.ErrNoRows)
		_, err := svc.GetMembership(ctx, 8, 4)
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("list members and memberships", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListMembersByHousehold(ctx, int64(8)).Return([]sqlc.HouseholdHouseholdMember{row}, nil)
		mq.EXPECT().ListMembershipsByUser(ctx, int64(4)).Return([]sqlc.HouseholdHouseholdMember{row, {HouseholdID: 9, UserID: 4, Role: "member"}}, nil)

		members, err := svc.ListMembersByHousehold(ctx, 8)
		require.NoError(t, err)
		require.Len(t, members, 1)

		mine, err := svc.ListMembershipsByUser(ctx, 4)
		require.NoError(t, err)
		require.Len(t, mine, 2)
		assert.Equal(t, "member", mine[1].Role)
	})

	t.Run("list my households joins role", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListHouseholdsByUser(ctx, int64(4)).Return([]sqlc.ListHouseholdsByUserRow{{
			HouseholdID: 8, Name: pgtype.Text{String: "Home", Valid: true},
			CreatedBy: "a@a.com", CreatedAt: now, MemberRole: "owner",
		}}, nil)
		got, err := svc.ListMyHouseholds(ctx, 4)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, int64(8), got[0].Household.HouseholdID)
		require.NotNil(t, got[0].Household.Name)
		assert.Equal(t, "Home", *got[0].Household.Name)
		assert.Equal(t, "owner", got[0].Role)
	})

	t.Run("count members", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountMembers(ctx, int64(8)).Return(int64(3), nil)
		n, err := svc.CountMembers(ctx, 8)
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
	})
}

func TestJoinAndRemoveMembership(t *testing.T) {
	ctx := context.Background()

	t.Run("join upserts", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertMembership(ctx, sqlc.UpsertMembershipParams{
			HouseholdID: 8, UserID: 4, Role: "member", CreatedBy: "a@a.com",
		}).Return(sqlc.HouseholdHouseholdMember{
			HouseholdID: 8, UserID: 4, Role: "member", CreatedBy: "a@a.com", CreatedAt: time.Now(),
		}, nil)
		m, err := svc.JoinHousehold(ctx, 8, 4, "member", "a@a.com")
		require.NoError(t, err)
		assert.Equal(t, "member", m.Role)
	})

	t.Run("remove deletes the row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteMembership(ctx, sqlc.DeleteMembershipParams{HouseholdID: 8, UserID: 4}).Return(int64(1), nil)
		require.NoError(t, svc.RemoveMembership(ctx, 8, 4))
	})

	t.Run("remove non-member is not found", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteMembership(ctx, gomock.Any()).Return(int64(0), nil)
		assert.ErrorIs(t, svc.RemoveMembership(ctx, 8, 4), domainerr.ErrNotFound)
	})
}
