# LENA — Deployment Guide

## 1. Runtime

- **Go monolith**: single binary compiled from `cmd/lena/main.go`.
- **PostgreSQL 18** via `pgvector/pgvector` (recipe embeddings).
- **Caddy 2** reverse proxy — the only service that publishes ports.
- Optional workers: **Ollama** (AI assistant + OCR parsing + embeddings)
  and the **ocr** sidecar (pdf2image + tesseract).

`docker-compose.yml` is the source of truth; this document describes its
contract rather than copying it, so the two can't drift.

## 2. Compose topology

`docker-compose.yml` alone is the production profile. Service map:

| Service | Role | Publishes | Restart |
|---|---|---|---|
| `db` | pgvector Postgres 18 | none | `unless-stopped` |
| `db-migrate` | one-shot golang-migrate `up` | none | one-shot |
| `db-grant` | one-shot: `ALTER ROLE lena_app` password | none | one-shot |
| `db-seed` | one-shot: optional reference data (`--profile seed`) | none | one-shot |
| `api` | Go monolith | none | `unless-stopped` |
| `web` | Next.js client | none | `unless-stopped` |
| `caddy` | edge proxy | `80`, `443` (+443/udp) | `unless-stopped` |

**No database or API port is ever published to the host.** Caddy is the
only inbound edge; every other service stays on internal compose
networks. The debug overlay (`docker-compose.debug.yml`) re-adds
loopback-only forwards for api `8080`, web `3000`, db `5432` — never run
it in production (see below).

All images are digest-pinned (`pgvector:0.8.7-pg18@sha256:…`,
`migrate:v4.20.1@sha256:…`, `caddy:2.11.7-alpine@sha256:…`,
`ollama:0.40.5@sha256:…`). Bump tags and digests together.

**Networks are segmented.** `edge` (pinned `172.31.0.0/24`) carries
caddy → api/web; `default` carries api → db/ocr/ollama;
`observability` carries api → jaeger/seq. The api is the only service
bridged across all three — web and caddy cannot reach the database.

**Containers are locked down.** `no-new-privileges`, `cap_drop: ALL`
(caddy re-adds `NET_BIND_SERVICE` to bind :80/:443), read-only root
filesystems with `tmpfs /tmp`, non-root users, `pids_limit`.

**Runtime images are shell-less.** The api runs on
`gcr.io/distroless/static-debian12:nonroot` and web on
`gcr.io/distroless/nodejs24-debian13:nonroot`. The api healthcheck can't
shell out, so the binary self-probes: `lena -healthcheck` hits `/ready`
(database-verified), not just `/health` (process liveness).

## 3. Boot order

```
db (healthy) → db-migrate → db-grant → api (ready) → caddy
                                          ↑
                                         web
```

- **`db-migrate`** runs all `migrations/*.up.sql` as the **POSTGRES_USER
  superuser role** (`postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}`).
  Migrations create schemas, extensions, roles, and DDL — they need a
  DDL-capable role, which is why the connection string is the superuser
  and not `lena_app`.
- **`db-grant`** runs immediately after migrations and executes
  `ALTER ROLE lena_app WITH PASSWORD …` so the application role's
  password comes from `.env` rather than being baked into a migration.
  The password is passed through a `psql -v` variable (`:'pw'` quoting),
  so special characters can't break the statement.
- **`db-seed`** (optional, `--profile seed`) applies
  `migrations/seed/*.sql` once each, tracked in `public.schema_seed`.
  The api does **not** wait on it — the schema stands alone without
  reference data.
- **`api`** waits on `db-migrate` *and* `db-grant` completing
  successfully, then runs `LENA_DATABASE_URL` as `lena_app`.

## 4. Caddy / TLS

The shipped `Caddyfile` is the source of truth for routing and the
security-header block (X-Frame-Options, Referrer-Policy,
X-Content-Type-Options, Permissions-Policy, HSTS). Content-Security-Policy
is the exception: `clients/web/proxy.ts` issues it per-request with a
nonce, so Caddy must not set a static CSP — it would shadow or conflict.
Routing:

```caddy
{$CADDY_ADDR:-:80} {
    handle /graphql   { reverse_proxy api:8080 }
    handle /auth/*    { reverse_proxy api:8080 }   # session endpoints
    handle /health    { reverse_proxy api:8080 }
    handle            { reverse_proxy web:3000 }
}
```

**`CADDY_ADDR` selects HTTP vs HTTPS:**

| Value | Result |
|---|---|
| `:80` (default) | Local HTTP only. Ports 443 stay published but unused. |
| `example.com` | Caddy provisions a Let's Encrypt cert, serves HTTPS on 443 (+HTTP/3 over UDP), and redirects :80 → https. DNS must already resolve to the host and ports 80+443 must be internet-reachable for the ACME challenge. |

Never set `CADDY_ADDR=:443` alone — a bare port has no hostname for the
certificate, so TLS is dead and nothing is served.

**Proxy topology matters for rate limiting.** The API trusts
X-Forwarded-For only when the direct peer is inside
`LENA_TRUSTED_PROXY_CIDRS` — which must be exactly Caddy's pinned compose
address (`172.31.0.10/32` by default). Caddy faces the internet directly
and does **not** trust inbound XFF, so it rewrites the header from the
real peer and clients can't spoof the limiter key. Keep the subnet,
`ipv4_address`, and `LENA_TRUSTED_PROXY_CIDRS` in agreement; never
broaden the CIDR to a whole private range.

If you deploy behind an upstream LB/CDN instead, re-add a scoped
`servers { trusted_proxies static <lb-cidr> }` block to the Caddyfile so
Caddy preserves the upstream XFF.

## 5. Optional overlays

| Overlay | Adds | Invocation |
|---|---|---|
| `docker-compose.import.yml` | `ocr` sidecar + `ollama` + `ollama-pull` under `--profile import`; `ollama` alone under `--profile ai` | `docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up -d` |
| `docker-compose.gpu.yml` | NVIDIA device reservation for ollama | append `-f docker-compose.gpu.yml` to the import/ai invocation |
| `docker-compose.debug.yml` | debug images (shell + curl), loopback port forwards | see below |
| `docker-compose.logging.yml` | Seq/GELF centralized logging (loopback-only) | `-f docker-compose.logging.yml` |
| `docker-compose.telemetry.yml` | Jaeger/OTLP tracing | `-f docker-compose.telemetry.yml` |
| `docker-compose.e2e.yml` | Playwright test stack + local OIDC issuer | CI only |
| `docker-compose.sonarqube.yml` | local SonarQube on `127.0.0.1:9000` | analysis only |

**Ollama is CPU-only by default** so `--profile import`/`--profile ai`
work on any host. GPU acceleration requires an NVIDIA card plus the
Container Toolkit; layer `docker-compose.gpu.yml` (compose cannot
env-toggle a `deploy.reservations` block, so it lives in its own file).
The `ollama-pull` one-shot pre-pulls both `LENA_OLLAMA_MODEL` and
`LENA_AI_EMBED_MODEL` against the bundled instance.

**Debug overlay (local only):**

```powershell
docker compose -f docker-compose.yml -f docker-compose.debug.yml `
  -f docker-compose.import.yml -f docker-compose.logging.yml `
  -f docker-compose.telemetry.yml --profile import up -d --build
```

It swaps api/web to alpine `debug` targets (shell + curl), lifts
`read_only`, and adds loopback port forwards. Those direct ports bypass
the edge proxy's XFF handling — never run it in production.

## 6. Required Environment Variables

Copy `.env.example` to `.env` and fill in:

```env
POSTGRES_USER=lena
POSTGRES_PASSWORD=<strong-superuser-password>   # db + db-migrate + db-grant
POSTGRES_DB=lena
LENA_DB_PASSWORD=<strong-app-password>          # lena_app role, set by db-grant

LENA_GOOGLE_CLIENT_ID=<client-id>.apps.googleusercontent.com
LENA_AUTH_AUDIENCES=<same client-id>
NEXT_PUBLIC_GOOGLE_CLIENT_ID=<same client-id>   # REQUIRED — baked into the
                                              # web build; missing = no
                                              # Google sign-in button
```

Recommended/optional additions:

```env
# TLS — a real hostname, not :443 (see §4)
CADDY_ADDR=example.com

# Long-lived sessions (rotating refresh tokens); unset = OIDC-only mode.
LENA_SESSION_SECRET=<openssl rand -hex 32>

# Extra sign-in providers — each is disabled until its id+secret are set.
LENA_DISCORD_CLIENT_ID= / LENA_DISCORD_CLIENT_SECRET=
LENA_MICROSOFT_CLIENT_ID= / LENA_MICROSOFT_CLIENT_SECRET= (+ LENA_MICROSOFT_TENANT=consumers)
LENA_FACEBOOK_CLIENT_ID= / LENA_FACEBOOK_CLIENT_SECRET=
LENA_APPLE_CLIENT_ID=<Services ID> + LENA_APPLE_TEAM_ID= + LENA_APPLE_KEY_ID=
  + LENA_APPLE_PRIVATE_KEY_FILE=<path to .p8> + LENA_APPLE_REDIRECT_URI=
  + LENA_APPLE_BUNDLE_ID=<iOS bundle id — trusts native tokens>
# + matching *_REDIRECT_URI values registered in each provider portal,
#   and LENA_*_MOBILE_REDIRECT_URI=lena://oauth/<provider> for the app's
#   deep-link sign-in flows (Apple is native — no mobile URI needed).

# AI assistant + semantic recipe search — each disabled until set.
# The `ai` compose profile starts a bundled Ollama and pre-pulls both models.
LENA_AI_PROVIDER=ollama          # or mock for deterministic e2e
LENA_OLLAMA_URL=http://ollama:11434
LENA_OLLAMA_MODEL=qwen2.5:7b-instruct  # also feeds OCR; chat default when AI_MODEL empty
LENA_AI_MODEL=                         # optional chat-model override
LENA_AI_EMBED_MODEL=nomic-embed-text   # must emit 768-dim vectors

# Security knobs (defaults shown); see .env.example for the full list.
LENA_TRUSTED_PROXY_CIDRS=172.31.0.10/32        # MUST equal Caddy's pinned IP
LENA_AUTH_CODE_EXCHANGE_RATE_LIMIT_PER_MINUTE=60  # global bucket on /auth/session/:provider
LENA_GRAPHQL_DISABLE_INTROSPECTION=false       # introspection is admin-only either way

# Mobile push notifications — default provider is `log` (writes the rendered
# payload to the server log; the outbox still drains, useful for dev/e2e).
# `fcm` requires a Firebase service-account JSON, mounted as a secret.
LENA_PUSH_PROVIDER=log
LENA_PUSH_POLL_INTERVAL=5s
LENA_PUSH_MAX_ATTEMPTS=5
# LENA_FCM_CREDENTIALS_FILE=/run/secrets/fcm-service-account.json

# Shopping-link provider (Instacart Developer Platform). Empty key disables
# the feature — `shopperProviders` returns [] and clients hide the action.
LENA_INSTACART_API_KEY=
LENA_INSTACART_BASE_URL=https://connect.dev.instacart.tools  # dev server; swap for prod
LENA_INSTACART_TIMEOUT=10s
```

## 7. Build & Run

```bash
cp .env.example .env
# edit .env with real values
docker compose up -d --build
```

The GraphQL endpoint is available at `http://localhost/graphql`
(`https://<domain>/graphql` when `CADDY_ADDR` is a domain).

Reference seed data (catalog categories, nutrient types, wine regions) is
opt-in and runs once — tracked in `public.schema_seed`:

```bash
docker compose --profile seed up db-seed
```

Centralized Seq/GELF logging is opt-in and binds to loopback only:

```bash
export SEQ_FIRSTRUN_ADMINPASSWORDHASH=<salted-hash>
docker compose -f docker-compose.yml -f docker-compose.logging.yml up -d
```

## 8. Local Development (no Docker)

```bash
# Start Postgres locally, create the lena database and lena_app role.
migrate -path ./migrations -database "postgres://lena_app:password@localhost:5432/lena?sslmode=disable" up
go run ./cmd/lena
```

See `docs/migration-runbook.md` for migration and seeding details.

## 9. Backup

Use `pg_dump` on a schedule:

```bash
docker compose exec db pg_dump -U lena_app -d lena > lena-backup-$(date +%F).sql
```

## 10. Health & Monitoring

- `/health` — process liveness (fast, no dependencies).
- `/ready` — readiness; verifies the database link. The compose
  healthcheck runs `lena -healthcheck` which probes this.
- Logs are structured JSON to stdout; the default `json-file` driver
  rotates at 10 MiB × 3. Centralized logging via
  `docker-compose.logging.yml` ships to Seq over GELF (loopback-only
  endpoint, daemon-side — unaffected by compose network segmentation).

## 11. Scaling

The monolith is stateless (sessions and idempotency records live in
Postgres); run multiple `api` replicas behind a load balancer if needed.
Postgres is the single source of truth. If you scale the api, keep
X-Forwarded-For trust scoped to the exact proxy addresses.
