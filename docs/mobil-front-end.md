Repository: JRAdams472/LENA. Two deliverables, in this order.

## Deliverable 1 — Flutter mobile Google sign-in (Phase M5), replacing the native Android app

Context: The backend already validates Google ID tokens as JWT bearer tokens (see `LENA.API/Program.cs`, audience = `Authentication:Google:ClientId`, issuer `https://accounts.google.com`). The web app (`frontend/`) is the reference implementation: it obtains a Google credential (ID token), stores it under `lena_id_token`, and sends it as `Authorization: Bearer <token>`. `GET /api/auth/me` requires a valid token and returns the current user, so it is the endpoint to prove the login loop.

The existing `android/` directory is a native Kotlin/Gradle app and is being REPLACED by the new Flutter app. This is a full replacement:
1. Create a new Flutter project under a new top-level directory `mobile/`.
2. After the Flutter app reaches feature parity for the login loop, DELETE the entire `android/` directory (native Kotlin app, its `README.md`, Gradle files, etc.). `LENA.slnx` does not reference `android/`, so no solution changes are needed for the removal. Check the root `README.md` and `docs/` for any references to the native Android app and update them to point to the new `mobile/` Flutter app.

Flutter implementation steps:
3. Add dependencies to `mobile/pubspec.yaml`: `google_sign_in`, `flutter_secure_storage`, and `http` (or `dio`).
4. Implement Google sign-in that requests an ID token for the SAME OAuth client id the API validates as its audience. On Android this requires configuring the Web/Server client id so the returned ID token's `aud` matches `Authentication:Google:ClientId`. Document the required client-id configuration in `mobile/README.md`.
5. Persist the returned ID token using `flutter_secure_storage` (key analogous to the web `lena_id_token`). On startup, read it back; treat an expired token as signed-out.
6. Build a minimal HTTP client wrapper that attaches `Authorization: Bearer <idToken>` to every request and, on a 401 response, clears the stored token and returns to the sign-in screen (mirroring the web `setOnUnauthorized` behavior in `frontend/lib/api.ts`).
7. Build a login screen and a simple authenticated screen that calls `GET /api/auth/me` and displays the returned `email`/`displayName`, proving the full login loop.
8. Port the grocery-list functionality the native app provided so the replacement has parity: load a list via `GET /api/GroceryList/{id}`, check off items, and increment stock via `POST /api/Item/items/{id}/quantity?quantity=...&purchaseDate=...` (skip manual items where `itemID == null`), per the old `android/README.md`.
9. Make the API base URL configurable (via a build-time define or a config file), documenting the emulator address `http://10.0.2.2:<port>` as the old `android/README.md` did.
10. Add a `mobile/README.md` covering setup, the Google client-id requirement, and how to run against a local API.

## Deliverable 2 — Hardening pass

### 2a. Global fallback authorization (confirmed approach)
Currently only `AuthController.Me` has `[Authorize]`; all data controllers are open. Adopt a global fallback authorization policy so endpoints are secure-by-default, then explicitly opt out only where anonymous access is intended.
- In `LENA.API/Program.cs`, replace the bare `builder.Services.AddAuthorization()` with a configuration that sets a `FallbackPolicy` requiring an authenticated user (`new AuthorizationPolicyBuilder().RequireAuthenticatedUser().Build()`).
- Audit every controller in `LENA.API/Controllers/` (`BottleController`, `ItemController`, `GroceryListController`, `MealPlanController`, `RecipeController`, `CountryController`, `RegionController`, `VintageController`, `WineTypeController`, `AuthController`) and add `[AllowAnonymous]` only to endpoints that must remain public (likely none). Confirm `AuthController.Me` still behaves correctly.
- Update `LENA.API.UnitTests` to add coverage asserting the fallback policy is in effect (or that representative data controllers now require authorization). The existing `AuthControllerTests.Me_Should_Be_Decorated_With_AuthorizeAttribute` is a pattern to follow.
- Update `README.md`: the "no authentication or authorization" security note is now incorrect and must describe the enforced fallback authorization.

### 2b. Tighten CORS
`Cors:AllowedOrigins` is already required and non-empty at startup (`LENA.API/Program.cs`). Verify the configured values in `LENA.API/appsettings.json`, `LENA.API/appsettings.Development.json`, and `docker-compose.yml` (`Cors__AllowedOrigins__0`) reflect only the intended origins. Remove the misleading `AllowAnyOrigin()` snippet from `README.md` section 4 and describe the actual `Cors:AllowedOrigins` policy. Consider narrowing `AllowAnyHeader()`/`AllowAnyMethod()` in `LENA.API/Program.cs` if the set of headers/methods is known.

### 2c. Integration tests
The existing `LENA.IntegrationTests` project is DB-only (stored-proc tests via `DatabaseFixture`). Add HTTP-level integration tests using `Microsoft.AspNetCore.Mvc.Testing` (`WebApplicationFactory<Program>`) in a new test class (either in `LENA.IntegrationTests` with the needed package reference added to `LENA.IntegrationTests.csproj`, or a new project registered in `LENA.slnx`). Cover at minimum:
- A request to a protected data endpoint with no `Authorization` header returns `401` (now enforced by the fallback policy).
- `GET /api/auth/me` with no token returns `401`.
- A request with a valid (test-signed or mocked) token reaches the endpoint — this requires overriding the JWT bearer authentication scheme with a test authentication handler in the factory, since real Google tokens cannot be minted in tests.
Ensure these tests can run in CI without a live Google dependency.