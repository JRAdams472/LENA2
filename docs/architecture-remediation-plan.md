Repository: JRAdams472/LENA. Phased remediation plan from an architecture / SonarQube-style review. Work the phases in order — they are ranked by risk (security, then correctness, then architecture, then scaling and best practices, then continuous enforcement). Each phase must build and pass the existing test suites before the next one starts (see Verification at the end).

## Phase 1 — Secrets & security hardening

Goal: no credentials in source control, no debug surfaces exposed outside development, no PII persisted to log files.

### 1a. Remove hard-coded SA credentials from `docker-compose.yml`
1. `docker-compose.yml` hard-codes the SQL Server SA password in four places: `MSSQL_SA_PASSWORD: "P@ssw0rd!"` on the `db` service (line 7), again in the `db` healthcheck `sqlcmd ... -P 'P@ssw0rd!'` (line 16), on the `db-init` service (line 28), and inside `ConnectionStrings__DefaultConnection=Server=db;Database=LENA;User Id=sa;Password=P@ssw0rd!;TrustServerCertificate=True;` on the `api` service (line 43).
2. Move the password into a Git-ignored `.env` file, following the pattern already documented in `docs/google-oauth-client-id.md` section 6 (`GOOGLE_CLIENT_ID` is read via `${GOOGLE_CLIENT_ID:?...}`). Introduce `MSSQL_SA_PASSWORD` and reference it as `${MSSQL_SA_PASSWORD:?MSSQL_SA_PASSWORD must be set in .env or environment}` in every place listed in step 1. The root `.gitignore` already ignores `*.env`; confirm a bare `.env` is covered (add an explicit `.env` entry if not).
3. Create a least-privilege application login instead of connecting as `sa`. Add a script under `LENA.Database/` (invoked from `LENA.Database/init.sh`, which `db-init` already runs) that creates a SQL login/user (e.g. `lena_app`) with only the permissions the API needs — `EXECUTE` on the stored-procedure schemas, since all data access goes through `[Schema].[usp_...]` procs. Source the app password from a second `.env` variable (e.g. `LENA_DB_PASSWORD`) and build `ConnectionStrings__DefaultConnection` from it.
4. Add a `.env.example` (committed, no real values) documenting `GOOGLE_CLIENT_ID`, `MSSQL_SA_PASSWORD`, and `LENA_DB_PASSWORD`, and update `README.md` and `docs/google-oauth-client-id.md` section 6 to mention the new variables.
5. Note in the connection-string documentation that `TrustServerCertificate=True` disables TLS certificate validation. Keep it for the local compose setup, but document that real deployments must provision a trusted certificate for SQL Server and drop this flag.

Acceptance: `git grep -n "P@ssw0rd"` returns nothing; `docker compose up` succeeds with a populated `.env` and fails fast with a clear message when a variable is missing; the API connects with the application login, not `sa`.

### 1b. Turn off or auth-gate Swagger outside development
1. `docker-compose.yml` sets `Swagger__Enabled=true` (line 46), which flips on Swagger UI via the gate in `LENA.API/Program.cs` (line 141: `if (app.Environment.IsDevelopment() || app.Configuration.GetValue<bool>("Swagger:Enabled"))`).
2. Remove `Swagger__Enabled=true` from the compose file (or move it into a `docker-compose.override.yml` used only for local development) so the default compose stack does not expose Swagger.
3. If Swagger must stay reachable in non-development environments, place `app.UseSwagger`/`app.UseSwaggerUI` behind authorization (e.g. a small middleware before the Swagger middleware in `LENA.API/Program.cs` that returns 401 for unauthenticated requests to the `Swagger:RoutePrefix` path, or restrict the path at the reverse proxy in `Caddyfile`).
4. Update the comment block at `LENA.API/Program.cs` lines 139-140 and `README.md` to describe the new behaviour.

Acceptance: with `ASPNETCORE_ENVIRONMENT=Production` and no `Swagger__Enabled`, `GET /api/swagger/index.html` returns 404 (or 401 when auth-gated); Swagger still works in `Development`.

### 1c. Add PII redaction / destructuring to logging
1. `LENA.Application/Behaviors/LoggingBehavior.cs` (line 23) logs the full request payload with `{@Request}` at Information level. Requests include emails (`UpsertUserCommand`), free-text notes, and purchase data.
2. Serilog in `LENA.API/Program.cs` (lines 22-28) writes rolling file logs retained for 31 days (`retainedFileCountLimit: 31`), so any PII logged in step 1 is persisted to disk.
3. Fix by doing one or more of the following, in preference order:
   - Log only the request *type name* at Information level (`typeof(TRequest).Name`) and move the destructured `{@Request}` to Debug/Verbose.
   - Add `Destructurama.Attributed` (or an equivalent `IDestructuringPolicy`) and mark sensitive properties on commands/entities (`Email`, `DisplayName`, `Notes`, purchase fields) with `[NotLogged]` / `[LogMasked]`.
   - Configure `Destructure.ToMaximumDepth(...)`/`ToMaximumStringLength(...)` on the Serilog `LoggerConfiguration` in `LENA.API/Program.cs` to cap what can leak.
4. Add a unit test in `LENA.Application.UnitTests` asserting `LoggingBehavior` does not emit sensitive fields at Information level (capture output with a Serilog test sink or an `ILogger<T>` fake).

Acceptance: a request carrying an email address produces no Information-level log line containing that address; file logs contain no request bodies.

### 1d. (Optional) Narrow CORS headers and methods
1. `LENA.API/Program.cs` (lines 51-52) uses `.AllowAnyHeader().AllowAnyMethod()`. Origins are already restricted via `Cors:AllowedOrigins`.
2. Inventory the headers and verbs used by `frontend/lib/api.ts` and `mobile/` (typically `Authorization`, `Content-Type`; `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`) and replace the wildcards with `.WithHeaders(...)` and `.WithMethods(...)`.
3. Re-run the frontend against the API to confirm no preflight failures.

Acceptance: browser preflight requests from the SPA succeed; requests using other methods/headers are rejected by CORS.

## Phase 2 — Correctness / data-isolation fixes

Goal: eliminate silent failure modes that can cross user boundaries or produce inconsistent data.

### 2a. Make `UserID` resolution fail loudly instead of defaulting to `0`
1. `LENA.API/Services/HttpContextCurrentUserService.cs` (line 37) returns `0` when the user cannot be resolved from `HttpContext`.
2. Every repository forwards that value straight into stored procedures, e.g. `UserID = _currentUser.UserID` in `LENA.Application/Repositories/BottleRepository.cs` (line 17). An unresolved user therefore reads/writes rows for `UserID = 0`, which is a data-isolation risk.
3. Change `ICurrentUserService` (in `LENA.Application`) so `UserID` throws a dedicated exception (e.g. `UnauthenticatedUserException`) when unresolved, or expose `TryGetUserId(out int userId)` and make `UserID` throw. Do not return `0`.
4. Map the new exception to `401 Unauthorized` (ProblemDetails) in `LENA.API/ExceptionHandling/GlobalExceptionHandler.cs`.
5. As defence in depth, add `IF @UserID IS NULL OR @UserID <= 0 THROW ...` guards to the user-scoped stored procedures in `LENA.Database/`, or validate `UserID > 0` in `BaseRepository` before executing any proc.
6. Add unit tests in `LENA.API.UnitTests` for `HttpContextCurrentUserService` (unauthenticated → throws) and in `LENA.Application.UnitTests` for a repository/handler receiving an unresolved user.

Acceptance: an authenticated request with no resolvable user yields a 401 ProblemDetails, never a query with `UserID = 0`.

### 2b. Route all timestamps through `TimeProvider`
1. `LENA.Application/Behaviors/AuditingBehavior.cs` (line 24) correctly uses `_timeProvider.GetUtcNow().UtcDateTime`.
2. `BottleRepository.SetFavoriteAsync` in `LENA.Application/Repositories/BottleRepository.cs` (lines 44-46) bypasses it with `DateTime.UtcNow`.
3. Inject `TimeProvider` into `BottleRepository` (or into `BaseRepository` so all repositories get it) and replace `DateTime.UtcNow` with `_timeProvider.GetUtcNow().UtcDateTime`.
4. Run `git grep -n "DateTime.UtcNow\|DateTime.Now\|DateTimeOffset.UtcNow" -- '*.cs'` and fix any other occurrences outside test projects.
5. Confirm `TimeProvider.System` is registered in `LENA.API/Program.cs` and that unit tests use a fake `TimeProvider` (e.g. `Microsoft.Extensions.TimeProvider.Testing`).

Acceptance: the grep in step 4 returns only test code; `SetFavoriteAsync` is testable with a fixed clock.

### 2c. Standardize not-found handling
1. `LENA.API/Controllers/BottleController.cs` (lines 39-40) returns `NotFound()` when a query result is null, while other flows throw `NotFoundException`, handled centrally in `LENA.API/ExceptionHandling/GlobalExceptionHandler.cs` (lines 34-39).
2. Pick one approach — recommended: handlers throw `NotFoundException` (consistent ProblemDetails body) and controllers never null-check.
3. Update `GetBottle` in `LENA.API/Controllers/BottleController.cs` and audit every other controller in `LENA.API/Controllers/` (`ItemController`, `GroceryListController`, `MealPlanController`, `RecipeController`, `CountryController`, `RegionController`, `VintageController`, `WineTypeController`) for `return NotFound()` patterns; move the null check into the corresponding MediatR query handler and throw `NotFoundException`.
4. Update/add controller tests in `LENA.API.UnitTests` so a missing entity is asserted through the exception path.

Acceptance: `git grep -n "NotFound()" LENA.API/Controllers` returns nothing; all 404 responses share the same ProblemDetails shape.

## Phase 3 — Architecture refactors

Goal: enforce clean-architecture boundaries, remove duplication, and make hidden coupling explicit.

### 3a. Extract a dedicated `LENA.Infrastructure` project for data access
1. Repository implementations and `DbConnectionFactory` currently live in `LENA.Application/Repositories`, giving the Application layer a hard dependency on `Microsoft.Data.SqlClient` (`LENA.Application/Repositories/DbConnectionFactory.cs`, line 4) and Dapper.
2. Create `LENA.Infrastructure/LENA.Infrastructure.csproj` (class library, same target framework as `LENA.Application`) referencing `LENA.Application` and `LENA.Domain`; move the `Microsoft.Data.SqlClient` and `Dapper` package references from `LENA.Application.csproj` to it.
3. Move `DbConnectionFactory`, `BaseRepository`, and every concrete `*Repository` class from `LENA.Application/Repositories` into `LENA.Infrastructure/Persistence/` (adjust namespaces). Keep only the interfaces (`IBottleRepository`, `IDbConnectionFactory`, `IRecipeRepository`, etc.) in `LENA.Application`.
4. Add an `AddInfrastructure(this IServiceCollection, IConfiguration)` extension in `LENA.Infrastructure/DependencyInjection.cs` containing the registrations currently inline in `LENA.API/Program.cs` (lines 118-134), and replace those lines with a single `builder.Services.AddInfrastructure(builder.Configuration);` call.
5. Register the new project in `LENA.slnx`; add a project reference from `LENA.API.csproj` and from `LENA.IntegrationTests.csproj` (which exercises the repositories via `DatabaseFixture`).
6. Verify `LENA.Application.csproj` no longer references `Microsoft.Data.SqlClient` or `Dapper`.

Acceptance: solution builds; `LENA.Application` has no database package references; all tests pass.

### 3b. Introduce request/response DTOs at the API boundary
1. Controllers bind and return domain entities directly, e.g. `CreateBottle([FromBody] Bottle bottle)` in `LENA.API/Controllers/BottleController.cs` (lines 95-98). This exposes audit fields (`CreatedAt`, `UpdatedAt`, `UserID`) and allows over-posting / mass assignment.
2. Add request DTOs (`CreateBottleRequest`, `UpdateBottleRequest`) and a response DTO (`BottleResponse`) under `LENA.API/Contracts/Wine/` (or `LENA.Application/Features/.../Dtos` if they should be shared with commands). Exclude audit and ownership fields from request DTOs.
3. Map DTO → command in the controller (or make the MediatR commands themselves the request contract) and entity → response DTO in the handler; keep mapping explicit (no reflection mappers required).
4. Repeat for the other controllers in `LENA.API/Controllers/`, prioritising write endpoints.
5. Update `frontend/lib/api.ts` types and `mobile/` models if any response field names change.
6. Add validators in `LENA.Application` for the new request types so `ValidationBehavior` covers them.

Acceptance: no controller action accepts or returns a type from `LENA.Domain`; posting `userID`/`createdAt` in a request body has no effect.

### 3c. Reduce duplication in `BottleRepository`
1. The full column/parameter list is hand-repeated in `CreateAsync` (`LENA.Application/Repositories/BottleRepository.cs` lines 51-76) and `UpdateAsync` (lines 91-117).
2. Extract a private `static object ToParameters(Bottle bottle, int userId, int? bottleId = null)` (or a small `BottleParameters` record) and use it in both methods.
3. Look for the same pattern in the other repositories (`ItemRepository`, `RecipeRepository`, `MealPlanRepository`, `GroceryListRepository`) and apply the same extraction.

Acceptance: each repository lists its columns once; integration tests for create/update still pass.

### 3d. Replace inline fully-qualified type references with an assembly marker
1. `LENA.API/Program.cs` (lines 100-104) uses `typeof(LENA.Application.Features.Wine.Bottles.Commands.CreateBottleCommand).Assembly` for both FluentValidation and MediatR registration.
2. Add `public interface IApplicationAssemblyMarker { }` (or a static `AssemblyReference` class) at the root of `LENA.Application`, and use `typeof(IApplicationAssemblyMarker).Assembly` in both registrations.

Acceptance: renaming/moving `CreateBottleCommand` no longer requires a change in `Program.cs`.

### 3e. Document the process-wide Dapper static mutation
1. `LENA.Application/Repositories/DbConnectionFactory.cs` (lines 12-15) sets `DefaultTypeMap.MatchNamesWithUnderscores = true` — a process-wide static that affects every Dapper query in the application domain, and is only executed once the factory is first constructed.
2. Move the assignment into the `AddInfrastructure` extension from step 3a (so it runs deterministically at startup rather than lazily), and add a short XML doc comment explaining that it is global and why it is required (snake_case columns in `LENA.Database`).
3. Ensure `LENA.IntegrationTests` (`DatabaseFixture`) sets the same option so tests behave identically.

Acceptance: the setting is applied exactly once at startup and is documented where it lives.

## Phase 4 — Scaling & best practices

Goal: remove per-request write amplification, bound response sizes, and prepare logging for multiple instances.

### 4a. Cache user resolution
1. `LENA.API/Middleware/UserResolutionMiddleware.cs` (lines 25-27) sends an `UpsertUserCommand` — a database write — on every authenticated request.
2. Add an `IMemoryCache` (or `HybridCache`) lookup keyed by the external subject claim (`sub`) with a short sliding expiration (e.g. 5-15 minutes). On a cache miss, run the upsert and cache the resolved `UserID`; on a hit, skip the upsert entirely.
3. Keep a periodic "last seen" refresh if that column matters (e.g. only upsert when the cached entry is older than N minutes).
4. Update `LENA.API.UnitTests` for the middleware: second request with the same subject does not send `UpsertUserCommand`.

Acceptance: N authenticated requests from one user produce one upsert per cache window.

### 4b. Add caching for read-heavy reference data
1. Countries, regions, wine types, and vintages change rarely and are read on most pages. Their handlers live under `LENA.Application/Features/Wine/` (`Countries`, `Regions`, `Types`, `Vintages`) and repositories `CountryRepository`, `RegionRepository`, `TypeRepository`, `VintageRepository`.
2. Add a caching decorator or a MediatR `CachingBehavior` for queries marked with an `ICacheableQuery` interface (key + TTL), backed by `IMemoryCache`/`HybridCache`.
3. Invalidate the relevant keys in the create/update/delete command handlers for those entities.
4. Add `Cache-Control`/`ETag` response headers on the corresponding GET endpoints in `CountryController`, `RegionController`, `WineTypeController`, `VintageController` so the SPA can cache client-side too.

Acceptance: repeated list requests for reference data hit the database at most once per TTL; a write invalidates the cache.

### 4c. Bound unbounded endpoints
1. `GetBottles` in `LENA.API/Controllers/BottleController.cs` (lines 20-25) calls `ListAllAsync` and can return an unbounded result set; the paged path (`GetBottlesPagedQuery` with `PaginationRequest.Clamp`, lines 30-31) already exists.
2. Either remove the unpaged endpoint, or make it delegate to the paged query with a default page size, and mark it `[Obsolete]` in the OpenAPI description.
3. Audit the other list endpoints (`ItemController`, `RecipeController`, `MealPlanController`, `GroceryListController`) for the same pattern and give each a paged variant using `PaginationRequest`.
4. Update `frontend/lib/api.ts` callers to use the paged endpoints.

Acceptance: every list endpoint enforces a maximum page size via `PaginationRequest.Clamp`.

### 4d. Move file-based Serilog logging to a centralized sink
1. `LENA.API/Program.cs` (lines 22-28) writes rolling files to local disk, which does not work across multiple API instances/containers.
2. Add a centralized sink (Seq via `Serilog.Sinks.Seq`, or OpenTelemetry via `Serilog.Sinks.OpenTelemetry` to an OTLP collector / ELK) configured from `appsettings*.json` (`Serilog:WriteTo`) rather than code, keep Console for container stdout, and make the file sink development-only.
3. Add the sink service (e.g. `seq`) to `docker-compose.yml` for local use and document the endpoint in `README.md`.
4. Enrich logs with a correlation/request id (`Enrich.FromLogContext()` + `UseSerilogRequestLogging`) so requests can be traced across instances.

Acceptance: running two `api` replicas produces a single, queryable log stream; no PII appears in it (per Phase 1c).

## Phase 5 — Continuous quality enforcement

Goal: prevent regressions in the categories above (bugs, security hotspots, duplication, code smells) on every commit.

1. Enable .NET analyzers solution-wide: add a root `Directory.Build.props` with `<EnableNETAnalyzers>true</EnableNETAnalyzers>`, `<AnalysisLevel>latest-recommended</AnalysisLevel>`, `<TreatWarningsAsErrors>true</TreatWarningsAsErrors>` (initially scoped to new warnings via an `.editorconfig` baseline if the count is large), and optionally `SonarAnalyzer.CSharp` as a package reference.
2. Add a root `.editorconfig` with formatting/naming rules and enforce with `dotnet format --verify-no-changes LENA.slnx`.
3. Frontend: the existing `.github/workflows/ci.yml` `Frontend` job already runs `npm run lint` (ESLint), `npx tsc --noEmit`, `npm test`, and `npm run build`; keep these required.
4. Extend the `Backend` job in `.github/workflows/ci.yml` with a `dotnet format --verify-no-changes LENA.slnx` step after `Restore`, and make sure `dotnet test LENA.slnx` can run `LENA.IntegrationTests` (SQL Server via a service container running `LENA.Database/init.sh`, or skip them via a trait filter if no database is available).
5. Wire SonarQube/SonarCloud (`SonarSource/sonarcloud-github-action` or `dotnet-sonarscanner`) into the same workflow as a new job with a quality gate that fails the PR on new bugs, security hotspots, duplicated blocks, or code smells; include `frontend/` in the analysis scope.
6. Protect `main` so the CI workflow and quality gate are required status checks.

Acceptance: a PR introducing a hard-coded secret, a `DateTime.UtcNow` call, or duplicated column lists fails CI before review.

## Verification

After completing each phase — and before starting the next — run the full build and existing test suites and require them to be green:

1. `dotnet build LENA.slnx`
2. `dotnet test LENA.slnx` (covers `LENA.API.UnitTests`, `LENA.Application.UnitTests`, and `LENA.IntegrationTests`; the integration tests need a reachable SQL Server, e.g. `docker compose up db db-init`)
3. `(cd frontend && npm run lint && npx tsc --noEmit && npm test)` for any phase that touches `frontend/`

Do not carry failing tests forward into a later phase; fix or explicitly document them in the phase's PR.
