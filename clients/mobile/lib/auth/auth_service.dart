import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:google_sign_in/google_sign_in.dart';

const _tokenKey = 'id_token';
const _serverClientId = String.fromEnvironment('LENA_GOOGLE_SERVER_CLIENT_ID');
const _iosClientId = String.fromEnvironment('LENA_GOOGLE_IOS_CLIENT_ID');

class AuthService extends ChangeNotifier {
  final _storage = const FlutterSecureStorage();
  final _google = GoogleSignIn(
    scopes: ['email', 'profile'],
    serverClientId: _serverClientId.isEmpty ? null : _serverClientId,
    clientId: _iosClientId.isEmpty ? null : _iosClientId,
  );

  String? _idToken;
  bool _isLoading = true;
  String? _lastError;

  AuthService() {
    _init();
  }

  bool get isLoading => _isLoading;
  bool get isSignedIn => _idToken != null && !_isExpired;
  String? get idToken => _idToken;
  String? get lastError => _lastError;

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
    _idToken = await _storage.read(key: _tokenKey);
    _isLoading = false;
    notifyListeners();
  }

  Future<void> signIn() async {
    _isLoading = true;
    _lastError = null;
    notifyListeners();
    try {
      final account = await _google.signIn();
      if (account == null) {
        _idToken = null;
        _lastError = 'Sign in cancelled';
      } else {
        final auth = await account.authentication;
        _idToken = auth.idToken;
        if (_idToken != null) {
          await _storage.write(key: _tokenKey, value: _idToken);
        } else {
          _lastError = 'No ID token returned';
        }
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

  Future<void> signOut() async {
    _idToken = null;
    _lastError = null;
    await _storage.delete(key: _tokenKey);
    try {
      await _google.signOut();
    } catch (_) {
      // ignore
    }
    notifyListeners();
  }
}

final AuthService authService = AuthService();
