# LENA Flutter Mobile Client

A Flutter client for the LENA GraphQL BFF.

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

Set the **web OAuth client ID** as `LENA_GOOGLE_SERVER_CLIENT_ID` so the
returned OIDC `id_token` has an audience that matches the BFF's
`LENA_AUTH_AUDIENCES`. On iOS, also set `LENA_GOOGLE_IOS_CLIENT_ID` to the
iOS client ID.

```bash
flutter run \
  --dart-define=LENA_GOOGLE_SERVER_CLIENT_ID=<web-client-id> \
  --dart-define=LENA_GOOGLE_IOS_CLIENT_ID=<ios-client-id>
```

On Android this requires a matching `google-services.json` in
`android/app/google-services.json`. On iOS, add the `GIDClientID` value to
`ios/Runner/Info.plist` and the reversed client ID to the URL types.

## Project structure

- `lib/graphql_config.dart` — `GraphQLClient` with `AuthLink` + `ErrorLink`.
- `lib/main.dart` — App entry point with `GraphQLProvider` and `AuthGate`.
- `lib/auth/auth_service.dart` — Google sign-in + secure token storage.
- `lib/screens/login_screen.dart` — Google sign-in button.
- `lib/screens/main_screen.dart` — Bottom-nav shell.
- `lib/screens/dashboard_screen.dart` — Today's meal plan + recommendations.
- `lib/screens/grocery_lists_screen.dart` + `grocery_list_screen.dart` — Grocery list list/detail.
- `lib/screens/pantry_screen.dart` — Pantry quantities.
- `lib/screens/scan_screen.dart` — Barcode scan placeholder (p4).

## Testing

```bash
flutter analyze
flutter test
```
