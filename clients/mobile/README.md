# LENA Flutter Mobile Client

A Flutter client for the LENA GraphQL BFF.

## Requirements

- Flutter SDK >= 3.0.0
- Dart >= 3.0.0
- Android SDK — `minSdk` comes from `flutter.minSdkVersion` (24 on recent Flutter), which covers `mobile_scanner`'s floor of 23
- iOS 13.0+

## Getting started

```bash
cd clients/mobile
flutter pub get
flutter run
```

## Configuration

### API URL

Set `LENA_API_URL` via `--dart-define`:

```bash
# Android emulator
flutter run --dart-define=LENA_API_URL=http://10.0.2.2:8080/graphql

# iOS simulator / local physical Android on LAN
flutter run --dart-define=LENA_API_URL=http://<host-ip>:8080/graphql
```

If not provided, the default is `http://localhost:8080/graphql`.

### Google Sign-In

The BFF validates Google OIDC ID tokens with an audience matching
`LENA_AUTH_AUDIENCES`. Three values must be configured:

1. **Web/Server OAuth client ID** — used by `GoogleSignIn` as `serverClientId`
   and by `AuthService`. Set it at compile time via `--dart-define`:

   ```bash
   flutter run \
     --dart-define=LENA_GOOGLE_SERVER_CLIENT_ID=<web-client-id>
   ```

2. **iOS OAuth client ID** — used as `GIDClientID` in `ios/Runner/Info.plist`.

3. **Reversed iOS client ID** — used in the `CFBundleURLSchemes` array in
   `ios/Runner/Info.plist`.

The `ios/Runner/Info.plist` already has placeholders:

```xml
<key>GIDClientID</key>
<string>$(GOOGLE_CLIENT_ID)</string>
```

and a `CFBundleURLTypes` `CFBundleURLSchemes` entry:

```xml
<string>$(GOOGLE_REVERSED_CLIENT_ID)</string>
```

Set these through `ios/Flutter/Release.xcconfig` or directly in the plist:

```xml
<key>GOOGLE_CLIENT_ID</key>
<string>com.googleusercontent.apps.<ios-client-id></string>
<key>GOOGLE_REVERSED_CLIENT_ID</key>
<string>com.googleusercontent.apps.<reversed></string>
```

On iOS you must also pass `LENA_GOOGLE_IOS_CLIENT_ID` to `AuthService`:

```bash
flutter run \
  --dart-define=LENA_GOOGLE_SERVER_CLIENT_ID=<web-client-id> \
  --dart-define=LENA_GOOGLE_IOS_CLIENT_ID=<ios-client-id>
```

On Android this requires a matching `google-services.json` in
`android/app/google-services.json`. The `serverClientId` must be the **web**
client ID, not the Android client ID, so that the returned ID token's `aud`
matches the BFF's configured `LENA_AUTH_AUDIENCES`.

### Sessions

When the server sets `LENA_SESSION_SECRET`, `AuthService` exchanges the
Google credential at `POST /auth/session` for a LENA session: a ~15-minute
`iss=lena` access token plus a ~30-day rotating refresh token stored in
`flutter_secure_storage`. The auth link proactively refreshes before
sending an expired token, and `ErrorLink` retries a request once after a
401. Without the secret the app falls back to the raw Google ID token.

Discord/Microsoft/Facebook sign-in is web-only for now — mobile OAuth
needs an HTTPS-callback → `lena://` app-link bounce (deferred). A user
whose account is linked to Google can still sign in on mobile via Google.

### Emulator sign-in for screenshots/e2e

Debug builds accept a pre-minted OIDC token instead of Google sign-in —
useful on emulators without Play Services:

```bash
flutter run \
  --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
  --dart-define=LENA_DEBUG_ID_TOKEN=<test-issuer token>
```

The seeded token signs the app in at startup; if it has expired by the
time you open the app, the login screen's button uses it too, so the
flow works either way. The check is gated by `kDebugMode`, so release
builds compile the read out entirely. Mint a token from the e2e stack's
test issuer (`http://localhost:8085/token?sub=...&email=...`) — tokens
are valid for one hour.

### Push notifications (Android)

Push runs through Firebase Cloud Messaging and shares the
`google-services.json` used by Google sign-in — the file is gitignored and
the google-services Gradle plugin applies only when it is present. When it
is absent (CI, plain emulator/dev builds), `PushService` falls back to a
disabled stub instead of crashing, and notification delivery silently
skips the push channel.

For a real-device build the Firebase Android app's package name must match
`applicationId` (still the placeholder `com.example.lena_mobile` — the
rename is an explicit LEN-17 follow-up). Server side, set
`LENA_PUSH_PROVIDER=fcm` and `LENA_FCM_CREDENTIALS_FILE`
(see `../../docs/deployment.md`).

At runtime the app requests `POST_NOTIFICATIONS` after sign-in
(Android 13+), registers the FCM token via `registerDeviceToken`, dedupes
it in `shared_preferences`, re-registers on `onTokenRefresh`, and
unregisters best-effort on sign-out through `AuthService.onBeforeSignOut`
(which runs while the bearer is still valid). Foreground pushes post a
local heads-up on the `lena_default` channel and bump the unread badge;
taps route through the shared `notification_links.dart` kind→screen map.
Per-category Push switches live in Notification settings — `_all` is the
master opt-in and independent of the Feed switches.

## On-device AI (Ask Dot)

Ask Dot can run inference on the phone itself via `flutter_gemma` instead of
calling the server's provider — the model answers using the same read-only,
household-scoped GraphQL tools, and falls back to the server when the local
engine isn't ready. The user opts in on the assistant screen, downloads the
model once (progress + cancel + delete in the UI), and can force server mode
from the overflow menu.

The model artifact is configured at build time:

```bash
flutter run \
  --dart-define=LENA_LOCAL_MODEL_URL=https://.../model.task \
  --dart-define=LENA_LOCAL_MODEL_ID=my-gemma \
  --dart-define=LENA_LOCAL_MODEL_TOKEN=<download token if gated>
```

The default source is Hugging Face's license-gated
`litert-community/Gemma3-1B-IT` (~550 MB int4 LiteRT-LM); point
`LENA_LOCAL_MODEL_URL` at a self-hosted mirror for builds that can't accept
the HF license, and `LENA_LOCAL_MODEL_TOKEN` supplies the download token for
gated artifacts.

## Project structure

- `lib/theme.dart` — `lenaTheme()` + `lenaDarkTheme()`: the web's sage/cream palette, bundled Nunito, and M3 component themes; dark uses warm-charcoal surfaces with lifted sage accents per `docs/design.md`.
- `lib/theme_mode.dart` — `ThemeModeController` (provider): system/light/dark selection persisted in `shared_preferences`; the Appearance card on `more_screen.dart` drives it.
- `lib/widgets/` — shared UI primitives: `EmptyState`, `SectionHeader`, `StatusChip`, `LenaFadeIn` motion tokens, `LenaSkeleton`/`LenaSplash` loading states, `PagedListView` (scroll-triggered `fetchMore` merging for every connection list), `SearchPickerField`/`SearchPickerSheet` (debounced server-side entity pickers), `RecipePickerField` (recipe-specific wrapper with `mealType`/`categoryIds`).
- `lib/graphql_config.dart` — `GraphQLClient` with `AuthLink` (proactive session refresh) + `ErrorLink` (one-time refresh-and-retry on 401).
- `lib/main.dart` — App entry point with `GraphQLProvider` and `AuthGate`.
- `lib/auth/auth_service.dart` — Google sign-in, `/auth/session` exchange, secure token storage, `LENA_DEBUG_ID_TOKEN` bypass for emulator/e2e runs.
- `lib/screens/login_screen.dart` — split-card sign-in: sage brand panel over the Google button, theme-aware in both schemes.
- `lib/screens/main_screen.dart` — Bottom-nav shell.
- `lib/screens/dashboard_screen.dart` + `dashboard_content.dart` — Dashboard GraphQL wrapper + the greeting/meals/recommendations layout; display logic lives in pure helpers under `lib/dashboard_helpers.dart`.
- `lib/screens/grocery_lists_screen.dart` + `grocery_list_screen.dart` — Grocery list list/detail; the detail screen renders the server's `groceryRouteGroups` verbatim (aisle-grouped, per-group `ReorderableListView`, "move to aisle" menu, store picker). Ingredient-only lines show a `usual:` caption once a brand is remembered; the first check-off opens a brand picker (`checkGroceryItemWithBrand`) that records the household's usual.
- `lib/screens/pantry_screen.dart` — Pantry quantities.
- `lib/screens/recipes_screen.dart` + `edit_recipe_screen.dart` — recipe list (search, category filters) and the recipe detail/editor. Detail defaults to the household-effective view (`items(view:)`/`steps(view:)`); `householdDelta` drives the Household/Original toggle, the stale banner (`acknowledgeRecipeDelta`), and per-line/per-step delta badges. `recipe_tweaks.dart` renders the tweaks card + bottom-sheet editors (swap/adjust/remove/add for lines, replace/remove/add for steps) saving through `setRecipeDelta`/`clearRecipeDelta`; `lib/recipe_delta.dart` holds the pure draft model, `describe*Change` copy, serialization, and dirty-check equality.
- `lib/screens/scan_screen.dart` — Barcode scan, UPC lookup, add/remove pantry, submit new items; UPC hits display the resolved ingredient (household override wins) with a link-ingredient prompt (`setHouseholdItemIngredient`).
- `lib/screens/household_screen.dart` — members, roles, invites, and the caller's own allergy/dietary records (`setMyAllergy`). An active-household header opens a switcher bottom sheet listing every `myHouseholds` membership (per-row `setActiveHousehold` Switch, `leaveHousehold(householdId)`, New household dialog via `createHousehold`); invite accept shows a Join-and-merge / Just join prompt when the caller solely owns a household (`mergeFromHouseholdId`), and pointer-moving mutations reset the GraphQL store. Warning badges on grocery rows, recipe details, meal-plan slots, and event dishes open a dialog naming the member + allergen + flag kind; an entity with no flags reports "No allergen information", never "safe".
- `lib/allergy.dart` — shared warning model + badge/dialog widgets used by every surface.
- `lib/scan/upc_utils.dart` — UPC digit normalization.
- `lib/format.dart` — shared display helpers (localized dates, brand-prefix dedupe, weekday names, grocery-source copy).
- `lib/widgets/skeleton.dart` — `SkeletonList`/`SkeletonCard`/`SkeletonForm` loading placeholders and the `LenaSplash` boot screen.
- `lib/ai/` — Ask Dot local inference: `engine.dart`/`protocol.dart`/`agent.dart` (engine contract, JSON tool protocol, bounded agent loop), `gemma_engine.dart` + `gemma_binding.dart` (`flutter_gemma` seam for tests), `model_manager.dart` (download/delete + dart-define config), `controller.dart` (orchestration + SharedPreferences mode), `api.dart` (assistant GraphQL queries).
- `lib/push/push_service.dart` — push lifecycle with injectable `MessagingClient`/`HeadsUpNotifier` seams (permission, token register/dedup/refresh, sign-out unregister, foreground heads-up, tap routing). `lib/notification_links.dart` is the shared notification→screen map used by feed rows and push taps.

## Testing

```bash
flutter analyze
flutter test --coverage
```

`tools/coveragegate` enforces a 70% per-file floor on `lib/` — only
`lib/ai/gemma_binding.dart` (a platform-plugin wrapper that cannot run
under `flutter test`) is excluded, via `tools/coveragegate/floors.json`.

### Screenshot walk

`integration_test/screenshot_test.dart` drives every reachable screen on an
emulator against the seeded `lena2shots` stack and writes PNGs to
`mobile-shots/` when the test completes:

```bash
flutter drive --driver=test_driver/integration_test.dart \
  --target=integration_test/screenshot_test.dart -d emulator-5554 \
  --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
  --dart-define=LENA_DEBUG_ID_TOKEN=<test-issuer token>
```

Mint the token with
`http://localhost:8085/token?sub=e2e-user-1&email=e2e@example.com&name=E2E%20User`
—the seeded demo data belongs to `e2e-user-1`, and tokens expire after one
hour.
