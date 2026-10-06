import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/screens/notification_settings_screen.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

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

const _prefs = [
  {
    'category': '_all',
    'label': 'All notifications',
    'enabled': true,
    'pushEnabled': false,
    'mutedUntil': null,
  },
  {
    'category': 'household',
    'label': 'Household',
    'enabled': true,
    'pushEnabled': true,
    'mutedUntil': null,
  },
  {
    'category': 'expiring',
    'label': 'Expiring items',
    'enabled': false,
    'pushEnabled': false,
    'mutedUntil': null,
  },
];

Widget _app(_CaptureLink link) => GraphQLProvider(
      client: ValueNotifier(
        GraphQLClient(
          cache: GraphQLCache(),
          link: link,
          defaultPolicies: DefaultPolicies(
            query: Policies(fetch: FetchPolicy.noCache),
            mutate: Policies(fetch: FetchPolicy.noCache),
          ),
        ),
      ),
      child: const MaterialApp(home: NotificationSettingsScreen()),
    );

void main() {
  testWidgets(
    'renders a Push switch for every row and Feed for non-_all rows',
    (tester) async {
      final link = _CaptureLink({
        'NotificationPrefs': {'myNotificationPreferences': _prefs},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      // Three rows: _all (Push only), household + expiring (Push + Feed).
      expect(find.text('Push'), findsNWidgets(3));
      expect(find.text('Feed'), findsNWidgets(2));
      expect(find.byType(Switch), findsNWidgets(5));
    },
  );

  testWidgets(
    'the _all row shows only the Push switch and no Feed switch',
    (tester) async {
      final link = _CaptureLink({
        'NotificationPrefs': {'myNotificationPreferences': _prefs},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      // First row is _all: its card contains Push but no Feed label.
      final allRow = find.ancestor(
        of: find.text('All notifications (global mute)'),
        matching: find.byType(Card),
      );
      expect(
        find.descendant(of: allRow, matching: find.text('Push')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: allRow, matching: find.text('Feed')),
        findsNothing,
      );
    },
  );

  testWidgets(
    'tapping a Push switch sends setNotificationCategoryPushEnabled',
    (tester) async {
      final link = _CaptureLink({
        'NotificationPrefs': {'myNotificationPreferences': _prefs},
        'SetCategoryPushEnabled': {'setNotificationCategoryPushEnabled': true},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      // Switch order: [_all.push, household.push, household.feed,
      // expiring.push, expiring.feed]. Index 1 = household push (was on).
      await tester.tap(find.byType(Switch).at(1));
      await tester.pumpAndSettle();

      final calls = link.byName('SetCategoryPushEnabled');
      expect(calls, hasLength(1));
      expect(calls.single.variables['category'], 'household');
      expect(calls.single.variables['enabled'], isFalse);
    },
  );

  testWidgets(
    'tapping the _all Push switch targets the _all category',
    (tester) async {
      final link = _CaptureLink({
        'NotificationPrefs': {'myNotificationPreferences': _prefs},
        'SetCategoryPushEnabled': {'setNotificationCategoryPushEnabled': true},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(Switch).first);
      await tester.pumpAndSettle();

      final calls = link.byName('SetCategoryPushEnabled');
      expect(calls, hasLength(1));
      expect(calls.single.variables['category'], '_all');
      expect(calls.single.variables['enabled'], isTrue);
    },
  );

  testWidgets(
    'tapping a Feed switch sends setNotificationCategoryEnabled',
    (tester) async {
      final link = _CaptureLink({
        'NotificationPrefs': {'myNotificationPreferences': _prefs},
        'SetCategoryEnabled': {'setNotificationCategoryEnabled': true},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      // Index 4 = expiring feed (was off) → toggles on.
      await tester.tap(find.byType(Switch).at(4));
      await tester.pumpAndSettle();

      final calls = link.byName('SetCategoryEnabled');
      expect(calls, hasLength(1));
      expect(calls.single.variables['category'], 'expiring');
      expect(calls.single.variables['enabled'], isTrue);
    },
  );

  testWidgets(
    'a failed push mutation surfaces the server message',
    (tester) async {
      final link = _CaptureLink(
        {
          'NotificationPrefs': {'myNotificationPreferences': _prefs},
        },
        errors: {
          'SetCategoryPushEnabled': [
            const GraphQLError(message: 'push disabled upstream'),
          ],
        },
      );
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(Switch).at(1));
      await tester.pumpAndSettle();

      expect(find.text('push disabled upstream'), findsOneWidget);
    },
  );
}
