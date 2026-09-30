package session

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

	"github.com/JRAdams472/LENA2/internal/session/sqlc"
	"github.com/JRAdams472/LENA2/internal/session/sqlc/mock"
)

var errDB = errors.New("db error")

func liveRow(id, family, userID int64, ttl time.Duration) sqlc.IdentitySession {
	return sqlc.IdentitySession{
		SessionID: id,
		UserID:    userID,
		FamilyID:  family,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// newService returns a service whose plain and tx-bound queries both land
// on the mock; the stub pool supplies a no-op transaction for InTx.
func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	s := &Service{
		q:          mq,
		pool:       &stubPool{tx: &stubTx{}},
		secret:     []byte("test-secret"),
		accessTTL:  15 * time.Minute,
		refreshTTL: 30 * 24 * time.Hour,
		now:        time.Now,
	}
	s.newQ = func(pgx.Tx) sqlc.Querier { return mq }
	return s, mq
}

func TestIssue(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().CreateSession(gomock.Any(), gomock.Any()).
		Return(liveRow(1, 1, 7, time.Hour), nil)

	out, err := s.Issue(context.Background(), 7, "web")
	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)

	// The issued access token validates back to the same user.
	uid, err := s.ValidateAccess(out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(7), uid)
}

func TestIssue_Disabled(t *testing.T) {
	s := &Service{now: time.Now}
	_, err := s.Issue(context.Background(), 7, "web")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestRefresh_Rotates(t *testing.T) {
	s, mq := newService(t)
	old := liveRow(1, 1, 7, time.Hour)
	next := liveRow(2, 1, 7, time.Hour)

	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).Return(old, nil)
	mq.EXPECT().GetSessionByIDForUpdate(gomock.Any(), int64(1)).Return(old, nil)
	mq.EXPECT().InsertRotatedSession(gomock.Any(), gomock.Any()).Return(next, nil)
	mq.EXPECT().MarkSessionReplaced(gomock.Any(), sqlc.MarkSessionReplacedParams{
		SessionID:  1,
		ReplacedBy: pgtype.Int8{Int64: 2, Valid: true},
	}).Return(int64(1), nil)

	out, err := s.Refresh(context.Background(), "old-token", "web")
	require.NoError(t, err)
	assert.NotEqual(t, "old-token", out.RefreshToken)

	uid, err := s.ValidateAccess(out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(7), uid)
}

func TestRefresh_UnknownToken(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).
		Return(sqlc.IdentitySession{}, pgx.ErrNoRows)

	_, err := s.Refresh(context.Background(), "nope", "web")
	assert.ErrorIs(t, err, ErrInvalidSession)
}

func TestRefresh_ReuseRevokesFamily(t *testing.T) {
	s, mq := newService(t)
	old := liveRow(1, 5, 7, time.Hour)
	old.ReplacedBy = pgtype.Int8{Int64: 2, Valid: true}
	old.RevokedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).Return(old, nil)
	mq.EXPECT().RevokeSessionFamily(gomock.Any(), int64(5)).Return(nil)

	_, err := s.Refresh(context.Background(), "replayed", "web")
	assert.ErrorIs(t, err, ErrSessionReuse)
}

func TestRefresh_Expired(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).
		Return(liveRow(1, 1, 7, -time.Hour), nil)

	_, err := s.Refresh(context.Background(), "stale", "web")
	assert.ErrorIs(t, err, ErrInvalidSession)
}

// A concurrent refresher replaced the token between the hash lookup and
// the row lock — the lock-path re-check catches it and revokes the family.
func TestRefresh_ReplacedBetweenLookupAndLock(t *testing.T) {
	s, mq := newService(t)
	old := liveRow(1, 5, 7, time.Hour)
	locked := liveRow(1, 5, 7, time.Hour)
	locked.ReplacedBy = pgtype.Int8{Int64: 2, Valid: true}

	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).Return(old, nil)
	mq.EXPECT().GetSessionByIDForUpdate(gomock.Any(), int64(1)).Return(locked, nil)
	mq.EXPECT().RevokeSessionFamily(gomock.Any(), int64(5)).Return(nil)

	_, err := s.Refresh(context.Background(), "raced", "web")
	assert.ErrorIs(t, err, ErrSessionReuse)
}

func TestRevoke(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).Return(liveRow(1, 1, 7, time.Hour), nil)
	mq.EXPECT().RevokeSession(gomock.Any(), int64(1)).Return(int64(1), nil)

	require.NoError(t, s.Revoke(context.Background(), "tok"))
}

func TestRevoke_Unknown(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().GetSessionByRefreshHash(gomock.Any(), gomock.Any()).
		Return(sqlc.IdentitySession{}, pgx.ErrNoRows)

	assert.ErrorIs(t, s.Revoke(context.Background(), "nope"), ErrInvalidSession)
}

func TestValidateAccess(t *testing.T) {
	s, _ := newService(t)

	t.Run("rejects tampered token", func(t *testing.T) {
		other := &Service{secret: []byte("different"), now: time.Now}
		signed, err := other.signAccess(7)
		require.NoError(t, err)
		_, err = s.ValidateAccess(signed)
		assert.ErrorIs(t, err, ErrInvalidSession)
	})

	t.Run("rejects expired token", func(t *testing.T) {
		short := &Service{secret: s.secret, accessTTL: -time.Hour, now: time.Now}
		signed, err := short.signAccess(7)
		require.NoError(t, err)
		_, err = s.ValidateAccess(signed)
		assert.ErrorIs(t, err, ErrInvalidSession)
	})

	t.Run("rejects non-lena issuer", func(t *testing.T) {
		_, err := s.ValidateAccess("not-a-jwt")
		assert.ErrorIs(t, err, ErrInvalidSession)
	})
}

func TestPurgeExpired(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().DeleteExpiredSessions(gomock.Any(), gomock.Any()).Return(int64(3), nil)
	n, err := s.PurgeExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func TestIssue_DBError(t *testing.T) {
	s, mq := newService(t)
	mq.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(sqlc.IdentitySession{}, errDB)
	_, err := s.Issue(context.Background(), 7, "web")
	assert.Error(t, err)
}

// --- test doubles (mirrors internal/grocery/service_intx_test.go) ---

type stubTx struct{ closed bool }

func (f *stubTx) Begin(context.Context) (pgx.Tx, error) { return nil, errors.New("nested tx") }
func (f *stubTx) Commit(context.Context) error          { f.closed = true; return nil }
func (f *stubTx) Rollback(context.Context) error        { f.closed = true; return nil }
func (f *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (f *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (f *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (f *stubTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
func (f *stubTx) Conn() *pgx.Conn                                  { return nil }

type stubPool struct{ tx *stubTx }

func (p *stubPool) Begin(context.Context) (pgx.Tx, error) { return p.tx, nil }
func (p *stubPool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not implemented")
}
func (p *stubPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (p *stubPool) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
