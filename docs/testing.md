# Testing Guide

How to run and extend LENA2's test suites: Go unit + integration tests, Jest
unit/component tests, and Playwright end-to-end tests.

## Overview

| Layer | Location | Tooling | Runs in CI |
|---|---|---|---|
| Go unit + integration | `internal/<domain>/*_test.go`, `cmd/lena` | `go test`, `testify`, gomock, testcontainers | `test.yml` → `go` job |
| Frontend unit/component | `clients/web/__tests__/` | Jest 30, React Testing Library | `test.yml` → `web` job |
| End-to-end | `clients/web/e2e/` | Playwright against the Docker stack | `test.yml` → `e2e` job |
| Lint / static checks | repo-wide | `go vet`, `gofmt`, `golangci-lint` (`ci.yml`), `eslint`, `tsc` | both workflows |

## Go tests

```sh
# unit + integration (integration tests need a working Docker daemon for
# testcontainers; on Windows make sure Docker Desktop is running first)
go test ./cmd/... ./internal/...

# unit only (skip testcontainers suites)
go test -short ./cmd/... ./internal/...

# with coverage
go test -count=1 -coverprofile=coverage.out -covermode=atomic ./cmd/... ./internal/...
go tool cover -func coverage.out   # per-package + total
```

Notes:

- Prefer `./cmd/... ./internal/...` over `./...` — `clients/web/node_modules`
  can contain Go files that break coverage runs.
- `-race` requires cgo (a C toolchain); on Windows without gcc, drop the flag.
- Coverage gate: CI fails below `GO_COVERAGE_MIN` (currently 45%) in
  `.github/workflows/test.yml`. Raise it as coverage improves.

### Test helpers (`internal/platform/testenv`)

- `testenv.NewTestDB(t, ctx)` — starts a `postgres:16-alpine` testcontainer,
  applies all `migrations/*.up.sql` plus `migrations/seed/*.sql`, and returns a
  `*pgxpool.Pool` and a cleanup func (registers container termination).
- `testenv.RunMigrations(ctx, pool)` — apply migrations to an existing pool.
- `testenv.MustUser(ctx, t, pool, email)` — upserts a user and returns its ID
  (needed for `created_by`/`updated_by` FK columns).
- `testenv.WithUser(ctx, userID, email)` — returns a context carrying a
  `currentuser.User` for resolver tests that need an authenticated principal.
- `testenv.WithAdmin(ctx, userID, email)` — same, but with `IsAdmin: true`.
  Required for shared-catalog mutations: those resolvers call
  `requireAdmin`, which checks the persisted `identity.users.role` column
  (`member` by default). In production, `LENA_ADMIN_EMAILS` (comma-separated)
  promotes a matching user to `admin` on their next authenticated request.
  In e2e, `e2e@example.com` is seeded admin and `e2e-other@example.com`
  remains a member to exercise the `forbidden` rejection path.
- `testenv.NewTestIssuer(t)` — in-process OIDC issuer (JWKS + token endpoint).
  `issuer.Token(t, sub, email, name)` mints a signed ID token accepted by
  `NewAuthenticator` configured with that issuer URL/audience.

### Adding a service unit test

Each domain package (`internal/inventory`, `internal/wine`, `internal/recipe`,
`internal/mealplan`, `internal/grocery`, `internal/userprefs`,
`internal/identity`) tests `Service` methods against the SQLC `Querier` mock in
`internal/<domain>/sqlc/mock` (gomock-generated; the exact `mockgen` command
is in a comment at the top of each generated file — rerun it after changing
`queries.sql`). Example pattern:

```go
ctrl := gomock.NewController(t)
mq := mock.NewMockQueries(ctrl)
mq.EXPECT().
    GetItemByID(gomock.Any(), int64(5)).
    Return(sqlc.InventoryItem{Name: "Milk"}, nil)

svc := NewService(mq)
item, err := svc.GetItemByID(context.Background(), 5)
require.NoError(t, err)
assert.Equal(t, "Milk", item.Name)
```

BFF resolvers are unit-tested in `internal/bff/resolver_*_test.go` against
gomock-generated service mocks (`internal/bff/mock`, regenerate with the
`mockgen` command shown at the top of `internal/bff/mock/services.go`) and
integration-tested end-to-end
in `internal/bff/bff_integration_test.go`.

## Frontend (Jest) tests

```sh
cd clients/web
npm test                 # all suites, watch mode off
npm run test:coverage    # with coverage + threshold gate
```

- Suites live under `__tests__/` mirroring `app/`, `components/`, and `lib/`.
- `jest.setup.js` installs Testing Library matchers; `global.fetch` is mocked
  per-suite (see `__tests__/lib/api.test.ts` for `mockGraphQL` helpers). Note
  that item-list API calls issue **two** GraphQL requests (`items` +
  `userItems`); the `beforeEach` in `api.test.ts` mocks a default empty
  `userItems` response.
- Coverage thresholds are baselines in `jest.config.mjs`; raise them as
  coverage grows.

## End-to-end (Playwright) tests

```sh
# from the repo root — the e2e override adds the local OIDC test issuer
docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build

cd clients/web
npx playwright install chromium   # first time only
npx playwright test               # or: npm run test:e2e
```

Environment overrides:

- `E2E_BASE_URL` — defaults to `http://localhost` (Caddy fronts the web app,
  `/graphql`, and `/health`).
- `E2E_ISSUER_URL` — defaults to `http://localhost:8085` (`cmd/testissuer`).

Key points:

- `cmd/testissuer` is a tiny OIDC issuer the API trusts **only** when the stack
  is started with `docker-compose.e2e.yml` (`LENA_AUTH_ISSUERS` /
  `LENA_AUTH_AUDIENCES` include it). `e2e/helpers.ts` mints real signed tokens
  from it — no shared secrets or real Google sign-in.
- `e2e/auth.setup.ts` performs the browser sign-in once and stores
  `e2e/.auth/user.json` (gitignored); the `chromium` project depends on it.
- Bring the stack down/reset with
  `docker compose -f docker-compose.yml -f docker-compose.e2e.yml down -v`.
- Specs create unique rows via `unique()`/`uniqueCode()` helpers and clean up
  through GraphQL mutations where the API allows it (there is no
  `deleteGroceryList` mutation, so generated lists and their plans persist).
- CI runs the suite on `ubuntu-latest` inside the `e2e` job and uploads the
  HTML report (always) and `test-results/` on failure.

## CI layout

- `.github/workflows/test.yml` — `go` (build, vet, gofmt, tests + coverage
  gate + artifact), `web` (tsc, eslint, Jest + coverage artifact, next build),
  `e2e` (Playwright + report artifacts).
- `.github/workflows/ci.yml` — Go build/vet/test, `gofmt`, `golangci-lint`
  (includes `gosec`; see `.golangci.yml` for excluded rules).
- `.github/workflows/docker.yml` — builds and pushes `lena2-api` / `lena2-web`
  images on `main`.

## Troubleshooting

- **`rootless Docker is not supported on Windows` / testcontainers provider
  errors**: Docker Desktop is not running or its npipe dropped — restart it.
- **e2e `401` on every GraphQL call**: the `api` container was recreated
  without `docker-compose.e2e.yml`; always use both `-f` flags when bringing
  services up/down.
- **e2e tests pass locally but the stack shows stale UI**: the web container
  runs a *built* image — rebuild with
  `docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build`.
