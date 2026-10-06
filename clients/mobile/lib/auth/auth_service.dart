import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:http/http.dart' as http;

const _tokenKey = 'id_token';
const _refreshKey = 'refresh_token';
// Debug builds only: seeds the bearer directly so emulators can reach
// the app for screenshots/e2e without Play Services Google sign-in.
// kDebugMode makes the read dead code in release builds.
const _debugIdToken = String.fromEnvironment('LENA_DEBUG_ID_TOKEN');
const _serverClientId = String.fromEnvironment('LENA_GOOGLE_SERVER_CLIENT_ID');
const _iosClientId = String.fromEnvironment('LENA_GOOGLE_IOS_CLIENT_ID');

/// GraphQL endpoint the app talks to. Session endpoints share the same
/// origin under /auth/session*.
const String lenaApiUrl = String.fromEnvironment(
  'LENA_API_URL',
  defaultValue: 'http://localhost:8080/graphql',
);

Uri _sessionUrl(String path) {
  final base = Uri.parse(lenaApiUrl);
  return base.replace(path: path, query: '', fragment: '');
}

/// Narrow storage seam so tests can substitute an in-memory map for the
/// platform keychain.
abstract class TokenStorage {
  Future<String?> read(String key);
  Future<void> write(String key, String? value);
  Future<void> delete(String key);
}

class _SecureTokenStorage implements TokenStorage {
  final _inner = const FlutterSecureStorage();
  @override
  Future<String?> read(String key) => _inner.read(key: key);
  @override
  Future<void> write(String key, String? value) =>
      _inner.write(key: key, value: value);
  @override
  Future<void> delete(String key) => _inner.delete(key: key);
}

class AuthService extends ChangeNotifier {
  final TokenStorage _storage;
  final http.Client _http;
  // Google is injected as closures — the SDK types resist simple fakes.
  late final Future<String?> Function() _googleSignIn;
  late final Future<void> Function() _googleSignOut;

  String? _idToken; // the active bearer: LENA access token or Google ID token
  String? _refreshToken;
  bool _isLoading = true;
  String? _lastError;
  Future<bool>? _refreshFuture;
  late final Future<void> _initFuture;

  AuthService({TokenStorage? storage, http.Client? httpClient})
      : _storage = storage ?? _SecureTokenStorage(),
        _http = httpClient ?? http.Client() {
    final google = GoogleSignIn(
      scopes: ['email', 'profile'],
      serverClientId: _serverClientId.isEmpty ? null : _serverClientId,
      clientId: _iosClientId.isEmpty ? null : _iosClientId,
    );
    _googleSignIn = () async {
      final account = await google.signIn();
      return account == null ? null : (await account.authentication).idToken;
    };
    _googleSignOut = () => google.signOut().then((_) {});
    _initFuture = _init();
  }

  @visibleForTesting
  AuthService.test({
    required TokenStorage storage,
    required http.Client httpClient,
    Future<String?> Function()? googleSignIn,
    Future<void> Function()? googleSignOut,
  })  : _storage = storage,
        _http = httpClient,
        _googleSignIn = googleSignIn ?? (() async => null),
        _googleSignOut = googleSignOut ?? (() async {}) {
    _initFuture = _init();
  }

  /// Completes when initial storage restore (and any session refresh) is
  /// done — tests await this before asserting.
  @visibleForTesting
  Future<void> get ready => _initFuture;

  bool get isLoading => _isLoading;
  // A live refresh token counts as signed in — the first request rotates
  // it into a fresh access token.
  bool get isSignedIn =>
      _idToken != null && (!_isExpired || _refreshToken != null);
  String? get idToken => _idToken;
  String? get lastError => _lastError;

  /// Runs at the top of signOut while the bearer is still valid — the push
  /// service hooks this to unregister the device token before the session
  /// disappears. Best-effort; failures must not block logout.
  Future<void> Function()? onBeforeSignOut;

  bool get _isExpired {
    if (_idToken == null) return true;
    final exp = _extractExp(_idToken!);
    if (exp == null) return true;
    return DateTime.now().isAfter(
      DateTime.fromMillisecondsSinceEpoch(exp * 1000),
    );
  }

  static int? _extractExp(String token) {
    try {
      final parts = token.split('.');
      if (parts.length != 3) return null;
      final payload = base64Url.normalize(parts[1]);
      final decoded = utf8.decode(base64Url.decode(payload));
      final map = json.decode(decoded) as Map<String, dynamic>;
      return (map['exp'] as num?)?.toInt();
    } catch (_) {
      return null;
    }
  }

  Future<void> _init() async {
    try {
      _idToken = await _storage.read(_tokenKey);
      _refreshToken = await _storage.read(_refreshKey);
      if (kDebugMode && _debugIdToken.isNotEmpty) {
        _idToken = _debugIdToken;
        await _storage.write(_tokenKey, _debugIdToken);
      }
      // Expired or missing bearer with a stored refresh token: rotate now
      // so startup doesn't hit a 401.
      if (_refreshToken != null && (_idToken == null || _isExpired)) {
        await refreshSession();
      }
    } catch (e) {
      debugPrint('Failed to read stored token: $e');
    }
    _isLoading = false;
    notifyListeners();
  }

  /// Returns a usable bearer token, rotating the refresh token when the
  /// access token is expired. Null when no valid credential exists.
  Future<String?> getValidToken() async {
    if (_idToken != null && !_isExpired) return _idToken;
    if (await refreshSession()) return _idToken;
    return _idToken;
  }

  Future<void> signIn() async {
    _isLoading = true;
    _lastError = null;
    notifyListeners();
    try {
      // Debug builds seeded with LENA_DEBUG_ID_TOKEN (screenshot/e2e
      // runs) skip Google entirely — the button works on emulators
      // without Play Services.
      final googleToken = (kDebugMode && _debugIdToken.isNotEmpty)
          ? _debugIdToken
          : await _googleSignIn();
      if (googleToken == null) {
        _idToken = null;
        _lastError = 'Sign in cancelled';
      } else {
        await _exchangeForSession(googleToken);
      }
    } catch (e) {
      _idToken = null;
      _lastError = 'Sign in failed: $e';
      debugPrint(_lastError);
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  /// Trades the provider credential for a LENA session. On any failure the
  /// Google token stays as the bearer — OIDC-only deployments keep working.
  Future<void> _exchangeForSession(String googleToken) async {
    try {
      final res = await _http
          .post(
            _sessionUrl('/auth/session'),
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer $googleToken',
            },
            body: json.encode({'device': 'mobile'}),
          )
          .timeout(const Duration(seconds: 10));
      if (res.statusCode == 200) {
        final body = json.decode(res.body) as Map<String, dynamic>;
        _idToken = body['accessToken'] as String;
        _refreshToken = body['refreshToken'] as String;
        await _storage.write(_tokenKey, _idToken);
        await _storage.write(_refreshKey, _refreshToken);
        return;
      }
    } catch (e) {
      debugPrint('session exchange failed, using provider token: $e');
    }
    _idToken = googleToken;
    await _storage.write(_tokenKey, _idToken);
  }

  /// Rotates the stored refresh token into a new credential pair.
  /// Single-flighted — concurrent callers share one rotation. A 401/403
  /// means the family is dead: both tokens are dropped so the user is
  /// signed out. Transport failures keep the tokens for a later retry.
  Future<bool> refreshSession() {
    return _refreshFuture ??= _doRefresh().whenComplete(() {
      _refreshFuture = null;
    });
  }

  Future<bool> _doRefresh() async {
    final rt = _refreshToken;
    if (rt == null) return false;
    int status;
    try {
      final res = await _http
          .post(
            _sessionUrl('/auth/session/refresh'),
            headers: {'Content-Type': 'application/json'},
            body: json.encode({'refreshToken': rt, 'device': 'mobile'}),
          )
          .timeout(const Duration(seconds: 10));
      status = res.statusCode;
      if (status == 200) {
        final body = json.decode(res.body) as Map<String, dynamic>;
        _idToken = body['accessToken'] as String;
        _refreshToken = body['refreshToken'] as String;
        await _storage.write(_tokenKey, _idToken);
        await _storage.write(_refreshKey, _refreshToken);
        notifyListeners();
        return true;
      }
    } catch (e) {
      debugPrint('session refresh failed (transport): $e');
      return false;
    }
    if (status == 401 || status == 403) {
      await _clearTokens();
      notifyListeners();
    }
    return false;
  }

  Future<void> _clearTokens() async {
    _idToken = null;
    _refreshToken = null;
    try {
      await _storage.delete(_tokenKey);
      await _storage.delete(_refreshKey);
    } catch (_) {
      // best effort
    }
  }

  Future<void> signOut() async {
    final rt = _refreshToken;
    try {
      await onBeforeSignOut?.call();
    } catch (e) {
      debugPrint('pre-signout hook failed (continuing): $e');
    }
    await _clearTokens();
    _lastError = null;
    if (rt != null) {
      try {
        await _http
            .post(
              _sessionUrl('/auth/session/revoke'),
              headers: {'Content-Type': 'application/json'},
              body: json.encode({'refreshToken': rt}),
            )
            .timeout(const Duration(seconds: 10));
      } catch (_) {
        // best effort — the session expires regardless
      }
    }
    try {
      await _googleSignOut();
    } catch (_) {
      // ignore
    }
    notifyListeners();
  }
}

final AuthService authService = AuthService();
