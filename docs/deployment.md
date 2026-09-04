# LENA — Deployment Guide

## 1. Runtime

- **Go monolith**: single binary compiled from `cmd/lena/main.go`.
- **PostgreSQL 16**.
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
      DATABASE_URL: postgres://lena_app:${LENA_DB_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable
      GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID:?}
      AUTH_ISSUERS: ${AUTH_ISSUERS:-https://accounts.google.com}
      CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-http://localhost,http://localhost:3000}
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

http://localhost {
    handle /graphql* {
        reverse_proxy api:8080
    }

    handle {
        reverse_proxy ui:3000
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
GOOGLE_CLIENT_ID=<client-id>
AUTH_ISSUERS=https://accounts.google.com
AUTH_AUDIENCES=<client-id>
CORS_ALLOWED_ORIGINS=http://localhost,http://localhost:3000
```

## 5. Build & Run

```bash
cp .env.example .env
# edit .env with real values
docker compose up --build
```

The GraphQL endpoint is available at `http://localhost/graphql`.

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