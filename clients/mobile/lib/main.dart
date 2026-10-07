import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:provider/provider.dart';
import 'auth/auth_gate.dart';
import 'auth/auth_service.dart';
import 'graphql_config.dart';
import 'push/push_service.dart';
import 'theme.dart';
import 'theme_mode.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await themeModeController.load();
  pushService = await PushService.create(
    navigatorKey: navigatorKey,
    client: graphQLClient,
  );
  // Revoke this device's push token before the session tokens clear —
  // the unregister mutation needs a live bearer.
  authService.onBeforeSignOut = pushService.unregister;
  runApp(const LenaApp());
}

/// App-wide theme selection — Light/System/Dark (LEN-74).
final themeModeController = ThemeModeController();

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
        child: ChangeNotifierProvider.value(
          value: themeModeController,
          child: Builder(
            builder: (context) => MaterialApp(
              debugShowCheckedModeBanner: false,
              title: 'LENA',
              theme: lenaTheme(),
              darkTheme: lenaDarkTheme(),
              themeMode: context.watch<ThemeModeController>().mode,
              navigatorKey: navigatorKey,
              home: const AuthGate(),
            ),
          ),
        ),
      ),
    );
  }
}
