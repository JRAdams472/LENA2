package bff

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/idempotency"
)

// Idempotency-Key is the header clients send to request exactly-once
// mutation semantics. Idempotency-Replayed marks responses served from the
// dedup store rather than a fresh execution.
const (
	headerIdempotencyKey      = "Idempotency-Key"
	headerIdempotencyReplayed = "Idempotency-Replayed"
)

// IdempotencyStore is the slice of *idempotency.Store the GraphQL handler
// uses; tests substitute a fake.
type IdempotencyStore interface {
	Begin(ctx context.Context, userID int64, key string, hash []byte) (*idempotency.Stored, error)
	Complete(ctx context.Context, userID int64, key string, status int32, body []byte) error
	Abandon(ctx context.Context, userID int64, key string) error
	Config() idempotency.Config
}

// isMutationOperation reports whether the document's first operation is a
// mutation. Clients send single-operation documents; a miss here only means
// the request skips dedup, so the cheap prefix check is safe.
func isMutationOperation(query string) bool {
	q := strings.TrimSpace(query)
	for strings.HasPrefix(q, "#") {
		if i := strings.IndexByte(q, '\n'); i >= 0 {
			q = strings.TrimSpace(q[i+1:])
		} else {
			return false
		}
	}
	if !strings.HasPrefix(q, "mutation") {
		return false
	}
	rest := q[len("mutation"):]
	return rest == "" || rest[0] == '{' || rest[0] == '(' || rest[0] == ' ' || rest[0] == '\n' || rest[0] == '\t' || rest[0] == '\r'
}

// idemClaim is a held dedup claim: complete() stores the response,
// abandon() releases the row when execution produced nothing cacheable.
type idemClaim struct {
	store  IdempotencyStore
	userID int64
	key    string
	done   bool
}

// complete stores the serialized response against the claim. Oversized
// responses abandon the claim instead — a reclaimed row degrades to a fresh
// execution on retry, which is always correct.
func (cl *idemClaim) complete(body []byte) {
	if cl == nil {
		return
	}
	cl.done = true
	if len(body) > cl.store.Config().WithDefaults().MaxResponseSz {
		cl.abandon()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cl.store.Complete(ctx, cl.userID, cl.key, http.StatusOK, body); err != nil {
		slog.Default().Warn("idempotency complete failed", "key", cl.key, "error", err)
	}
}

// abandon releases a claim that will never hold a response.
func (cl *idemClaim) abandon() {
	if cl == nil {
		return
	}
	cl.done = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cl.store.Abandon(ctx, cl.userID, cl.key); err != nil {
		slog.Default().Warn("idempotency abandon failed", "key", cl.key, "error", err)
	}
}

// abandonIfPending releases a claim only if it was never resolved — the
// deferred call sites panic/cancellation unwinding into an abandoned row.
func (cl *idemClaim) abandonIfPending() {
	if cl != nil && !cl.done {
		cl.abandon()
	}
}

// idemRejection describes a dedup rejection the handler renders as a
// GraphQL-shaped error response.
type idemRejection struct {
	msg  string
	code string
}

// beginDedup attempts the dedup claim for a mutation request. It returns a
// held claim (caller executes, then calls complete/abandon), a stored
// response to replay, or a rejection describing why the request must not
// run. A nil store or unauthenticated request skips dedup entirely; store
// failures fail open — dedup must never break the API.
func (r *Resolver) beginDedup(c echo.Context, userID int64, body []byte) (*idemClaim, *idempotency.Stored, *idemRejection) {
	store := r.IdemStore
	hash := idempotency.Hash(body)
	key := c.Request().Header.Get(headerIdempotencyKey)
	if key == "" {
		key = idempotency.AutoKey(hash)
	} else if len(key) > store.Config().WithDefaults().MaxKeyLen {
		return nil, nil, &idemRejection{
			msg:  "idempotency key exceeds maximum length",
			code: codeIdemKeyInvalid,
		}
	}

	stored, err := store.Begin(c.Request().Context(), userID, key, hash)
	switch {
	case err == nil && stored != nil:
		return nil, stored, nil
	case errors.Is(err, idempotency.ErrKeyReused):
		return nil, nil, &idemRejection{
			msg:  "idempotency key already used with a different payload",
			code: codeIdemKeyReused,
		}
	case errors.Is(err, idempotency.ErrInFlight):
		return nil, nil, &idemRejection{
			msg:  "an identical request is still processing; retry shortly",
			code: codeIdemInFlight,
		}
	case err != nil:
		slog.Default().Warn("idempotency begin failed; proceeding without dedup",
			"key", key, "error", err)
		return nil, nil, nil
	}
	return &idemClaim{store: store, userID: userID, key: key}, nil, nil
}

func idemErrorBody(msg, code string) map[string]any {
	return map[string]any{
		"errors": []map[string]any{{
			"message":    msg,
			"extensions": map[string]any{"code": code},
		}},
	}
}
