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

- `lib/theme.dart` — `lenaTheme()`: the web's sage/cream palette, bundled Nunito, and M3 component themes.
- `lib/graphql_config.dart` — `GraphQLClient` with `AuthLink` (proactive session refresh) + `ErrorLink` (one-time refresh-and-retry on 401).
- `lib/main.dart` — App entry point with `GraphQLProvider` and `AuthGate`.
- `lib/auth/auth_service.dart` — Google sign-in, `/auth/session` exchange, secure token storage, `LENA_DEBUG_ID_TOKEN` bypass for emulator/e2e runs.
- `lib/screens/login_screen.dart` — Google sign-in button.
- `lib/screens/main_screen.dart` — Bottom-nav shell.
- `lib/screens/dashboard_screen.dart` + `dashboard_content.dart` — Dashboard GraphQL wrapper + the greeting/meals/recommendations layout; display logic lives in pure helpers under `lib/dashboard_helpers.dart`.
- `lib/screens/grocery_lists_screen.dart` + `grocery_list_screen.dart` — Grocery list list/detail; the detail screen renders the server's `groceryRouteGroups` verbatim (aisle-grouped, per-group `ReorderableListView`, "move to aisle" menu, store picker). Ingredient-only lines show a `usual:` caption once a brand is remembered; the first check-off opens a brand picker (`checkGroceryItemWithBrand`) that records the household's usual.
- `lib/screens/pantry_screen.dart` — Pantry quantities.
- `lib/screens/scan_screen.dart` — Barcode scan, UPC lookup, add/remove pantry, submit new items; UPC hits display the resolved ingredient (household override wins) with a link-ingredient prompt (`setHouseholdItemIngredient`).
- `lib/scan/upc_utils.dart` — UPC digit normalization.
- `lib/ai/` — Ask Dot local inference: `engine.dart`/`protocol.dart`/`agent.dart` (engine contract, JSON tool protocol, bounded agent loop), `gemma_engine.dart` + `gemma_binding.dart` (`flutter_gemma` seam for tests), `model_manager.dart` (download/delete + dart-define config), `controller.dart` (orchestration + SharedPreferences mode), `api.dart` (assistant GraphQL queries).

## Testing

```bash
flutter analyze
flutter test
```
