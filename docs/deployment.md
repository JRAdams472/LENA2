# LENA — Deployment Guide

## 1. Runtime

- **Go monolith**: single binary compiled from `cmd/lena/main.go`.
- **PostgreSQL 18** (see `docker-compose.yml` for the pinned image).
- **Caddy 2** reverse proxy.

## 2. Docker Compose

```yaml
services:
  db:
    image: pgvector/pgvector:0.8.6-pg18
    container_name: lena-db
    environment:
      POSTGRES_USER: ${POSTGRES_USER:?}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?}
      POSTGRES_DB: ${POSTGRES_DB:-lena}
    volumes:
      - pg_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5

  db-migrate:
    image: migrate/migrate
    command: ["-path", "/migrations", "-database", "postgres://lena_app:${LENA_DB_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable", "up"]
    volumes:
      - ./migrations:/migrations:ro
    depends_on:
      db:
        condition: service_healthy

  api:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: lena-api
    environment:
      LENA_DATABASE_URL: postgres://lena_app:${LENA_DB_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable
      LENA_GOOGLE_CLIENT_ID: ${LENA_GOOGLE_CLIENT_ID:?}
      LENA_AUTH_ISSUERS: ${LENA_AUTH_ISSUERS:-https://accounts.google.com}
      LENA_AUTH_AUDIENCES: ${LENA_AUTH_AUDIENCES:?}
      LENA_CORS_ALLOWED_ORIGINS: ${LENA_CORS_ALLOWED_ORIGINS:-http://localhost}
      LENA_SESSION_SECRET: ${LENA_SESSION_SECRET:-}
      PORT: 8080
    ports:
      - "8080:8080"
    depends_on:
      db-migrate:
        condition: service_completed_successfully

  proxy:
    image: caddy:2-alpine
    container_name: lena-proxy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - api

volumes:
  pg_data:
  caddy_data:
  caddy_config:
```

## 3. Caddyfile

The shipped `Caddyfile` is the source of truth — see it for the full
security-header block (CSP, X-Frame-Options, Referrer-Policy,
X-Content-Type-Options, Permissions-Policy, HSTS). Routing:

```caddy
{$CADDY_ADDR:-:80} {
    handle /graphql {
        reverse_proxy api:8080
    }

    # Auth/session endpoints must reach the API, not the web container.
    handle /auth/* {
        reverse_proxy api:8080
    }

    handle /health {
        reverse_proxy api:8080
    }

    handle {
        reverse_proxy web:3000
    }
}
```

**Proxy topology matters for rate limiting.** The API keys its IP rate
limiter on the "real client" derived from X-Forwarded-For, and trusts that
header only when the direct peer is inside `LENA_TRUSTED_PROXY_CIDRS` —
which must be exactly Caddy's pinned compose address (`172.31.0.10/32` by
default). Caddy faces the internet directly and does **not** trust inbound
XFF: `servers { trusted_proxies }` is intentionally absent, so Caddy
rewrites the header from the real peer and a client cannot spoof the
limiter key. Never publish the API port to the host, and never broaden the
trusted CIDR to a whole private range — any compose-network peer could
then spoof client IPs.

If you deploy behind an upstream LB/CDN instead, re-add a scoped
`servers { trusted_proxies static <lb-cidr> }` block to the Caddyfile so
Caddy preserves the upstream XFF.

**Container networks are segmented.** Three networks exist: `edge`
(pinned `172.31.0.0/24`) carries caddy → api/web; `default` carries
api → db/ocr/ollama; `observability` carries api → jaeger/seq. The api is
the only service bridged across all three — web and caddy cannot reach
the database, and seq/jaeger cannot reach anything but each other and
the api's OTLP port. GELF log shipping is unaffected: it flows
daemon-side to the loopback-published `seq-gelf` UDP endpoint, not over
compose networks.

**Runtime images are shell-less (production profile).** The api runs on
`gcr.io/distroless/static-debian12:nonroot` and web on
`gcr.io/distroless/nodejs24-debian13:nonroot` — no shell, package
manager, or OS tools exist inside either container. The api healthcheck
can't shell out to curl, so the binary self-probes: `lena -healthcheck`.

`docker-compose.yml` alone is the production profile. For local
development/UAT, layer **`docker-compose.debug.yml`** over it:

```powershell
docker compose -f docker-compose.yml -f docker-compose.debug.yml `
  -f docker-compose.import.yml -f docker-compose.logging.yml `
  -f docker-compose.telemetry.yml --profile import up -d --build
```

The debug overlay swaps api/web to `debug` Dockerfile targets (alpine
images with a shell + curl, tagged `lena2-api:debug` / `lena2-web:debug`),
lifts `read_only`, and adds loopback-only port forwards: api `8080`, web
`3000`, db `5432` — direct access that bypasses Caddy. The rest of the
hardening (no-new-privileges, cap_drop, network segmentation) stays on.
Those direct ports skip the edge proxy's XFF handling and must never be
published beyond loopback — never run the debug overlay in production.

All containers additionally run with `no-new-privileges`, `cap_drop:
ALL` (caddy re-adds only `NET_BIND_SERVICE`), read-only root
filesystems with `tmpfs /tmp`, and non-root users — the ocr sidecar was
already the most isolated service and is unchanged.

For production, set `CADDY_ADDR` to your domain (e.g. `example.com`) so
Caddy provisions Let's Encrypt automatically; the HSTS and cookie
`Secure` flags activate on HTTPS automatically.

## 4. Required Environment Variables

Copy `.env.example` to `.env` and fill in:

```env
POSTGRES_USER=postgres
POSTGRES_PASSWORD=<strong-sa-password>
POSTGRES_DB=lena
LENA_DB_PASSWORD=<app-password>
LENA_GOOGLE_CLIENT_ID=<client-id>
LENA_AUTH_ISSUERS=https://accounts.google.com
LENA_AUTH_AUDIENCES=<client-id>
LENA_CORS_ALLOWED_ORIGINS=http://localhost
```

Recommended/optional additions:

```env
# Long-lived sessions (rotating refresh tokens); unset = OIDC-only mode.
# Browsers carry the refresh token in an HttpOnly SameSite=Strict cookie
# (path-scoped to /auth/session); mobile clients use the JSON field.
LENA_SESSION_SECRET=<openssl rand -hex 32>

# Extra sign-in providers — each is disabled until its id+secret are set.
# Secrets stay server-side; NEXT_PUBLIC_* vars only carry the public
# client id so the web build shows the button.
LENA_DISCORD_CLIENT_ID= / LENA_DISCORD_CLIENT_SECRET=
LENA_MICROSOFT_CLIENT_ID= / LENA_MICROSOFT_CLIENT_SECRET= (+ LENA_MICROSOFT_TENANT=consumers)
LENA_FACEBOOK_CLIENT_ID= / LENA_FACEBOOK_CLIENT_SECRET=
# + matching *_REDIRECT_URI values registered in each provider portal.

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
```

## 5. Build & Run

```bash
cp .env.example .env
# edit .env with real values
docker compose up --build
```

The GraphQL endpoint is available at `http://localhost/graphql`.

Reference seed data (catalog items, brands, nutrient types) is opt-in and
runs once — it is tracked in the `public.schema_seed` table:

```bash
docker compose --profile seed up db-seed
```

Centralized Seq/GELF logging is also opt-in via an overlay that binds to
loopback only:

```bash
export SEQ_FIRSTRUN_ADMINPASSWORDHASH=<salted-hash>
docker compose -f docker-compose.yml -f docker-compose.logging.yml up -d
```

## 6. Local Development (no Docker)

```bash
# Start Postgres locally, create lena database and lena_app user.
migrate -path ./migrations -database "postgres://lena_app:password@localhost:5432/lena?sslmode=disable" up
go run ./cmd/lena
```

## 7. Backup

Use `pg_dump` on a schedule:

```bash
pg_dump -h db -U lena_app -d lena > lena-backup-$(date +%F).sql
```

## 8. Health & Monitoring

- Add a `/health` endpoint returning `200`.
- Expose `/ready` after Postgres and migrations are connected.
- Logs are structured JSON to stdout; collect with a sidecar or `docker logs`.

## 9. Scaling

The monolith is stateless; run multiple `api` replicas behind a load balancer if needed. Postgres is the single source of truth.