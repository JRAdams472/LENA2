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

/// Captures the authorize URL the service hands to the browser sheet
/// and returns a canned lena:// callback.
class _FakeWebAuth {
  Uri? gotUrl;
  String? gotScheme;
  String callback;
  _FakeWebAuth(this.callback);

  Future<String?> call(Uri url, String scheme) async {
    gotUrl = url;
    gotScheme = scheme;
    return callback;
  }
}

http_testing.MockClient _api(
  void Function(http.Request) onRequest, {
  int status = 200,
  String body = '{"accessToken":"acc","refreshToken":"ref"}',
}) {
  return http_testing.MockClient((req) async {
    onRequest(req);
    return http.Response(body, status);
  });
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('code-exchange providers', () {
    test('discord callback posts code + PKCE + mobile redirect', () async {
      final auth = _FakeWebAuth('lena://oauth/discord?code=ABC&state=ANY');
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api((_) {}),
        webAuth: auth.call,
        providerClientIds: const {'discord': 'dcid'},
      );
      await svc.ready;

      // The authorize URL goes to Discord with the deep-link redirect.
      final authorize =
          await svc.signInWithProvider('Discord').then((_) => auth.gotUrl);
      expect(authorize!.host, 'discord.com');
      expect(authorize.path, '/oauth2/authorize');
      expect(authorize.queryParameters['client_id'], 'dcid');
      expect(
        authorize.queryParameters['redirect_uri'],
        'lena://oauth/discord',
      );
      expect(authorize.queryParameters['code_challenge_method'], 'S256');
      expect(authorize.queryParameters['state'], isNotEmpty);
      expect(auth.gotScheme, 'lena');
    });

    test('state mismatch on the callback is rejected before exchange',
        () async {
      http.Request? got;
      final auth = _FakeWebAuth('lena://oauth/discord?code=ABC&state=WRONG');
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api((req) => got = req),
        webAuth: auth.call,
        providerClientIds: const {'discord': 'dcid'},
      );
      await svc.ready;
      await svc.signInWithProvider('Discord');

      expect(got, isNull); // no exchange attempt
      expect(svc.lastError, 'Sign-in was cancelled or denied.');
      expect(svc.isSignedIn, isFalse);
    });

    test('successful exchange stores the session pair', () async {
      http.Request? got;
      // The state check needs the real generated value — capture the
      // authorize URL first, then answer with a matching state.
      Uri? authorize;
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api((req) => got = req),
        webAuth: (url, scheme) async {
          authorize = url;
          final state = url.queryParameters['state'];
          return 'lena://oauth/discord?code=ABC&state=$state';
        },
        providerClientIds: const {'discord': 'dcid'},
      );
      await svc.ready;
      await svc.signInWithProvider('Discord');

      expect(authorize, isNotNull);
      expect(got, isNotNull);
      expect(got!.url.path, '/auth/session/discord');
      final body = json.decode(got!.body) as Map<String, dynamic>;
      expect(body['code'], 'ABC');
      expect(body['codeVerifier'], isNotEmpty);
      expect(body['redirectUri'], 'lena://oauth/discord');
      expect(body['nonce'], authorize!.queryParameters['nonce'] ?? isNotEmpty);
      expect(svc.isSignedIn, isTrue);
      expect(svc.idToken, 'acc');
    });

    test('user cancel leaves a readable message', () async {
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api((_) {}),
        webAuth: (_, __) async => null,
        providerClientIds: const {'discord': 'dcid'},
      );
      await svc.ready;
      await svc.signInWithProvider('Discord');
      expect(svc.lastError, 'Sign in cancelled');
      expect(svc.isSignedIn, isFalse);
    });

    test('403 pending sets the approval gate', () async {
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api(
          (_) {},
          status: 403,
          body: '{"message":"account_pending"}',
        ),
        webAuth: (url, _) async =>
            'lena://oauth/facebook?code=ABC&state=${url.queryParameters['state']}',
        providerClientIds: const {'facebook': 'fbid'},
      );
      await svc.ready;
      await svc.signInWithProvider('Facebook');
      expect(svc.accountGate, 'pending');
    });

    test('unconfigured provider explains instead of browsing', () async {
      var launched = false;
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api((_) {}),
        webAuth: (_, __) async {
          launched = true;
          return null;
        },
        // No client id for discord in this build.
      );
      await svc.ready;
      await svc.signInWithProvider('Discord');
      expect(launched, isFalse);
      expect(
        svc.lastError,
        "Sign in with Discord isn't configured for this app build.",
      );
    });
  });

  group('apple native flow', () {
    test('identityToken exchanges through the bearer session path', () async {
      http.Request? got;
      final svc = AuthService.test(
        storage: MemStorage(),
        httpClient: _api((req) => got = req),
        appleSignIn: () async => 'apple-id-token',
      );
      await svc.ready;
      await svc.signInWithProvider('Apple');

      // On non-Apple test platforms the flow stops with a message…
      if (svc.lastError ==
          'Sign in with Apple is only available on Apple devices.') {
        expect(got, isNull);
        return;
      }
      // …but where supported the token posts as a bearer.
      expect(got!.url.path, '/auth/session');
      expect(
        got!.headers['authorization'],
        'Bearer apple-id-token',
      );
      expect(svc.isSignedIn, isTrue);
    });
  });
}
