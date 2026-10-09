import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/push/push_service.dart';
import 'package:lena_mobile/screens/edit_recipe_screen.dart';
import 'package:lena_mobile/screens/household_screen.dart';
import 'package:shared_preferences/shared_preferences.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

/// Records outgoing operations and answers with canned data (or errors)
/// keyed by operation name — same pattern as analytics_screens_test.
class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, Map<String, dynamic>> responses;
  final Map<String, List<GraphQLError>> errors;

  _CaptureLink(this.responses, {this.errors = const {}});

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    yield Response(
      data: responses[_opName(request)] ?? <String, dynamic>{},
      errors: errors[_opName(request)],
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

GraphQLClient _client(_CaptureLink link) => GraphQLClient(
      cache: GraphQLCache(),
      link: link,
      defaultPolicies: DefaultPolicies(
        query: Policies(fetch: FetchPolicy.noCache),
        mutate: Policies(fetch: FetchPolicy.noCache),
      ),
    );

class _FakeMessaging implements MessagingClient {
  String? token = 'tok-1';
  int permissionRequests = 0;
  PushEnvelope? initial;
  final refresh = StreamController<String>.broadcast();
  final foreground = StreamController<PushEnvelope>.broadcast();
  final opened = StreamController<PushEnvelope>.broadcast();

  @override
  Future<void> requestPermission() async => permissionRequests++;

  @override
  Future<String?> getToken() async => token;

  @override
  Stream<String> get onTokenRefresh => refresh.stream;

  @override
  Stream<PushEnvelope> get onMessage => foreground.stream;

  @override
  Stream<PushEnvelope> get onMessageOpenedApp => opened.stream;

  @override
  Future<PushEnvelope?> getInitialMessage() async => initial;

  Future<void> close() async {
    await refresh.close();
    await foreground.close();
    await opened.close();
  }
}

class _FakeNotifier implements HeadsUpNotifier {
  final List<PushEnvelope> shown = [];

  @override
  Future<void> initialize() async {}

  @override
  Future<void> show(PushEnvelope envelope) async => shown.add(envelope);
}

Future<SharedPreferences> _prefs() async {
  SharedPreferences.setMockInitialValues({});
  return SharedPreferences.getInstance();
}

PushService _service(
  _FakeMessaging messaging,
  _FakeNotifier notifier,
  SharedPreferences prefs,
  _CaptureLink link, {
  GlobalKey<NavigatorState>? navigatorKey,
}) =>
    PushService(
      messaging: messaging,
      notifier: notifier,
      prefs: prefs,
      navigatorKey: navigatorKey,
      client: _client(link),
    );

void main() {
  test('start requests permission and registers the token', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({
      'RegisterDeviceToken': {'registerDeviceToken': true},
    });
    final svc = _service(messaging, _FakeNotifier(), prefs, link);

    await svc.start();

    expect(messaging.permissionRequests, 1);
    final regs = link.byName('RegisterDeviceToken');
    expect(regs, hasLength(1));
    expect(regs.single.variables['token'], 'tok-1');
    expect(regs.single.variables['platform'], 'android');
    expect(prefs.getString('push.registered_token'), 'tok-1');
    await messaging.close();
  });

  test('token refresh re-registers only when the token changed', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({
      'RegisterDeviceToken': {'registerDeviceToken': true},
    });
    final svc = _service(messaging, _FakeNotifier(), prefs, link);
    await svc.start();

    // Same token still valid — the refresh event alone must not re-send.
    messaging.refresh.add('tok-1');
    await pumpEventQueue();
    expect(link.byName('RegisterDeviceToken'), hasLength(1));

    messaging.token = 'tok-2';
    messaging.refresh.add('tok-2');
    await pumpEventQueue();
    final regs = link.byName('RegisterDeviceToken');
    expect(regs, hasLength(2));
    expect(regs.last.variables['token'], 'tok-2');
    await messaging.close();
  });

  test('failed registration is not deduplicated — next refresh retries',
      () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink(
      {},
      errors: {
        'RegisterDeviceToken': [const GraphQLError(message: 'down')],
      },
    );
    final svc = _service(messaging, _FakeNotifier(), prefs, link);
    await svc.start();

    expect(link.byName('RegisterDeviceToken'), hasLength(1));
    expect(prefs.getString('push.registered_token'), isNull);

    messaging.refresh.add('tok-1');
    await pumpEventQueue();
    expect(link.byName('RegisterDeviceToken'), hasLength(2));
    await messaging.close();
  });

  test('unregister revokes the stored token and clears it', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({
      'RegisterDeviceToken': {'registerDeviceToken': true},
      'UnregisterDeviceToken': {'unregisterDeviceToken': true},
    });
    final svc = _service(messaging, _FakeNotifier(), prefs, link);
    await svc.start();
    await svc.unregister();

    final unregs = link.byName('UnregisterDeviceToken');
    expect(unregs, hasLength(1));
    expect(unregs.single.variables['token'], 'tok-1');
    expect(prefs.getString('push.registered_token'), isNull);
    await messaging.close();
  });

  test('unregister without a registered token sends nothing', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({});
    final svc = _service(messaging, _FakeNotifier(), prefs, link);

    await svc.unregister();

    expect(link.byName('UnregisterDeviceToken'), isEmpty);
    await messaging.close();
  });

  test('foreground push shows a heads-up and fires the badge callback',
      () async {
    final messaging = _FakeMessaging();
    final notifier = _FakeNotifier();
    final prefs = await _prefs();
    final svc = _service(messaging, notifier, prefs, _CaptureLink({}));
    await svc.start();

    var bumped = false;
    svc.onPushReceived = () => bumped = true;
    messaging.foreground.add(
      const PushEnvelope(title: 'LENA', body: 'Milk expires tomorrow'),
    );
    await pumpEventQueue();

    expect(notifier.shown, hasLength(1));
    expect(notifier.shown.single.body, 'Milk expires tomorrow');
    expect(bumped, isTrue);
    await messaging.close();
  });

  test('a service without messaging is a disabled no-op', () async {
    final prefs = await _prefs();
    final link = _CaptureLink({});
    final svc = PushService(prefs: prefs, client: _client(link));

    expect(svc.isEnabled, isFalse);
    await svc.start();
    await svc.unregister();

    expect(link.requests, isEmpty);
  });

  testWidgets('opened-app push navigates to the linked screen', (tester) async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final navKey = GlobalKey<NavigatorState>();
    final link = _CaptureLink({});
    final svc = _service(
      messaging,
      _FakeNotifier(),
      prefs,
      link,
      navigatorKey: navKey,
    );

    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(_client(link)),
        child: MaterialApp(
          navigatorKey: navKey,
          home: const Scaffold(body: Text('home')),
        ),
      ),
    );
    await svc.start();

    messaging.opened.add(const PushEnvelope(data: {'recipeId': 'r-42'}));
    // Stream delivery is a microtask in the fake-async zone; pump flushes
    // it and builds the pushed route (destinations poll, so never settle).
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(EditRecipeScreen), findsOneWidget);
    await messaging.close();
  });

  testWidgets('push tap with no link fields lands on the household screen',
      (tester) async {
    final messaging = _FakeMessaging()
      ..initial = const PushEnvelope(data: {'kind': 'member_joined'});
    final prefs = await _prefs();
    final navKey = GlobalKey<NavigatorState>();
    final link = _CaptureLink({});
    final svc = _service(
      messaging,
      _FakeNotifier(),
      prefs,
      link,
      navigatorKey: navKey,
    );

    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(_client(link)),
        child: MaterialApp(
          navigatorKey: navKey,
          home: const Scaffold(body: Text('home')),
        ),
      ),
    );
    await svc.start();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(HouseholdScreen), findsOneWidget);
    await messaging.close();
  });

  test('create() falls back to a disabled service without Firebase', () async {
    // No google-services.json in tests → Firebase.initializeApp throws and
    // the app keeps running with push disabled rather than crashing.
    final prefs = await _prefs();
    final svc = await PushService.create(prefs: prefs);

    expect(svc.isEnabled, isFalse);
    await svc.start(); // no-op
    await svc.unregister();
    await svc.dispose();
  });

  test('a messaging-backed service reports enabled', () async {
    final svc = _service(
        _FakeMessaging(), _FakeNotifier(), await _prefs(), _CaptureLink({}));
    expect(svc.isEnabled, isTrue);
  });

  test('start is idempotent — a second call does not re-register', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({
      'RegisterDeviceToken': {'registerDeviceToken': true},
    });
    final svc = _service(messaging, _FakeNotifier(), prefs, link);

    await svc.start();
    await svc.start();

    expect(messaging.permissionRequests, 1);
    expect(link.byName('RegisterDeviceToken'), hasLength(1));
    await messaging.close();
  });

  test('a null device token registers nothing', () async {
    final messaging = _FakeMessaging()..token = null;
    final prefs = await _prefs();
    final link = _CaptureLink({});
    final svc = _service(messaging, _FakeNotifier(), prefs, link);

    await svc.start();

    expect(messaging.permissionRequests, 1);
    expect(link.byName('RegisterDeviceToken'), isEmpty);
    expect(prefs.getString('push.registered_token'), isNull);
    await messaging.close();
  });

  test('unregister swallows mutation errors but still clears the token',
      () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink(
      {
        'RegisterDeviceToken': {'registerDeviceToken': true},
      },
      errors: {
        'UnregisterDeviceToken': [const GraphQLError(message: 'down')],
      },
    );
    final svc = _service(messaging, _FakeNotifier(), prefs, link);
    await svc.start();
    expect(prefs.getString('push.registered_token'), 'tok-1');

    await svc.unregister();

    expect(link.byName('UnregisterDeviceToken'), hasLength(1));
    expect(prefs.getString('push.registered_token'), isNull);
    await messaging.close();
  });

  test('start after unregister re-registers the token', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({
      'RegisterDeviceToken': {'registerDeviceToken': true},
      'UnregisterDeviceToken': {'unregisterDeviceToken': true},
    });
    final svc = _service(messaging, _FakeNotifier(), prefs, link);

    await svc.start();
    await svc.unregister();
    await svc.start();

    expect(messaging.permissionRequests, 2);
    expect(link.byName('RegisterDeviceToken'), hasLength(2));
    await messaging.close();
  });

  test('dispose cancels the stream subscriptions', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final link = _CaptureLink({
      'RegisterDeviceToken': {'registerDeviceToken': true},
    });
    final svc = _service(messaging, _FakeNotifier(), prefs, link);
    await svc.start();
    await svc.dispose();

    // Events after dispose no longer reach the service.
    messaging.refresh.add('tok-2');
    await pumpEventQueue();
    expect(link.byName('RegisterDeviceToken'), hasLength(1));
    await messaging.close();
  });

  test('a push tap without a navigator is ignored', () async {
    final messaging = _FakeMessaging();
    final prefs = await _prefs();
    final svc = _service(messaging, _FakeNotifier(), prefs, _CaptureLink({}));
    await svc.start();

    // No navigatorKey — the envelope is dropped quietly.
    messaging.opened.add(const PushEnvelope(data: {'recipeId': 'r-1'}));
    await pumpEventQueue();
    await messaging.close();
  });

  test('PushEnvelope defaults to an empty payload', () {
    const e = PushEnvelope(title: 't', body: 'b');
    expect(e.title, 't');
    expect(e.body, 'b');
    expect(e.data, isEmpty);
  });
}
