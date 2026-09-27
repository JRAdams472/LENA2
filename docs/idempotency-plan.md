# Idempotent API Endpoints — Phased Implementation

Add Stripe-style idempotency-key deduplication plus a short-window payload-hash
fallback to the single `/graphql` endpoint, wire keys into the web and mobile
clients, and follow with targeted semantic hardening — a medium-large but
additive change that avoids rewriting 110 resolvers.

## Objective

Make every GraphQL mutation safe to replay: client retries after network
errors, mobile offline replays, and double-submits must apply **exactly once**.
Scope decision (user-confirmed): transport-level dedup (`Idempotency-Key`
header + payload-hash fallback) first, targeted semantic hardening second.
Delta mutations (`adjustUserItem`, `incrementUserItem`, `adjustUserBottle`,
`toggleGroceryItemChecked`) keep their signatures — keys provide exactly-once.

## Size assessment

**Medium-large, but additive — not a rewrite.** All 110 mutations funnel
through one `POST /graphql` (`cmd/lena/main.go` → `bff.NewGraphQLHandler` →
`parsed.Exec`). A single dedup layer inside that handler covers every mutation
without touching resolvers or services. The rejected alternative — semantic
idempotency per mutation (upserts/set-semantics everywhere) — would be a
multi-month, high-risk rewrite; we only apply targeted semantic fixes in a
later phase where keys can't help.

## Design

### Server — `internal/idempotency` + handler integration (phase 1, landed)

**Migration `0034_idempotency`** — new `platform` schema (matches
`internal/platform/` Go naming):

```sql
CREATE TABLE platform.idempotency_key (
    user_id         BIGINT       NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    key             TEXT         NOT NULL,
    request_hash    BYTEA        NOT NULL,  -- sha256 of the raw request body
    status          TEXT         NOT NULL,  -- 'in_progress' | 'completed'
    response        JSONB,
    response_status INT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (user_id, key)
);
```

**Store (`internal/idempotency`)** — sqlc-backed, `Claim` is an upsert that
takes over only expired rows, so expiry is enforced row-by-row with no cron:

- `Begin(userID, key, hash)` → `nil` (claim held — caller executes then
  `Complete`/`Abandon`), `*Stored` (replay), `ErrKeyReused` (same key,
  different payload), or `ErrInFlight` (twin still running past the wait
  bound). A duplicate arriving while its twin is in flight **waits** on a
  poll loop (bounded by `WaitTimeout`) and replays once the twin completes —
  friendlier than the originally-planned immediate error; stale in-progress
  rows older than `InFlightTTL` are reclaimed and re-claimed.
- `Complete(userID, key, status, body)` — stores the serialized response;
  keyed rows keep `KeyTTL`, `auto:` rows keep `AutoTTL`.
- `Abandon` — releases a claim that produced no response (panic, oversized
  body).
- `Sweep` — deletes expired/stale rows; invoked opportunistically at most
  once a minute on the `Begin` path (no cron infra exists).

**Handler (`internal/bff/idempotency.go` + `resolver.go`):** reads the raw
body once (hash = sha256 of exact bytes — no canonicalization needed since we
hash what the server executes), detects mutations by leading `mutation`
keyword (queries bypass dedup — caching real-time reads would be wrong), and:

- `Idempotency-Key` header present → claim under that key (`KeyTTL`);
  >255 chars → `IDEMPOTENCY_KEY_INVALID`.
- Header absent → synthetic `auto:<hex(hash)>` key (`AutoTTL` fallback
  window) — byte-identical mutations inside the window dedup even without
  client cooperation.
- Replay writes the stored body verbatim with `Idempotency-Replayed: true`.
- Store failures fail **open** — dedup never breaks the API.
- Error codes: `IDEMPOTENCY_KEY_REUSED`, `IDEMPOTENCY_IN_FLIGHT`,
  `IDEMPOTENCY_KEY_INVALID` in `extensions.code`.
- Wiring via `bff.Options.Idempotency` → `Resolver.IdemStore` (nil disables);
  `NewGraphQLHandler`'s signature is unchanged.

**Config:** `LENA_IDEMPOTENCY_ENABLED` (default true),
`LENA_IDEMPOTENCY_KEY_TTL` (24h), `LENA_IDEMPOTENCY_AUTO_TTL` (30s),
`LENA_IDEMPOTENCY_IN_FLIGHT_TTL` (60s), `LENA_IDEMPOTENCY_WAIT_TIMEOUT` (30s).

### Web client (phase 2)

- `request()`: mutations get `Idempotency-Key: crypto.randomUUID()`;
  one bounded retry on network `TypeError` reuses the same key.
- Jest: header on mutations / absent on queries; retry reuses key.

### Mobile client (phase 3)

- `flutter pub add uuid`; `IdempotencyLink` inspects operation type and sets
  `Idempotency-Key` via `HttpLinkHeaders` for mutations only, inserted before
  `HttpLink` in `graphql_config.dart`. Unit test header injection.

## Phases (each = branch `idempotency-pN` + PR)

1. **`idempotency-p1` — Server core** ✅ migration 0034,
   `internal/idempotency` (sqlc + store), handler integration, config +
   `.env.example`, this doc. Tests: store lifecycle/replay/reuse/isolation/
   in-flight/stale/expired/abandon (integration) and handler claim/replay/
   conflict/fallback/fail-open (unit).
2. **`idempotency-p2` — Web client:** key generation + header + same-key
   retry in `request()`; jest coverage; tsc/eslint/jest green.
3. **`idempotency-p3` — Mobile client:** `uuid` dep, `IdempotencyLink`, link
   chain wiring; unit test; `flutter analyze` + `flutter test`.
4. **`idempotency-p4` — Targeted semantic hardening:** map unique-violation
   (23505) to friendly `CONFLICT` or return-existing on top create mutations;
   decide `generateGroceryList` duplicate handling; document the intentionally
   non-idempotent set (deltas, toggle, telemetry) — exactly-once only with
   keys.
5. **`idempotency-p5` — Closeout:** README/wiki updates, e2e smoke, branch
   cleanup, full verification matrix, and **mark the "System validations —
   make the entire interface and api idempotent" item in `docs/newfeatures.md`
   as complete**.

## Risks / decisions logged

- **Fallback false-positives:** legitimate rapid repeats (two `+1 milk` taps
  inside 30s) get deduped. Mitigated by the short configurable window
  (`AUTO_TTL`); the accepted tradeoff for zero-client-change protection.
  Watch e2e for flakes.
- **In-flight same-key:** bounded wait then `IDEMPOTENCY_IN_FLIGHT` — a
  client-visible error only if the twin genuinely stalls.
- **Replay skips fresh auth checks:** stored response is scoped to
  `(user_id, key)` + TTL — same-user replay only.
- **Replays still consume rate-limit budget** — desirable (prevents replay
  storms).
- **Table growth:** TTL + opportunistic sweep.
- **Query staleness avoided:** dedup applies to mutations only.
