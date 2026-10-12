import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/auth/auth_service.dart';
import 'package:lena_mobile/screens/login_screen.dart';
import 'package:provider/provider.dart';

class MemStorage implements TokenStorage {
  final map = <String, String>{};
  @override
  Future<String?> read(String key) async => map[key];
  @override
  Future<void> write(String key, String? value) async {
    if (value == null) {
      map.remove(key);
    } else {
      map[key] = value;
    }
  }

  @override
  Future<void> delete(String key) async {
    map.remove(key);
  }
}

AuthService _svc({Future<String?> Function()? googleSignIn}) {
  return AuthService.test(
    storage: MemStorage(),
    httpClient: http_testing.MockClient((req) async => http.Response('', 404)),
    googleSignIn: googleSignIn,
  );
}

Widget _app(AuthService svc) {
  return ChangeNotifierProvider<AuthService>.value(
    value: svc,
    child: const MaterialApp(home: LoginScreen()),
  );
}

void main() {
  testWidgets('login shows the compliance copy and provider stack', (
    tester,
  ) async {
    final svc = _svc();
    await svc.ready;
    await tester.pumpWidget(_app(svc));
    await tester.pumpAndSettle();

    // Exact copy from the store-compliance spec (LEN-136).
    expect(find.text('Welcome to Lena'), findsOneWidget);
    expect(
      find.text(
        'Sign in with the provider associated with your web account to '
        'access your dashboard and configure notifications.',
      ),
      findsOneWidget,
    );
    for (final label in [
      'Continue with Apple',
      'Continue with Google',
      'Continue with Discord',
      'Continue with Facebook',
    ]) {
      expect(find.text(label), findsOneWidget);
    }
    // Footer is present verbatim — and rendered as plain Text, never a
    // link (a tappable lenaskitchen.app URL fails review).
    final footer = find.text(
      'Account registration and plan management are handled on our '
      'website. New users must set up their accounts on '
      'lenaskitchen.app before logging in to the mobile companion app.',
    );
    expect(footer, findsOneWidget);
    for (final tappable in [InkWell, GestureDetector, TextButton]) {
      expect(
        find.ancestor(of: footer, matching: find.byType(tappable)),
        findsNothing,
        reason: 'footer must not be wrapped in a $tappable',
      );
    }
  });

  testWidgets('the four buttons stack vertically at equal prominence', (
    tester,
  ) async {
    final svc = _svc();
    await svc.ready;
    await tester.pumpWidget(_app(svc));
    await tester.pumpAndSettle();

    final xs = <double>[];
    for (final label in [
      'Continue with Apple',
      'Continue with Google',
      'Continue with Discord',
      'Continue with Facebook',
    ]) {
      final button = tester.widget<ElevatedButton>(
        find.ancestor(
          of: find.text(label),
          matching: find.byType(ElevatedButton),
        ),
      );
      // Every listed provider button is enabled — equal prominence.
      expect(button.onPressed, isNotNull, reason: '$label must be tappable');
      final rect = tester.getRect(find.text(label));
      xs.add(rect.top);
    }
    // Stacked vertically: strictly increasing tops.
    for (var i = 1; i < xs.length; i++) {
      expect(xs[i], greaterThan(xs[i - 1]));
    }
  });

  testWidgets('unconfigured providers explain instead of dead-ending', (
    tester,
  ) async {
    final svc = _svc();
    await svc.ready;
    await tester.pumpWidget(_app(svc));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Continue with Apple'));
    await tester.pumpAndSettle();

    expect(
      svc.lastError,
      "Sign in with Apple isn't available yet — use Google for now.",
    );
  });

  testWidgets('Google remains the working sign-in path', (tester) async {
    var calls = 0;
    final svc = _svc(
      googleSignIn: () async {
        calls++;
        return null; // user cancels — no session exchange needed
      },
    );
    await svc.ready;
    await tester.pumpWidget(_app(svc));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Continue with Google'));
    await tester.pumpAndSettle();

    expect(calls, 1);
    expect(svc.lastError, 'Sign in cancelled');
  });
}
