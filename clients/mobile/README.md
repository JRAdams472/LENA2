# LENA Flutter Mobile Client

A Flutter client for the LENA GraphQL BFF.

## Requirements

- Flutter SDK >= 3.0.0
- Dart >= 3.0.0
- Android SDK with `minSdk 23` (for `mobile_scanner`)
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

## Project structure

- `lib/graphql_config.dart` — `GraphQLClient` with `AuthLink` + `ErrorLink`.
- `lib/main.dart` — App entry point with `GraphQLProvider` and `AuthGate`.
- `lib/auth/auth_service.dart` — Google sign-in + secure token storage.
- `lib/screens/login_screen.dart` — Google sign-in button.
- `lib/screens/main_screen.dart` — Bottom-nav shell.
- `lib/screens/dashboard_screen.dart` — Today's meal plan + recommendations.
- `lib/screens/grocery_lists_screen.dart` + `grocery_list_screen.dart` — Grocery list list/detail.
- `lib/screens/pantry_screen.dart` — Pantry quantities.
- `lib/screens/scan_screen.dart` — Barcode scan, UPC lookup, add/remove pantry, submit new items.
- `lib/scan/upc_utils.dart` — UPC digit normalization.

## Testing

```bash
flutter analyze
flutter test
```
