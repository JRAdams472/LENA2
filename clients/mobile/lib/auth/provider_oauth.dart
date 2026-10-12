import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';

/// Provider OAuth plumbing for the non-Google buttons (LEN-136 P2).
///
/// Discord, Microsoft, and Facebook run the standard authorization-code
/// flow: the app opens the provider's authorize page in a system browser
/// sheet (flutter_web_auth_2 → ASWebAuthenticationSession on iOS,
/// Custom Tabs on Android) with PKCE, the provider redirects back on
/// the `lena://` deep-link scheme, and the BFF exchanges the code
/// server-side — client secrets never ship in the app. Apple on iOS
/// uses the native Sign in with Apple sheet and needs none of this.

const _discordClientId = String.fromEnvironment('LENA_DISCORD_CLIENT_ID');
const _microsoftClientId = String.fromEnvironment('LENA_MICROSOFT_CLIENT_ID');
const _microsoftTenant = String.fromEnvironment(
  'LENA_MICROSOFT_TENANT',
  defaultValue: 'consumers',
);
const _facebookClientId = String.fromEnvironment('LENA_FACEBOOK_CLIENT_ID');

/// Deep-link scheme the OS routes back into the app — registered in the
/// iOS URL types and the Android manifest intent-filter, and mirrored in
/// the server's LENA_*_MOBILE_REDIRECT_URI allowlist.
const oauthCallbackScheme = 'lena';

/// The deep link a provider redirects to after authorization. Must match
/// a URI registered in the provider's console and allowlisted on the
/// server (LENA_<PROVIDER>_MOBILE_REDIRECT_URI).
String mobileRedirectUri(String provider) =>
    '$oauthCallbackScheme://oauth/$provider';

/// Resolves a provider's OAuth client id — from `overrides` when tests
/// supply them, else the compile-time dart-define.
String? _clientId(String provider, Map<String, String>? overrides) {
  if (overrides != null && overrides.containsKey(provider)) {
    return overrides[provider];
  }
  return switch (provider) {
    'discord' => _discordClientId,
    'microsoft' => _microsoftClientId,
    'facebook' => _facebookClientId,
    _ => null,
  };
}

/// Whether a code-exchange provider has a client id compiled in —
/// unconfigured providers get a clear message instead of a dead flow.
bool providerConfigured(String provider, [Map<String, String>? overrides]) =>
    (_clientId(provider, overrides) ?? '').isNotEmpty;

String _b64url(List<int> bytes) => base64Url.encode(bytes).replaceAll('=', '');

/// RFC 7636 PKCE verifier — 32 bytes of entropy as base64url.
String generatePkceVerifier() {
  final rng = Random.secure();
  return _b64url(List<int>.generate(32, (_) => rng.nextInt(256)));
}

/// The S256 challenge derived from a verifier — sent on the authorize
/// request; the verifier itself goes only to the BFF at exchange time.
String derivePkceChallenge(String verifier) =>
    _b64url(sha256.convert(utf8.encode(verifier)).bytes);

/// 128-bit random token for the authorize-request state/nonce values.
String generateOAuthToken() {
  final rng = Random.secure();
  return _b64url(List<int>.generate(16, (_) => rng.nextInt(256)));
}

/// Builds the provider's authorize URL — same parameters as the web
/// oauth.ts flow, with the mobile deep link as redirect_uri. Returns
/// null for unknown or unconfigured providers.
Uri? providerAuthorizeUrl(
  String provider,
  String state,
  String nonce,
  String challenge, {
  Map<String, String>? clientIds,
}) {
  final clientId = _clientId(provider, clientIds);
  if (clientId == null || clientId.isEmpty) return null;
  final redirect = mobileRedirectUri(provider);
  final pkce = {'code_challenge': challenge, 'code_challenge_method': 'S256'};
  switch (provider) {
    case 'discord':
      return Uri.https('discord.com', '/oauth2/authorize', {
        'client_id': clientId,
        'response_type': 'code',
        'redirect_uri': redirect,
        'scope': 'identify email',
        'state': state,
        ...pkce,
      });
    case 'microsoft':
      return Uri.https(
        'login.microsoftonline.com',
        '/$_microsoftTenant/oauth2/v2.0/authorize',
        {
          'client_id': clientId,
          'response_type': 'code',
          'response_mode': 'query',
          'redirect_uri': redirect,
          'scope': 'openid profile email',
          'state': state,
          'nonce': nonce,
          ...pkce,
        },
      );
    case 'facebook':
      return Uri.https('www.facebook.com', '/v21.0/dialog/oauth', {
        'client_id': clientId,
        'response_type': 'code',
        'redirect_uri': redirect,
        'scope': 'openid email',
        'state': state,
        'nonce': nonce,
        ...pkce,
      });
    default:
      return null;
  }
}
