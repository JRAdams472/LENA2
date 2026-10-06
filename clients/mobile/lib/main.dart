import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:provider/provider.dart';
import 'auth/auth_gate.dart';
import 'auth/auth_service.dart';
import 'graphql_config.dart';
import 'push/push_service.dart';
import 'theme.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  pushService = await PushService.create(
    navigatorKey: navigatorKey,
    client: graphQLClient,
  );
  // Revoke this device's push token before the session tokens clear —
  // the unregister mutation needs a live bearer.
  authService.onBeforeSignOut = pushService.unregister;
  runApp(const LenaApp());
}

/// Root navigator key so push taps can route from outside the widget tree.
final GlobalKey<NavigatorState> navigatorKey = GlobalKey<NavigatorState>();

class LenaApp extends StatelessWidget {
  const LenaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return GraphQLProvider(
      client: ValueNotifier(graphQLClient),
      child: ChangeNotifierProvider.value(
        value: authService,
        child: MaterialApp(
          debugShowCheckedModeBanner: false,
          title: 'LENA',
          theme: lenaTheme(),
          navigatorKey: navigatorKey,
          home: const AuthGate(),
        ),
      ),
    );
  }
}
