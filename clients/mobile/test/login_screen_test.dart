import 'package:flutter/foundation.dart';
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

AuthService _svc({
  Future<String?> Function()? googleSignIn,
  Map<String, String>? providerClientIds,
}) {
  return AuthService.test(
    storage: MemStorage(),
    httpClient: http_testing.MockClient((req) async => http.Response('', 404)),
    googleSignIn: googleSignIn,
    providerClientIds: providerClientIds,
  );
}

Widget _app(AuthService svc) {
  return ChangeNotifierProvider<AuthService>.value(
    value: svc,
    child: const MaterialApp(home: LoginScreen()),
  );
}

/// Runs [body] with the target platform faked to iOS. The override must
/// reset inside the test body — invariant checks run before tearDowns.
Future<void> _asIos(Future<void> Function() body) async {
  debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
  try {
    await body();
  } finally {
    debugDefaultTargetPlatformOverride = null;
  }
}

void main() {
  testWidgets('iOS + configured providers shows the full button stack', (
    tester,
  ) async {
    final svc = _svc(
      providerClientIds: {'discord': 'd-id', 'facebook': 'f-id'},
    );
    await svc.ready;
    await _asIos(() async {
      await tester.pumpWidget(_app(svc));
      await tester.pumpAndSettle();
    });

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

  testWidgets('Android shows Google only — Apple hides, unconfigured '
      'providers hide', (tester) async {
    final svc = _svc(); // no provider client ids
    await svc.ready;
    await tester.pumpWidget(_app(svc));
    await tester.pumpAndSettle();

    expect(find.text('Continue with Google'), findsOneWidget);
    expect(find.text('Continue with Apple'), findsNothing);
    expect(find.text('Continue with Discord'), findsNothing);
    expect(find.text('Continue with Facebook'), findsNothing);
  });

  testWidgets('Android shows Discord only when configured', (tester) async {
    final svc = _svc(providerClientIds: {'discord': 'd-id'});
    await svc.ready;
    await tester.pumpWidget(_app(svc));
    await tester.pumpAndSettle();

    expect(find.text('Continue with Google'), findsOneWidget);
    expect(find.text('Continue with Discord'), findsOneWidget);
    expect(find.text('Continue with Apple'), findsNothing);
    expect(find.text('Continue with Facebook'), findsNothing);
  });

  testWidgets('visible buttons stack vertically at equal prominence', (
    tester,
  ) async {
    final svc = _svc(
      providerClientIds: {'discord': 'd-id', 'facebook': 'f-id'},
    );
    await svc.ready;
    await _asIos(() async {
      await tester.pumpWidget(_app(svc));
      await tester.pumpAndSettle();
    });

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
