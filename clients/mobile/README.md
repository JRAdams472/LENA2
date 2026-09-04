# LENA Flutter Mobile Client

A minimal Flutter starter that consumes the LENA GraphQL BFF.

## Getting started

```bash
cd clients/mobile
flutter pub get
flutter run
```

## Configuration

Set the API URL in `lib/graphql_config.dart`:

```dart
const String lenaApiUrl = 'http://localhost:8080/graphql';
```

The app expects an OIDC ID token stored with `flutter_secure_storage` under the key `id_token`. Replace `getIdToken()` in `lib/graphql_config.dart` with your provider's flow.

## Project structure

- `lib/graphql_config.dart` — Apollo-compatible GraphQL client setup with auth link.
- `lib/main.dart` — App entry point with `GraphQLProvider`.
- `lib/screens/home_screen.dart` — Example dashboard using `Me` and `userItems` queries.
- `lib/screens/grocery_screen.dart` — Example grocery list with `toggleGroceryItemChecked` mutation.
