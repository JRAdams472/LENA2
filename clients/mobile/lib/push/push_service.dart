import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../notification_links.dart';
import '../screens/household_screen.dart';

/// One push envelope, decoupled from firebase_messaging types so tests and
/// the heads-up notifier share one shape.
class PushEnvelope {
  const PushEnvelope({
    this.title,
    this.body,
    this.data = const {},
  });
  final String? title;
  final String? body;
  final Map<String, String> data;
}

/// The FCM surface PushService needs — narrow so tests can fake it without
/// Firebase (same seam style as TokenStorage/GoogleSignIn in auth_service).
abstract class MessagingClient {
  Future<void> requestPermission();
  Future<String?> getToken();
  Stream<String> get onTokenRefresh;
  Stream<PushEnvelope> get onMessage;
  Stream<PushEnvelope> get onMessageOpenedApp;
  Future<PushEnvelope?> getInitialMessage();
}

/// Production client over FirebaseMessaging.
class FirebaseMessagingClient implements MessagingClient {
  FirebaseMessagingClient(this._messaging);
  final FirebaseMessaging _messaging;

  static PushEnvelope _wrap(RemoteMessage m) => PushEnvelope(
        title: m.notification?.title,
        body: m.notification?.body,
        data: Map<String, String>.from(m.data),
      );

  @override
  Future<void> requestPermission() async {
    await _messaging.requestPermission();
  }

  @override
  Future<String?> getToken() => _messaging.getToken();

  @override
  Stream<String> get onTokenRefresh => _messaging.onTokenRefresh;

  @override
  Stream<PushEnvelope> get onMessage => FirebaseMessaging.onMessage.map(_wrap);

  @override
  Stream<PushEnvelope> get onMessageOpenedApp =>
      FirebaseMessaging.onMessageOpenedApp.map(_wrap);

  @override
  Future<PushEnvelope?> getInitialMessage() async {
    final m = await _messaging.getInitialMessage();
    return m == null ? null : _wrap(m);
  }
}

/// Foreground heads-up display — a local notification while the app is
/// visible (FCM doesn't show banners for onMessage). Narrow seam so tests
/// can observe shows without a real NotificationManager.
abstract class HeadsUpNotifier {
  Future<void> show(PushEnvelope envelope);
  Future<void> initialize();
}

/// Production notifier over flutter_local_notifications on the app's
/// default channel.
class LocalHeadsUpNotifier implements HeadsUpNotifier {
  LocalHeadsUpNotifier(this._plugin);
  final FlutterLocalNotificationsPlugin _plugin;

  static const _channel = AndroidNotificationChannel(
    'lena_default',
    'LENA notifications',
    description: 'Reminders and household activity',
    importance: Importance.high,
  );

  @override
  Future<void> initialize() async {
    await _plugin.initialize(
      settings: const InitializationSettings(
        android: AndroidInitializationSettings('@mipmap/ic_launcher'),
      ),
    );
    await _plugin
        .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin>()
        ?.createNotificationChannel(_channel);
  }

  @override
  Future<void> show(PushEnvelope e) async {
    await _plugin.show(
      id: e.hashCode & 0x7fffffff,
      title: e.title,
      body: e.body,
      notificationDetails: const NotificationDetails(
        android: AndroidNotificationDetails(
          'lena_default',
          'LENA notifications',
          importance: Importance.high,
          priority: Priority.high,
        ),
      ),
    );
  }
}

const _registeredTokenKey = 'push.registered_token';
const _platform = 'android';

/// Coordinates the push lifecycle: permission, token registration with the
/// BFF, token refresh, foreground heads-up display, tap routing, and
/// unregistration on sign-out.
class PushService {
  PushService({
    MessagingClient? messaging,
    HeadsUpNotifier? notifier,
    SharedPreferences? prefs,
    GlobalKey<NavigatorState>? navigatorKey,
    GraphQLClient? client,
  })  : _messaging = messaging,
        _notifier = notifier,
        _prefs = prefs,
        _navigatorKey = navigatorKey,
        _client = client;

  final MessagingClient? _messaging;
  final HeadsUpNotifier? _notifier;
  final SharedPreferences? _prefs;
  final GlobalKey<NavigatorState>? _navigatorKey;
  final GraphQLClient? _client;

  final List<StreamSubscription<dynamic>> _subs = [];
  bool _started = false;

  /// Invoked when a foreground push arrives so the unread badge can
  /// re-poll immediately rather than waiting for the interval tick.
  void Function()? onPushReceived;

  /// Creates the production service. Without google-services.json
  /// Firebase.initializeApp throws — the app keeps working with push
  /// simply disabled (dev builds, CI, e2e).
  static Future<PushService> create({
    GlobalKey<NavigatorState>? navigatorKey,
    GraphQLClient? client,
    SharedPreferences? prefs,
  }) async {
    try {
      await Firebase.initializeApp();
    } catch (e) {
      debugPrint('push disabled — Firebase init failed: $e');
      return PushService._disabled(
        navigatorKey: navigatorKey,
        client: client,
        prefs: prefs,
      );
    }
    final notifier = LocalHeadsUpNotifier(FlutterLocalNotificationsPlugin());
    await notifier.initialize();
    return PushService(
      messaging: FirebaseMessagingClient(FirebaseMessaging.instance),
      notifier: notifier,
      prefs: prefs ?? await SharedPreferences.getInstance(),
      navigatorKey: navigatorKey,
      client: client,
    );
  }

  PushService._disabled({
    GlobalKey<NavigatorState>? navigatorKey,
    GraphQLClient? client,
    SharedPreferences? prefs,
  })  : _messaging = null,
        _notifier = null,
        _prefs = prefs,
        _navigatorKey = navigatorKey,
        _client = client;

  bool get isEnabled => _messaging != null;

  /// Called once the user is signed in: asks for POST_NOTIFICATIONS
  /// (Android 13+), registers the device token, then listens for refreshes
  /// and taps.
  Future<void> start() async {
    final messaging = _messaging;
    if (messaging == null || _started) return;
    _started = true;
    await messaging.requestPermission();
    await _registerCurrentToken();
    _subs.add(messaging.onTokenRefresh.listen((_) => _registerCurrentToken()));
    _subs.add(messaging.onMessage.listen(_onForeground));
    _subs.add(messaging.onMessageOpenedApp.listen(_onTap));
    final initial = await messaging.getInitialMessage();
    if (initial != null) _onTap(initial);
  }

  /// Unregisters the stored token — called before auth tokens are cleared
  /// on sign-out. Best-effort: a failed call leaves the server-side token
  /// to expire naturally rather than blocking logout.
  Future<void> unregister() async {
    final token = _prefs?.getString(_registeredTokenKey);
    for (final s in _subs) {
      await s.cancel();
    }
    _subs.clear();
    _started = false;
    if (token == null) return;
    try {
      await _mutate(unregisterMutation, {'token': token});
    } catch (e) {
      debugPrint('push token unregister failed (best effort): $e');
    }
    await _prefs?.remove(_registeredTokenKey);
  }

  Future<void> _registerCurrentToken() async {
    final token = await _messaging?.getToken();
    if (token == null) return;
    if (_prefs?.getString(_registeredTokenKey) == token) return;
    try {
      final ok = await _mutate(registerMutation, {
        'token': token,
        'platform': _platform,
      });
      if (ok) await _prefs?.setString(_registeredTokenKey, token);
    } catch (e) {
      debugPrint('push token register failed (will retry on next start): $e');
    }
  }

  void _onForeground(PushEnvelope e) {
    unawaited(_notifier?.show(e));
    onPushReceived?.call();
  }

  void _onTap(PushEnvelope e) {
    final nav = _navigatorKey?.currentState;
    if (nav == null) return;
    final dest = notificationDestination(e.data) ?? const HouseholdScreen();
    nav.push(MaterialPageRoute(builder: (_) => dest));
  }

  /// Mutations go through the app's client so they carry the session
  /// bearer. Returns the mutation's Boolean payload.
  Future<bool> _mutate(String doc, Map<String, dynamic> vars) async {
    final client = _client;
    if (client == null) return false;
    final result = await client.mutate(
      MutationOptions(document: gql(doc), variables: vars),
    );
    if (result.hasException) {
      throw result.exception!;
    }
    return result.data?.values.first == true;
  }

  Future<void> dispose() async {
    for (final s in _subs) {
      await s.cancel();
    }
    _subs.clear();
  }
}

const registerMutation = r'''
  mutation RegisterDeviceToken($token: String!, $platform: String!) {
    registerDeviceToken(token: $token, platform: $platform)
  }
''';

const unregisterMutation = r'''
  mutation UnregisterDeviceToken($token: String!) {
    unregisterDeviceToken(token: $token)
  }
''';

/// The app-wide push instance. main() replaces this stub with the real
/// service before runApp; the default is a disabled no-op so tests and
/// widget trees that never run main() still work.
PushService pushService = PushService._disabled();
