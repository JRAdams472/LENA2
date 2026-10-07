import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/auth/auth_service.dart';
import 'package:lena_mobile/screens/login_screen.dart';
import 'package:lena_mobile/theme.dart';
import 'package:lena_mobile/widgets/empty_state.dart';
import 'package:lena_mobile/widgets/status_chip.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _MemStorage implements TokenStorage {
  final map = <String, String>{};

  @override
  Future<String?> read(String key) async => map[key];

  @override
  Future<void> write(String key, String? value) async {
    value == null ? map.remove(key) : map[key] = value;
  }

  @override
  Future<void> delete(String key) async => map.remove(key);
}

AuthService _auth() => AuthService.test(
      storage: _MemStorage(),
      httpClient: http_testing.MockClient(
        (req) async => http.Response('{}', 401),
      ),
    );

Widget _wrap(Widget child, {bool dark = false, AuthService? auth}) =>
    MaterialApp(
      theme: lenaTheme(),
      darkTheme: lenaDarkTheme(),
      themeMode: dark ? ThemeMode.dark : ThemeMode.light,
      home: ChangeNotifierProvider.value(
        value: auth ?? _auth(),
        child: child,
      ),
    );

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  group('goldens', () {
    for (final dark in [false, true]) {
      final tag = dark ? 'dark' : 'light';
      testWidgets('login card — $tag', (tester) async {
        final auth = _auth();
        await auth.ready;
        await tester.pumpWidget(_wrap(const LoginScreen(), dark: dark, auth: auth));
        await tester.pumpAndSettle();
        await expectLater(
          find.byType(LoginScreen),
          matchesGoldenFile('goldens/login_$tag.png'),
        );
      });
    }

    testWidgets('empty state', (tester) async {
      await tester.pumpWidget(_wrap(const Scaffold(
        body: EmptyState(
          icon: Icons.kitchen_outlined,
          title: 'Your pantry is empty',
          description: 'Scan an item or add one to get started.',
          actionLabel: 'Add item',
          onAction: _noop,
        ),
      )));
      await tester.pumpAndSettle();
      await expectLater(
        find.byType(EmptyState),
        matchesGoldenFile('goldens/empty_state.png'),
      );
    });

    testWidgets('status chips — both schemes', (tester) async {
      await tester.pumpWidget(_wrap(const Scaffold(
        body: Column(
          children: [
            StatusChip(label: 'Active', tone: StatusTone.primary),
            StatusChip(label: 'owner', tone: StatusTone.primary),
            StatusChip(label: 'member', tone: StatusTone.neutral),
            StatusChip(label: 'Declined', tone: StatusTone.error),
          ],
        ),
      )));
      await tester.pumpAndSettle();
      await expectLater(
        find.byType(Scaffold),
        matchesGoldenFile('goldens/status_chips.png'),
      );
    });
  });
}

void _noop() {}
