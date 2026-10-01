# LENA — Deployment Guide

## 1. Runtime

- **Go monolith**: single binary compiled from `cmd/lena/main.go`.
- **PostgreSQL 18** (see `docker-compose.yml` for the pinned image).
- **Caddy 2** reverse proxy.

## 2. Docker Compose

```yaml
services:
  db:
    image: postgres:16-alpine
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

```caddy
{
    auto_https off
}

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

For production, replace `http://localhost` with the real domain and remove `auto_https off` so Caddy provisions Let's Encrypt.

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
LENA_SESSION_SECRET=<openssl rand -hex 32>

# Extra sign-in providers — each is disabled until its id+secret are set.
# Secrets stay server-side; NEXT_PUBLIC_* vars only carry the public
# client id so the web build shows the button.
LENA_DISCORD_CLIENT_ID= / LENA_DISCORD_CLIENT_SECRET=
LENA_MICROSOFT_CLIENT_ID= / LENA_MICROSOFT_CLIENT_SECRET= (+ LENA_MICROSOFT_TENANT=consumers)
LENA_FACEBOOK_CLIENT_ID= / LENA_FACEBOOK_CLIENT_SECRET=
# + matching *_REDIRECT_URI values registered in each provider portal.
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