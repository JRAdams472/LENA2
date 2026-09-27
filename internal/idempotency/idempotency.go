// Package idempotency owns the request-dedup store backing idempotent
// mutations. The BFF claims (user_id, key) rows before executing a GraphQL
// mutation, caches the serialized response on completion, and replays it for
// identical retries. Two callers cooperate here: explicit Idempotency-Key
// headers from clients, and "auto:" keys synthesized from payload hashes for
// the fallback dedup window.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/idempotency/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
)

// AutoKeyPrefix marks server-synthesized keys from the payload-hash fallback
// so they never collide with client-supplied keys.
const AutoKeyPrefix = "auto:"

var (
	// ErrKeyReused means the key was already completed with a different
	// request payload.
	ErrKeyReused = errors.New("idempotency key reused with a different payload")
	// ErrInFlight means an identical request is still executing and did not
	// finish within the wait window.
	ErrInFlight = errors.New("identical request still in flight")
)

// Config tunes retention and contention behavior. Zero values fall back to
// defaults.
type Config struct {
	KeyTTL        time.Duration // retention for completed keyed entries
	AutoTTL       time.Duration // retention for completed fallback entries
	InFlightTTL   time.Duration // in-progress rows older than this are stale
	WaitTimeout   time.Duration // max time a duplicate waits on in-flight
	PollInterval  time.Duration // poll cadence while waiting on in-flight
	MaxKeyLen     int           // reject longer client keys
	MaxResponseSz int           // responses larger than this are not cached
}

// WithDefaults returns the config with unset fields replaced by defaults.
func (c Config) WithDefaults() Config {
	if c.KeyTTL <= 0 {
		c.KeyTTL = 24 * time.Hour
	}
	if c.AutoTTL <= 0 {
		c.AutoTTL = 30 * time.Second
	}
	if c.InFlightTTL <= 0 {
		c.InFlightTTL = 60 * time.Second
	}
	if c.WaitTimeout <= 0 {
		c.WaitTimeout = 30 * time.Second
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 100 * time.Millisecond
	}
	if c.MaxKeyLen <= 0 {
		c.MaxKeyLen = 255
	}
	if c.MaxResponseSz <= 0 {
		c.MaxResponseSz = 1 << 20
	}
	return c
}

// Stored is a completed response safe to replay.
type Stored struct {
	Status int
	Body   []byte
}

// Store claims and resolves idempotency keys.
type Store struct {
	q         sqlc.Querier
	cfg       Config
	lastSweep atomic.Int64
}

// NewStore creates a Store on pool. The querier runs on the pool directly —
// deliberately not dbtx.ContextExecer — because claims and completions must
// commit immediately and must never join a caller's transaction.
func NewStore(pool dbtx.Pool, cfg Config) *Store {
	return &Store{
		q:   sqlc.New(dbtx.NewTimedExecer(pool, "idempotency")),
		cfg: cfg.WithDefaults(),
	}
}

// Hash returns the SHA-256 of the raw request body. The BFF hashes the
// untouched body bytes — there is nothing attacker-controlled to canonicalize
// beyond what the server will execute.
func Hash(body []byte) []byte {
	sum := sha256.Sum256(body)
	return sum[:]
}

// AutoKey converts a payload hash into a fallback key.
func AutoKey(hash []byte) string {
	return AutoKeyPrefix + hex.EncodeToString(hash)
}

// Config returns the effective configuration (defaults applied).
func (s *Store) Config() Config { return s.cfg }

// ttlFor maps a key to its retention class: client keys keep the long TTL,
// fallback "auto:" keys live only for the short dedup window.
func (s *Store) ttlFor(key string) time.Duration {
	if len(key) >= len(AutoKeyPrefix) && key[:len(AutoKeyPrefix)] == AutoKeyPrefix {
		return s.cfg.AutoTTL
	}
	return s.cfg.KeyTTL
}

// Begin attempts to claim (userID, key) for a request with payload hash.
// Returns:
//   - (nil, nil)          → claimed; the caller must execute and Complete or
//     Abandon the claim.
//   - (*Stored, nil)      → replay this stored response; do not execute.
//   - (nil, ErrKeyReused) → same key already completed a different payload.
//   - (nil, ErrInFlight)  → a twin request is still running.
func (s *Store) Begin(ctx context.Context, userID int64, key string, hash []byte) (*Stored, error) {
	s.maybeSweep()
	expires := time.Now().Add(s.ttlFor(key))
	waitUntil := time.Now().Add(s.cfg.WaitTimeout)

	for claims := 0; claims < 3; claims++ {
		n, err := s.q.Claim(ctx, sqlc.ClaimParams{
			UserID:      userID,
			Key:         key,
			RequestHash: hash,
			ExpiresAt:   expires,
		})
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return nil, nil
		}
		stored, retry, err := s.resolveConflict(ctx, userID, key, hash, waitUntil)
		if err != nil || !retry {
			return stored, err
		}
	}
	return nil, ErrInFlight
}

// resolveConflict inspects an existing row after a failed claim. retry=true
// means the row was reclaimed as stale or disappeared — claim again.
func (s *Store) resolveConflict(ctx context.Context, userID int64, key string, hash []byte, waitUntil time.Time) (*Stored, bool, error) {
	for {
		row, err := s.q.Get(ctx, sqlc.GetParams{UserID: userID, Key: key})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, err
		}
		if row.Status == "completed" {
			if !bytes.Equal(row.RequestHash, hash) {
				return nil, false, ErrKeyReused
			}
			status := 200
			if row.ResponseStatus.Valid {
				status = int(row.ResponseStatus.Int32)
			}
			return &Stored{Status: status, Body: row.Response}, false, nil
		}
		// in_progress: reclaim if stale, otherwise wait for the twin.
		if time.Since(row.CreatedAt) > s.cfg.InFlightTTL {
			if err := s.q.ReclaimStale(ctx, time.Now().Add(-s.cfg.InFlightTTL)); err != nil {
				return nil, false, err
			}
			return nil, true, nil
		}
		if time.Now().After(waitUntil) {
			return nil, false, ErrInFlight
		}
		timer := time.NewTimer(s.cfg.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, false, ctx.Err()
		case <-timer.C:
		}
	}
}

// Complete stores the finished response against an in-flight claim. A
// zero-row update means the claim was reclaimed as stale — not an error.
func (s *Store) Complete(ctx context.Context, userID int64, key string, status int32, body []byte) error {
	_, err := s.q.Complete(ctx, sqlc.CompleteParams{
		UserID:         userID,
		Key:            key,
		Response:       body,
		ResponseStatus: pgtype.Int4{Int32: status, Valid: true},
		ExpiresAt:      time.Now().Add(s.ttlFor(key)),
	})
	return err
}

// Abandon releases an in-flight claim without a stored response — used when
// execution could not produce a result (panic, disconnect).
func (s *Store) Abandon(ctx context.Context, userID int64, key string) error {
	return s.q.Abandon(ctx, sqlc.AbandonParams{UserID: userID, Key: key})
}

// Sweep removes expired completed rows and reclaims stale in-progress rows.
func (s *Store) Sweep(ctx context.Context) error {
	if err := s.q.DeleteExpired(ctx); err != nil {
		return err
	}
	return s.q.ReclaimStale(ctx, time.Now().Add(-s.cfg.InFlightTTL))
}

// maybeSweep runs Sweep detached at most once a minute. Expiry is also
// enforced row-by-row by the claim upsert, so the sweep only bounds table
// growth and laziness is safe.
func (s *Store) maybeSweep() {
	last := s.lastSweep.Load()
	now := time.Now().Unix()
	if now-last < 60 || !s.lastSweep.CompareAndSwap(last, now) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Sweep(ctx); err != nil {
			slog.Default().Warn("idempotency sweep failed", "error", err)
		}
	}()
}
