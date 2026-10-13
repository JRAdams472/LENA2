import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'main.dart' as app;

/// Screenshot entrypoint — seeds a session bearer minted by the e2e
/// issuer into secure storage, then boots the normal app. Used only for
/// wiki/proof captures against the lena2shots stack:
///
///   flutter run -t lib/main_shots.dart \
///     --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
///     --dart-define=SHOTS_ID_TOKEN=<id_token from localhost:8085/token>
void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  const tok = String.fromEnvironment('SHOTS_ID_TOKEN');
  if (tok.isNotEmpty) {
    const store = FlutterSecureStorage();
    await store.write(key: 'id_token', value: tok);
    await store.delete(key: 'refresh_token');
  }
  app.main();
}
