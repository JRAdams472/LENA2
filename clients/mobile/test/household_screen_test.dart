import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/household_screen.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, Map<String, dynamic>> responses;

  _CaptureLink(this.responses);

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    yield Response(
      data: responses[_opName(request)] ?? <String, dynamic>{},
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

const _me = {'id': '2', 'displayName': 'E2E User', 'firstName': null, 'lastName': null};
const _mate = {
  'id': '9',
  'displayName': 'Mate',
  'firstName': null,
  'lastName': null,
};

const _household = {
  'id': '3',
  'name': null,
  'myRole': 'OWNER',
  'createdAt': '2026-01-01T00:00:00Z',
  'members': [
    {'user': _me, 'role': 'OWNER', 'isMe': true},
    {'user': _mate, 'role': 'MEMBER', 'isMe': false},
  ],
};

// e2e's inactive, sole-owned second household — merge-eligible on accept.
const _solo = {
  'id': '8',
  'name': 'Beach house',
  'myRole': 'OWNER',
  'isActive': false,
  'members': [
    {'user': _me, 'role': 'OWNER', 'isMe': true},
  ],
};

const _householdList = [
  {
    'id': '3',
    'name': null,
    'myRole': 'OWNER',
    'isActive': true,
    'members': [
      {'role': 'OWNER', 'isMe': true},
      {'role': 'MEMBER', 'isMe': false},
    ],
  },
  _solo,
];

Map<String, dynamic> _householdResponse({List invites = const []}) => {
      'me': {'id': '2', 'isSearchable': true},
      'myHousehold': _household,
      'myHouseholds': _householdList,
      'householdInvites': invites,
      'myNotifications': const [],
      'unreadNotificationCount': 0,
    };

_CaptureLink _link({Map<String, Map<String, dynamic>> extra = const {}}) =>
    _CaptureLink({
      'HouseholdScreen': _householdResponse(),
      'MyAllergies': const <String, dynamic>{},
      'SetActiveHousehold': const {'setActiveHousehold': {'id': '8'}},
      'CreateHousehold': const {'createHousehold': {'id': '9', 'name': 'Cabin'}},
      'AcceptInvite': const {'acceptHouseholdInvite': {'id': '3'}},
      'LeaveHousehold': const {'leaveHousehold': true},
      ...extra,
    });

Widget _app(_CaptureLink link) => GraphQLProvider(
      client: ValueNotifier(
        GraphQLClient(cache: GraphQLCache(), link: link),
      ),
      child: const MaterialApp(home: HouseholdScreen()),
    );

Future<void> _settle(WidgetTester tester) async {
  // The screen polls every 5s — pumpAndSettle would never return. The
  // 400ms pump also lets the bottom-sheet slide-up finish.
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
}

// The page is a long lazy ListView — a phone-sized surface renders every
// section so finders don't depend on scrolling.
void _bigSurface(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2700);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

void main() {
  testWidgets(
    'HouseholdScreen renders its app bar while the query loads',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: HouseholdScreen()),
        ),
      );
      await tester.pump();

      expect(find.text('Household'), findsOneWidget);
    },
  );

  testWidgets('active-household header opens the switcher sheet', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('Unnamed household'), findsWidgets);
    await tester.tap(find.widgetWithText(TextButton, 'Switch'));
    await _settle(tester);

    expect(find.text('Your households'), findsNothing); // web-only label
    expect(find.text('Beach house'), findsOneWidget);
    expect(find.text('New household'), findsOneWidget);
    expect(find.text('2 member(s) · owner'), findsOneWidget);
    expect(find.text('1 member(s) · owner'), findsOneWidget);
  });

  testWidgets('switching households calls setActiveHousehold', (tester) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.widgetWithText(TextButton, 'Switch'));
    await _settle(tester);
    // Two Switch buttons would be ambiguous — the sheet row for the
    // inactive household is the only TextButton('Switch') inside it.
    await tester.tap(
      find.descendant(
        of: find.byType(BottomSheet),
        matching: find.widgetWithText(TextButton, 'Switch'),
      ),
    );
    await _settle(tester);

    final calls = link.byName('SetActiveHousehold');
    expect(calls, hasLength(1));
    expect(calls.single.variables['householdId'], '8');
  });

  testWidgets('new household dialog calls createHousehold', (tester) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.widgetWithText(TextButton, 'Switch'));
    await _settle(tester);
    await tester.tap(find.text('New household'));
    await _settle(tester);

    await tester.enterText(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ),
      'Cabin',
    );
    await tester.tap(find.widgetWithText(FilledButton, 'Create'));
    await _settle(tester);

    final calls = link.byName('CreateHousehold');
    expect(calls, hasLength(1));
    expect(calls.single.variables['name'], 'Cabin');
  });

  testWidgets('per-household leave sends the target id', (tester) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.widgetWithText(TextButton, 'Switch'));
    await _settle(tester);
    await tester.tap(find.byTooltip('Leave Beach house'));
    await _settle(tester);
    await tester.tap(find.widgetWithText(FilledButton, 'Confirm'));
    await _settle(tester);

    final calls = link.byName('LeaveHousehold');
    expect(calls, hasLength(1));
    expect(calls.single.variables['householdId'], '8');
  });

  testWidgets('accept offers merge when caller solely owns a household', (
    tester,
  ) async {
    const invite = {
      'id': '55',
      'status': 'PENDING',
      'fromUser': {
        'id': '134',
        'displayName': 'Dana',
        'firstName': null,
        'lastName': null,
      },
      'toUser': _me,
    };
    _bigSurface(tester);
    final link = _link(
      extra: {
        'HouseholdScreen': _householdResponse(invites: const [invite]),
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.byIcon(Icons.check));
    await _settle(tester);

    expect(find.text('Join this household?'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Join and merge'));
    await _settle(tester);

    final calls = link.byName('AcceptInvite');
    expect(calls, hasLength(1));
    expect(calls.single.variables['inviteId'], '55');
    expect(calls.single.variables['mergeFromHouseholdId'], '8');
  });

  testWidgets('just join accepts without a merge source', (tester) async {
    const invite = {
      'id': '55',
      'status': 'PENDING',
      'fromUser': {
        'id': '134',
        'displayName': 'Dana',
        'firstName': null,
        'lastName': null,
      },
      'toUser': _me,
    };
    _bigSurface(tester);
    final link = _link(
      extra: {
        'HouseholdScreen': _householdResponse(invites: const [invite]),
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.byIcon(Icons.check));
    await _settle(tester);
    await tester.tap(find.widgetWithText(TextButton, 'Just join'));
    await _settle(tester);

    final calls = link.byName('AcceptInvite');
    expect(calls, hasLength(1));
    expect(calls.single.variables['mergeFromHouseholdId'], isNull);
  });
}
