import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/auth/auth_gate.dart';
import 'package:lena_mobile/auth/auth_service.dart';
import 'package:lena_mobile/screens/approval_gate_screen.dart';
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

Widget buildApp(AuthService svc) {
  return ChangeNotifierProvider<AuthService>.value(
    value: svc,
    child: const MaterialApp(home: AuthGate()),
  );
}

AuthService serviceWithGate(String kind) {
  final svc = AuthService.test(
    storage: MemStorage(),
    httpClient: http_testing.MockClient((req) async => http.Response('', 404)),
  );
  svc.flagAccountGate(kind);
  return svc;
}

void main() {
  testWidgets('pending accounts see the access-request screen', (
    tester,
  ) async {
    final svc = serviceWithGate('pending');
    await svc.ready;

    await tester.pumpWidget(buildApp(svc));
    await tester.pumpAndSettle();

    expect(find.byType(ApprovalGateScreen), findsOneWidget);
    expect(find.text('Access request sent'), findsOneWidget);
    expect(find.text('Sign out'), findsOneWidget);
  });

  testWidgets('denied accounts see the denied screen', (tester) async {
    final svc = serviceWithGate('denied');
    await svc.ready;

    await tester.pumpWidget(buildApp(svc));
    await tester.pumpAndSettle();

    expect(find.byType(ApprovalGateScreen), findsOneWidget);
    expect(find.text('Access not granted'), findsOneWidget);
  });
}
