# Go + PostgreSQL Rewrite — Missing Information & Requirements

The existing `lena-architecture-map.md` captures the *shape* of LENA, but it is not a re-implementation specification. To start over in **Go + PostgreSQL**, the following details must be gathered.

---

## 1. Requirements & Constraints That Are Not Written Down

- **Target scale**: number of users, recipes, items, meal plans, concurrent clients. PostgreSQL will not be the bottleneck for a personal app, but it changes whether you keep per-user `UserItem`/`UserBottle` splits or denormalize.
- **Self-hosting constraints**: must remain single-tenant, offline-capable? No cloud dependencies? This drives choices like containerization, Postgres image, backup strategy.
- **Mobile needs**: is the Flutter app a full client or just a grocery-list companion? The `mobile/` directory is incomplete.
- **Multi-IdP future**: currently Google only. If you want other providers later, `sub` + `provider` must remain the stable key, and the Go JWT middleware needs a provider-agnostic audience/issuer list.

---

## 2. The Complete API Contract

The architecture map lists routes at a high level. You need:

- **Every endpoint**: path, HTTP method, query parameters, request body, response body, status codes.
- **OpenAPI/Swagger output** (or `LENA.API.http` / `.http` files) to see exact request examples.
- **Frontend `lib/api.ts` and mobile `ApiService` calls** as the canonical consumer contract; these define what the new backend must not break.
- **Pagination rules**: `PaginationRequest.Clamp` allows only `10, 25, 50, 100` page sizes.

---

## 3. Business Rules Embedded in Stored Procedures

The map calls out two complex procedures (grocery generation and nutrition). You need all of them, because the current code pushes most logic into SQL:

- Full inventory of stored procedures in `LENA.Database/{Inventory,MealPlan,Recipe,Wine}/StoredProcedures/`.
- Exact calculations: recipe scaling by `Servings`, optional-ingredient selection, nutrition aggregation, UPC uniqueness, brand/category constraints, favorite handling.
- Transaction boundaries and error semantics: which procedures `THROW`, which return `@@ROWCOUNT`, which use `QueryMultiple`.
- The seed-data scripts and `Migrations/` to understand how the schema is evolving (e.g., `UserItem`, `UserBottle`, `UserRecipePreference`).

---

## 4. Domain Invariants & Validation Rules

- FluentValidation rules for every command (`CreateItemCommandValidator`, `CreateBottleCommandValidator`, etc.). These define allowed ranges, required fields, and cross-field checks.
- Unique constraints: `UQ_Item_Name_BrandID`, `UQ_Item_UPC12`, `UQ_Recipe_RecipeName`, `UQ_MealSlot_MealPlanID_DayOfWeek_MealType`, etc.
- Foreign-key behavior: `CASCADE`, `SET NULL`, and how that maps to Go repository logic.
- Soft-delete / `IsActive` semantics for global reference data.

---

## 5. Authentication & Authorization Details

- Google client ID, token lifetime, issuer/audience validation, and how `email` vs `sub` is used.
- User upsert behavior: what refreshes on every login (`LastLoginDate`, `DisplayName`, `Email`) and what does not.
- The exact shape of `GET /api/auth/me` response, because the frontend depends on it.
- Whether the app needs refresh tokens or relies on Google re-sign-in.

---

## 6. Data Model Gaps for PostgreSQL

- SQL Server `schema.table` usage → Postgres `schema` usage or `table_prefix` decision.
- Data types: `DATETIME2`, `DECIMAL(10,2)`, `NVARCHAR`, `TINYINT`, `BIT` mappings to Postgres.
- Identity columns (`INT IDENTITY`) → `SERIAL`/`GENERATED ALWAYS AS IDENTITY`.
- The multi-user split design: which columns are global vs per-user and where `UserID` is enforced.
- Index design for the new engine: the existing index files are SQL Server specific; you will need to re-derive them for Postgres query patterns.

---

## 7. Error, Logging, and Observability Contract

- The exact `ProblemDetails` schema returned by `GlobalExceptionHandler`.
- Which fields are logged and which are redacted (PII concern from the remediation plan).
- Whether Seq is required, or if structured JSON logging to stdout is enough.
- Request tracing / correlation IDs.

---

## 8. Operational/Deployment Details

- `docker-compose.yml` orchestration: you need equivalent `postgres`, `db-init` (migrations), `api`, `ui`, `proxy`, and `seq` services.
- Secrets management: `.env.example` variables (`GOOGLE_CLIENT_ID`, `POSTGRES_PASSWORD`, `LENA_DB_PASSWORD`, `NEXT_PUBLIC_*`, `API_BASE_URL`).
- Backup/migration path from existing SQL Server to Postgres if any data must be preserved.

---

## 9. Test Expectations

- `LENA.API.UnitTests`, `LENA.Application.UnitTests`, and `LENA.IntegrationTests` define the desired behavior. The Go rewrite needs equivalent coverage.
- Database fixture / test-container setup (e.g., `testcontainers` for Postgres).
- CI pipeline in `.github/workflows/ci.yml` needs to be translated to Go (`go test`, `golangci-lint`, `tparse`, etc.).

---

## 10. Go Architecture Decisions You Must Make

None of these are in the map because they are .NET-specific:

- **HTTP framework**: Gin, Echo, Fiber, or stdlib `net/http`.
- **Database layer**: `database/sql` + `pgx`, `sqlc`, `GORM`, or `Bun`.
- **Migrations**: `golang-migrate`, `goose`, `atlas`.
- **CQRS / mediator pattern**: custom bus, or drop MediatR and use service/use-case packages.
- **Dependency injection**: manual wiring, `wire`, or keep it simple.
- **Validation**: `go-playground/validator` or custom.
- **Auth middleware**: `golang-jwt/jwt` or `lestrrat-go/jwx` for Google ID token validation.
- **Project layout**: `internal/` packages by domain, or `pkg/` with ports-and-adapters.
