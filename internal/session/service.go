// Package session implements LENA-issued sessions: a short-lived signed
// access token plus an opaque, rotating refresh token stored hashed.
// Refresh rotation tracks a token family; replaying an already-rotated
// token revokes the whole family, so a stolen token can't outlive the
// legitimate client.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/session/sqlc"
)

var (
	// ErrInvalidSession means the presented token is unknown, expired, or
	// revoked — the client must fall back to a full sign-in.
	ErrInvalidSession = errors.New("invalid session")
	// ErrSessionReuse means a rotated/revoked token was replayed; the whole
	// family has been revoked as a precaution.
	ErrSessionReuse = errors.New("session token reuse detected")
	// ErrUnavailable means session issuing is disabled (no signing secret).
	ErrUnavailable = errors.New("sessions are not configured")
)

const (
	issuer     = "lena"
	refreshLen = 32
)

// Config holds the session service knobs.
type Config struct {
	// Secret signs access tokens; empty disables the service.
	Secret string
	// AccessTTL is the signed-token lifetime.
	AccessTTL time.Duration
	// RefreshTTL is the sliding refresh-token lifetime.
	RefreshTTL time.Duration
}

// Issued is the credential bundle returned to a client.
type Issued struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Service issues, rotates, and revokes sessions.
type Service struct {
	q          sqlc.Querier
	pool       dbtx.Pool
	tx         pgx.Tx
	newQ       func(pgx.Tx) sqlc.Querier
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	// now is injectable for expiry-edge tests.
	now func() time.Time
}

// NewService builds the session service on the given pool. A nil pool is
// legal for tests that stub the querier only via withQuerier.
func NewService(pool dbtx.Pool, cfg Config) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "session")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "session"))
		},
		secret:     []byte(cfg.Secret),
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		now:        time.Now,
	}
}

// Enabled reports whether session issuing is configured.
func (s *Service) Enabled() bool { return s != nil && len(s.secret) > 0 }

// WithTx returns a copy bound to tx.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "session")) }
	}
	c := *s
	c.q = newQ(tx)
	c.tx = tx
	c.newQ = newQ
	return &c
}

// InTx runs fn in a transaction (joining an existing ctx/uow tx if one is
// already active).
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	if s.tx != nil || dbtx.HasTx(ctx) {
		return fn(s)
	}
	if s.pool == nil {
		return fmt.Errorf("session: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// Issue creates a new session family for the user and returns the first
// credential pair. The caller has already authenticated the user via the
// OIDC path.
func (s *Service) Issue(ctx context.Context, userID int64, device string) (Issued, error) {
	if !s.Enabled() {
		return Issued{}, ErrUnavailable
	}
	raw, hash, err := newRefreshToken()
	if err != nil {
		return Issued{}, err
	}
	row, err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:      userID,
		RefreshHash: hash,
		Device:      optText(device),
		ExpiresAt:   s.now().Add(s.refreshTTL),
	})
	if err != nil {
		return Issued{}, fmt.Errorf("create session: %w", err)
	}
	return s.bundle(row, raw)
}

// Refresh rotates a refresh token: the presented token is revoked and a
// new pair returned. Replaying an already-rotated or revoked token revokes
// the entire family and reports ErrSessionReuse; an unknown or expired
// token reports ErrInvalidSession.
func (s *Service) Refresh(ctx context.Context, refreshToken, device string) (Issued, error) {
	if !s.Enabled() {
		return Issued{}, ErrUnavailable
	}
	hash := hashToken(refreshToken)
	row, err := s.q.GetSessionByRefreshHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Issued{}, ErrInvalidSession
		}
		return Issued{}, fmt.Errorf("lookup session: %w", err)
	}
	// Reuse of a rotated/revoked token is a theft signal: kill the family.
	if err := s.rejectReused(ctx, row); err != nil {
		return Issued{}, err
	}
	if !row.ExpiresAt.After(s.now()) {
		return Issued{}, ErrInvalidSession
	}

	var out Issued
	err = s.InTx(ctx, func(tx *Service) error {
		// Re-check under the row lock so two concurrent refreshes of the
		// same token can't both succeed.
		cur, err := tx.q.GetSessionByIDForUpdate(ctx, row.SessionID)
		if err != nil {
			return fmt.Errorf("lock session: %w", err)
		}
		if err := tx.rejectReused(ctx, cur); err != nil {
			return err
		}
		iss, err := tx.rotateSessionTx(ctx, cur, device, s.now().Add(s.refreshTTL))
		if err != nil {
			return err
		}
		out = iss
		return nil
	})
	if err != nil {
		return Issued{}, err
	}
	return out, nil
}

// rejectReused revokes the session family and reports ErrSessionReuse when
// the presented token was already rotated or revoked.
func (s *Service) rejectReused(ctx context.Context, row sqlc.IdentitySession) error {
	if !row.RevokedAt.Valid && !row.ReplacedBy.Valid {
		return nil
	}
	if err := s.q.RevokeSessionFamily(ctx, row.FamilyID); err != nil {
		return fmt.Errorf("revoke session family: %w", err)
	}
	return ErrSessionReuse
}

// rotateSessionTx inserts the successor session and marks the current row
// replaced. A zero-row mark means a concurrent rotation won — reuse.
func (s *Service) rotateSessionTx(ctx context.Context, cur sqlc.IdentitySession, device string, expiresAt time.Time) (Issued, error) {
	raw, hash, err := newRefreshToken()
	if err != nil {
		return Issued{}, err
	}
	next, err := s.q.InsertRotatedSession(ctx, sqlc.InsertRotatedSessionParams{
		UserID:      cur.UserID,
		FamilyID:    cur.FamilyID,
		RefreshHash: hash,
		Device:      optText(device),
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return Issued{}, fmt.Errorf("rotate session: %w", err)
	}
	n, err := s.q.MarkSessionReplaced(ctx, sqlc.MarkSessionReplacedParams{
		SessionID:  cur.SessionID,
		ReplacedBy: pgtype.Int8{Int64: next.SessionID, Valid: true},
	})
	if err != nil {
		return Issued{}, fmt.Errorf("replace session: %w", err)
	}
	if n == 0 {
		return Issued{}, ErrSessionReuse
	}
	return s.bundle(next, raw)
}

// Revoke ends a single session (sign-out).
func (s *Service) Revoke(ctx context.Context, refreshToken string) error {
	if !s.Enabled() {
		return ErrUnavailable
	}
	row, err := s.q.GetSessionByRefreshHash(ctx, hashToken(refreshToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidSession
		}
		return fmt.Errorf("lookup session: %w", err)
	}
	if _, err := s.q.RevokeSession(ctx, row.SessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// ValidateAccess parses a LENA-issued access token and returns the user ID
// it was issued to.
func (s *Service) ValidateAccess(token string) (int64, error) {
	if !s.Enabled() {
		return 0, ErrUnavailable
	}
	t, err := jwt.Parse([]byte(token), jwt.WithKey(jwa.HS256(), s.secret), jwt.WithValidate(true))
	if err != nil {
		return 0, ErrInvalidSession
	}
	if iss, _ := t.Issuer(); iss != issuer {
		return 0, ErrInvalidSession
	}
	sub, ok := t.Subject()
	if !ok {
		return 0, ErrInvalidSession
	}
	id, err := strconv.ParseInt(sub, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidSession
	}
	return id, nil
}

// PurgeExpired deletes sessions past expiry or long-revoked; run from a
// periodic sweep.
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	n, err := s.q.DeleteExpiredSessions(ctx, sqlc.DeleteExpiredSessionsParams{
		ExpiresAt: s.now(),
		RevokedAt: pgtype.Timestamptz{Time: s.now().Add(-30 * 24 * time.Hour), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	return n, nil
}

// bundle signs the access token for a session row and pairs it with the
// raw refresh token (only ever returned to the client — never stored).
func (s *Service) bundle(row sqlc.IdentitySession, rawRefresh string) (Issued, error) {
	access, err := s.signAccess(row.UserID)
	if err != nil {
		return Issued{}, err
	}
	return Issued{AccessToken: access, RefreshToken: rawRefresh, ExpiresAt: row.ExpiresAt}, nil
}

func (s *Service) signAccess(userID int64) (string, error) {
	tok, err := jwt.NewBuilder().
		Issuer(issuer).
		Subject(strconv.FormatInt(userID, 10)).
		IssuedAt(s.now()).
		Expiration(s.now().Add(s.accessTTL)).
		Build()
	if err != nil {
		return "", fmt.Errorf("build access token: %w", err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.HS256(), s.secret))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return string(signed), nil
}

func newRefreshToken() (raw string, hash []byte, err error) {
	buf := make([]byte, refreshLen)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
