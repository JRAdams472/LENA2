import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/auth/auth_service.dart';

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

String makeToken(Map<String, dynamic> claims) {
  String b64(Map<String, dynamic> m) =>
      base64Url.encode(utf8.encode(json.encode(m))).replaceAll('=', '');
  return '${b64({'alg': 'none'})}.${b64(claims)}.sig';
}

final _future = DateTime.now().millisecondsSinceEpoch ~/ 1000 + 3600;
final _past = DateTime.now().millisecondsSinceEpoch ~/ 1000 - 3600;

String get liveAccess => makeToken({'iss': 'lena', 'sub': '7', 'exp': _future});
String get expiredAccess =>
    makeToken({'iss': 'lena', 'sub': '7', 'exp': _past});
String get googleToken => makeToken({
      'iss': 'https://accounts.google.com',
      'email': 'm@example.com',
      'exp': _future,
    });

Map<String, dynamic> bundle(String access, String refresh) => {
      'accessToken': access,
      'refreshToken': refresh,
      'expiresAt':
          DateTime.now().add(const Duration(days: 30)).toIso8601String(),
    };

AuthService makeService(
  MemStorage storage,
  http.Response Function(http.Request) handler, {
  Future<String?> Function()? googleSignIn,
  List<http.Request>? sink,
}) {
  final client = http_testing.MockClient((req) async {
    sink?.add(req);
    return handler(req);
  });
  return AuthService.test(
    storage: storage,
    httpClient: client,
    googleSignIn: googleSignIn,
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('sign-in exchanges the Google credential for a session', () async {
    final storage = MemStorage();
    final reqs = <http.Request>[];
    final svc = makeService(storage, (req) {
      if (req.url.path == '/auth/session') {
        return http.Response(json.encode(bundle(liveAccess, 'rt-1')), 200);
      }
      return http.Response('not found', 404);
    }, googleSignIn: () async => googleToken, sink: reqs);
    await svc.ready;

    await svc.signIn();

    expect(svc.isSignedIn, isTrue);
    expect(svc.idToken, liveAccess);
    expect(storage.map['id_token'], liveAccess);
    expect(storage.map['refresh_token'], 'rt-1');
    final create = reqs.singleWhere((r) => r.url.path == '/auth/session');
    expect(create.headers['Authorization'], 'Bearer $googleToken');
  });

  test('sign-in falls back to the Google token when sessions fail', () async {
    final storage = MemStorage();
    final svc = makeService(
      storage,
      (req) => http.Response('unavailable', 503),
      googleSignIn: () async => googleToken,
    );
    await svc.ready;

    await svc.signIn();

    expect(svc.isSignedIn, isTrue);
    expect(svc.idToken, googleToken);
    expect(storage.map.containsKey('refresh_token'), isFalse);
  });

  test('init restores an expired session via the refresh token', () async {
    final storage = MemStorage()
      ..map['id_token'] = expiredAccess
      ..map['refresh_token'] = 'rt-old';
    final svc = makeService(storage, (req) {
      if (req.url.path == '/auth/session/refresh') {
        return http.Response(json.encode(bundle(liveAccess, 'rt-new')), 200);
      }
      return http.Response('not found', 404);
    });
    await svc.ready;

    expect(svc.isSignedIn, isTrue);
    expect(svc.idToken, liveAccess);
    expect(storage.map['refresh_token'], 'rt-new');
  });

  test('refresh rotation replaces both stored tokens', () async {
    final storage = MemStorage()
      ..map['id_token'] = expiredAccess
      ..map['refresh_token'] = 'rt-old';
    final svc = makeService(storage, (req) {
      return http.Response(json.encode(bundle(liveAccess, 'rt-new')), 200);
    });
    await svc.ready;

    final token = await svc.getValidToken();
    expect(token, liveAccess);
    expect(storage.map['refresh_token'], 'rt-new');
  });

  test('concurrent refreshes share one rotation', () async {
    final storage = MemStorage()
      ..map['id_token'] = expiredAccess
      ..map['refresh_token'] = 'rt-old';
    var refreshCalls = 0;
    final svc = makeService(storage, (req) {
      if (req.url.path == '/auth/session/refresh') refreshCalls++;
      return http.Response(json.encode(bundle(liveAccess, 'rt-new')), 200);
    });
    await svc.ready;
    refreshCalls = 0;

    final results =
        await Future.wait([svc.refreshSession(), svc.refreshSession()]);
    expect(results, [true, true]);
    expect(refreshCalls, 1);
  });

  test('a rejected refresh clears both tokens and signs out', () async {
    final storage = MemStorage()
      ..map['id_token'] = expiredAccess
      ..map['refresh_token'] = 'rt-dead';
    final svc = makeService(
      storage,
      (req) => http.Response('{"message":"invalid session"}', 401),
    );
    await svc.ready;

    expect(svc.isSignedIn, isFalse);
    expect(svc.idToken, isNull);
    expect(storage.map, isEmpty);
  });

  test('transport failure during refresh keeps tokens for retry', () async {
    final storage = MemStorage()
      ..map['id_token'] = expiredAccess
      ..map['refresh_token'] = 'rt-keep';
    final svc = AuthService.test(
      storage: storage,
      httpClient: http_testing.MockClient(
        (req) async => throw Exception('offline'),
      ),
    );
    await svc.ready;

    expect(svc.isSignedIn, isTrue, reason: 'refresh token survives a blip');
    expect(storage.map['refresh_token'], 'rt-keep');
  });

  test('sign-out revokes the refresh token and clears storage', () async {
    final storage = MemStorage()
      ..map['id_token'] = liveAccess
      ..map['refresh_token'] = 'rt-bye';
    final reqs = <http.Request>[];
    var googleSignedOut = false;
    final svc = AuthService.test(
      storage: storage,
      httpClient: http_testing.MockClient((req) async {
        reqs.add(req);
        return http.Response('', 204);
      }),
      googleSignOut: () async {
        googleSignedOut = true;
      },
    );
    await svc.ready;

    await svc.signOut();

    final revoke = reqs.singleWhere(
      (r) => r.url.path == '/auth/session/revoke',
    );
    expect(json.decode(revoke.body)['refreshToken'], 'rt-bye');
    expect(storage.map, isEmpty);
    expect(svc.isSignedIn, isFalse);
    expect(googleSignedOut, isTrue);
  });
}
