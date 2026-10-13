import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/auth/auth_service.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/household_screen.dart';
import 'package:provider/provider.dart';

class _MemStorage implements TokenStorage {
  final map = <String, String>{'id_token': 'tok', 'refresh_token': 'rt'};

  @override
  Future<String?> read(String key) async => map[key];

  @override
  Future<void> write(String key, String? value) async {
    value == null ? map.remove(key) : map[key] = value;
  }

  @override
  Future<void> delete(String key) async => map.remove(key);
}

// A test auth that stays signed in: _init rotates the stored refresh
// token, so the refresh endpoint must answer 200. Revoke is best-effort
// and may 401.
AuthService _testAuth() => AuthService.test(
      storage: _MemStorage(),
      httpClient: http_testing.MockClient((req) async {
        if (req.url.path == '/auth/session/refresh') {
          return http.Response(
            '{"accessToken":"at","refreshToken":"rt2"}',
            200,
          );
        }
        return http.Response('{}', 401);
      }),
    );

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
  final Map<String, List<GraphQLError>> errorsOnce;
  final _onceSeen = <String>{};

  _CaptureLink(
    this.responses, {
    this.errors = const {},
    this.errorsOnce = const {},
  });

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    final name = _opName(request);
    var errs = errors[name];
    if (name != null &&
        errorsOnce.containsKey(name) &&
        _onceSeen.add(name)) {
      errs = errorsOnce[name];
    }
    yield Response(
      data: responses[name] ?? <String, dynamic>{},
      errors: errs,
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

const _me = {
  'id': '2',
  'displayName': 'E2E User',
  'firstName': null,
  'lastName': null
};
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

Map<String, dynamic> _householdResponse({
  List invites = const [],
  List notifications = const [],
  int unread = 0,
  List? searchResults,
  Map<String, dynamic>? household,
  bool? isSearchable,
  bool? disclaimerAccepted,
}) =>
    {
      'me': {
        'id': '2',
        'isSearchable': isSearchable ?? true,
        'disclaimerAccepted': disclaimerAccepted ?? true,
      },
      'myHousehold': household ?? _household,
      'myHouseholds': _householdList,
      'householdInvites': invites,
      'myNotifications': notifications,
      'unreadNotificationCount': unread,
      if (searchResults != null) 'searchHouseholdUsers': searchResults,
    };

_CaptureLink _link({
  Map<String, Map<String, dynamic>> extra = const {},
  Map<String, List<GraphQLError>> errors = const {},
  Map<String, List<GraphQLError>> errorsOnce = const {},
  bool? disclaimerAccepted,
}) =>
    _CaptureLink(
      {
        'HouseholdScreen': _householdResponse(
          disclaimerAccepted: disclaimerAccepted,
        ),
        'MyAllergies': const <String, dynamic>{},
        'SetActiveHousehold': const {
          'setActiveHousehold': {'id': '8'}
        },
        'CreateHousehold': const {
          'createHousehold': {'id': '9', 'name': 'Cabin'}
        },
        'AcceptInvite': const {
          'acceptHouseholdInvite': {'id': '3'}
        },
        'LeaveHousehold': const {'leaveHousehold': true},
        ...extra,
      },
      errors: errors,
      errorsOnce: errorsOnce,
    );

Widget _app(_CaptureLink link, {AuthService? auth}) => GraphQLProvider(
      client: ValueNotifier(
        GraphQLClient(cache: GraphQLCache(), link: link),
      ),
      child: ChangeNotifierProvider<AuthService>.value(
        value: auth ?? _testAuth(),
        child: const MaterialApp(home: HouseholdScreen()),
      ),
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

  testWidgets('owner renames the household inline', (tester) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'RenameHousehold': const {
        'renameHousehold': {'id': '3', 'name': 'Cabin'},
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.enterText(find.byType(TextField).first, 'Cabin');
    await tester.tap(find.byTooltip('Save household name'));
    await _settle(tester);

    final calls = link.byName('RenameHousehold');
    expect(calls, hasLength(1));
    expect(calls.single.variables['name'], 'Cabin');
  });

  testWidgets('member menu makes a member an admin', (tester) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'SetHouseholdRole': const {
        'setHouseholdRole': {'id': '9'}
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.byIcon(Icons.more_vert));
    await _settle(tester);

    expect(find.text('Make admin'), findsOneWidget);
    expect(find.text('Transfer ownership'), findsOneWidget);
    expect(find.text('Remove from household'), findsOneWidget);

    await tester.tap(find.text('Make admin'));
    await _settle(tester);

    final calls = link.byName('SetHouseholdRole');
    expect(calls, hasLength(1));
    expect(calls.single.variables['userId'], '9');
    expect(calls.single.variables['role'], 'ADMIN');
  });

  testWidgets('member menu transfers ownership after confirm', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'TransferHouseholdOwnership': const {
        'transferHouseholdOwnership': {'id': '9'},
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.byIcon(Icons.more_vert));
    await _settle(tester);
    await tester.tap(find.text('Transfer ownership'));
    await _settle(tester);

    expect(find.textContaining('Transfer ownership to Mate'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Confirm'));
    await _settle(tester);

    final calls = link.byName('TransferHouseholdOwnership');
    expect(calls, hasLength(1));
    expect(calls.single.variables['userId'], '9');
  });

  testWidgets('member menu removes a member after confirm', (tester) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'RemoveHouseholdMember': const {
        'removeHouseholdMember': {'id': '9'},
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.byIcon(Icons.more_vert));
    await _settle(tester);
    await tester.tap(find.text('Remove from household'));
    await _settle(tester);

    expect(find.textContaining('Remove Mate'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Confirm'));
    await _settle(tester);

    final calls = link.byName('RemoveHouseholdMember');
    expect(calls, hasLength(1));
    expect(calls.single.variables['userId'], '9');
  });

  testWidgets('leaving the active household confirms and mutates', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Leave household'));
    await _settle(tester);

    expect(find.textContaining('Leave this household?'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Confirm'));
    await _settle(tester);

    final calls = link.byName('LeaveHousehold');
    expect(calls, hasLength(1));
    expect(calls.single.variables['householdId'], '3');
  });

  testWidgets('declining an incoming invite calls declineHouseholdInvite', (
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
        'DeclineInvite': const {
          'declineHouseholdInvite': {'id': '55'}
        },
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('Dana invited you'), findsOneWidget);
    await tester.tap(find.byIcon(Icons.close));
    await _settle(tester);

    final calls = link.byName('DeclineInvite');
    expect(calls, hasLength(1));
    expect(calls.single.variables['inviteId'], '55');
  });

  testWidgets('sent invitations can be cancelled', (tester) async {
    const invite = {
      'id': '56',
      'status': 'PENDING',
      'fromUser': _me,
      'toUser': {
        'id': '77',
        'displayName': 'Pat',
        'firstName': null,
        'lastName': null,
      },
    };
    _bigSurface(tester);
    final link = _link(
      extra: {
        'HouseholdScreen': _householdResponse(invites: const [invite]),
        'CancelInvite': const {
          'cancelHouseholdInvite': {'id': '56'}
        },
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('Sent invitations'), findsOneWidget);
    expect(find.text('Pat'), findsOneWidget);
    await tester.tap(find.byIcon(Icons.cancel_outlined));
    await _settle(tester);

    final calls = link.byName('CancelInvite');
    expect(calls, hasLength(1));
    expect(calls.single.variables['inviteId'], '56');
  });

  testWidgets(
    'unread notifications mark all read on open and render kinds',
    (tester) async {
      final notifications = [
        {
          'id': 'n1',
          'kind': 'MEMBER_JOINED',
          'foodEventId': null,
          'title': null,
          'body': null,
          'recipeId': null,
          'itemId': null,
          'createdAt': DateTime.now().toUtc().toIso8601String(),
          'actor': {
            'id': '134',
            'displayName': 'Dana',
            'firstName': null,
            'lastName': null,
          },
        },
        {
          'id': 'n2',
          'kind': 'ITEM_EXPIRING',
          'foodEventId': null,
          'title': 'Milk expires soon',
          'body': 'Expires in 2 days',
          'recipeId': null,
          'itemId': 'i5',
          'createdAt': DateTime.now()
              .subtract(const Duration(hours: 3))
              .toUtc()
              .toIso8601String(),
          'actor': null,
        },
        {
          'id': 'n3',
          'kind': 'EVENT_UPDATED',
          'foodEventId': null,
          'title': null,
          'body': null,
          'recipeId': null,
          'itemId': null,
          'createdAt': DateTime.now()
              .subtract(const Duration(days: 2))
              .toUtc()
              .toIso8601String(),
          'actor': {
            'id': '9',
            'displayName': 'Mate',
            'firstName': null,
            'lastName': null,
          },
        },
      ];
      _bigSurface(tester);
      final link = _link(extra: {
        'HouseholdScreen': _householdResponse(
          notifications: notifications,
          unread: 2,
        ),
        'MarkAllRead': const {'markAllNotificationsRead': true},
      });
      await tester.pumpWidget(_app(link));
      await _settle(tester);

      expect(link.byName('MarkAllRead'), hasLength(1));
      expect(find.text('Notifications'), findsOneWidget);
      expect(find.text('Dana joined your household'), findsOneWidget);
      expect(find.text('Milk expires soon'), findsOneWidget);
      expect(find.text('Expires in 2 days'), findsOneWidget);
      expect(find.text('Mate updated an event'), findsOneWidget);
      expect(find.text('3h ago'), findsOneWidget);
      expect(find.text('2d ago'), findsOneWidget);
    },
  );

  testWidgets('expiring-item notification adds a replacement to the list', (
    tester,
  ) async {
    final notifications = [
      {
        'id': 'n2',
        'kind': 'ITEM_EXPIRING',
        'foodEventId': null,
        'title': 'Milk expires soon',
        'body': null,
        'recipeId': null,
        'itemId': 'i5',
        'createdAt': DateTime.now().toUtc().toIso8601String(),
        'actor': null,
      },
    ];
    _bigSurface(tester);
    final link = _link(extra: {
      'HouseholdScreen': _householdResponse(notifications: notifications),
      'AddToGrocery': const {
        'addItemToCurrentGroceryList': {'id': 'gl1'},
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Add to list'));
    await _settle(tester);
    await _settle(tester);

    final calls = link.byName('AddToGrocery');
    expect(calls, hasLength(1));
    expect(calls.single.variables['itemId'], 'i5');
  });

  testWidgets('tapping a linked notification opens its destination', (
    tester,
  ) async {
    final notifications = [
      {
        'id': 'n4',
        'kind': 'PROTEIN_DEFROST',
        'foodEventId': null,
        'title': 'Defrost the chicken',
        'body': null,
        'recipeId': 'r9',
        'itemId': null,
        'createdAt': DateTime.now().toUtc().toIso8601String(),
        'actor': null,
      },
    ];
    _bigSurface(tester);
    final link = _link(extra: {
      'HouseholdScreen': _householdResponse(notifications: notifications),
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('Defrost the chicken'), findsOneWidget);
    await tester.tap(find.text('Defrost the chicken'));
    await _settle(tester);

    // notificationDestination routes recipeId to the recipe editor.
    expect(find.text('Edit Recipe'), findsOneWidget);
  });

  testWidgets('allergy rows set and clear through SetMyAllergy', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'MyAllergies': const {
        'allergens': [
          {
            'id': 'a1',
            'name': 'Peanuts',
            'description': null,
            'isActive': true,
          },
          {
            'id': 'a2',
            'name': 'Gluten',
            'description': null,
            'isActive': false,
          },
        ],
        'myAllergies': [
          {
            'kind': 'allergy',
            'allergen': {'id': 'a1', 'name': 'Peanuts'},
          },
        ],
      },
      'SetMyAllergy': const {'setMyAllergy': true},
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    // Inactive allergen is filtered out.
    expect(find.text('Peanuts'), findsOneWidget);
    expect(find.text('Gluten'), findsNothing);

    // Switch Allergy → Dietary (kind + on:true).
    await tester.tap(find.text('Dietary'));
    await _settle(tester);
    var calls = link.byName('SetMyAllergy');
    expect(calls, hasLength(1));
    expect(calls.single.variables['allergenId'], 'a1');
    expect(calls.single.variables['kind'], 'dietary');
    expect(calls.single.variables['on'], isTrue);

    // None clears the record, keeping the current kind.
    await tester.tap(find.text('None'));
    await _settle(tester);
    calls = link.byName('SetMyAllergy');
    expect(calls, hasLength(2));
    expect(calls[1].variables['on'], isFalse);
  });

  const _peanuts = {
    'allergens': [
      {
        'id': 'a1',
        'name': 'Peanuts',
        'description': null,
        'isActive': true,
      },
    ],
    'myAllergies': <Map<String, dynamic>>[],
  };

  testWidgets('recording an allergy asks for the disclaimer first', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link(
      disclaimerAccepted: false,
      extra: {
        'MyAllergies': _peanuts,
        'SetMyAllergy': const {'setMyAllergy': true},
        'AcceptDisclaimer': const {'acceptDisclaimer': true},
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Allergy'));
    await _settle(tester);

    // The consent sheet opens before anything is written.
    expect(find.text('Medical & allergen disclaimer'), findsOneWidget);
    expect(link.byName('SetMyAllergy'), isEmpty);

    await tester.tap(find.text('I understand and accept'));
    await _settle(tester);

    expect(link.byName('AcceptDisclaimer'), hasLength(1));
    final calls = link.byName('SetMyAllergy');
    expect(calls, hasLength(1));
    expect(calls.single.variables['allergenId'], 'a1');
    expect(calls.single.variables['kind'], 'allergy');
    expect(calls.single.variables['on'], isTrue);
  });

  testWidgets('dismissing the disclaimer sheet writes nothing', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link(
      disclaimerAccepted: false,
      extra: {
        'MyAllergies': _peanuts,
        'SetMyAllergy': const {'setMyAllergy': true},
        'AcceptDisclaimer': const {'acceptDisclaimer': true},
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Allergy'));
    await _settle(tester);
    await tester.tap(find.text('Not now'));
    await _settle(tester);

    expect(link.byName('AcceptDisclaimer'), isEmpty);
    expect(link.byName('SetMyAllergy'), isEmpty);
  });

  testWidgets('clearing a record never asks for the disclaimer', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link(
      disclaimerAccepted: false,
      extra: {
        'MyAllergies': const {
          'allergens': [
            {
              'id': 'a1',
              'name': 'Peanuts',
              'description': null,
              'isActive': true,
            },
          ],
          'myAllergies': [
            {
              'kind': 'allergy',
              'allergen': {'id': 'a1', 'name': 'Peanuts'},
            },
          ],
        },
        'SetMyAllergy': const {'setMyAllergy': true},
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('None'));
    await _settle(tester);

    expect(find.text('Medical & allergen disclaimer'), findsNothing);
    expect(link.byName('AcceptDisclaimer'), isEmpty);
    final calls = link.byName('SetMyAllergy');
    expect(calls, hasLength(1));
    expect(calls.single.variables['on'], isFalse);
  });

  testWidgets('a server refusal re-asks once and retries the write', (
    tester,
  ) async {
    _bigSurface(tester);
    // The cached disclaimerAccepted flag says accepted, but the server
    // still refuses — a stale flag re-opens the sheet and retries once.
    final link = _link(
      extra: {
        'MyAllergies': _peanuts,
        'SetMyAllergy': const {'setMyAllergy': true},
        'AcceptDisclaimer': const {'acceptDisclaimer': true},
      },
      errorsOnce: {
        'SetMyAllergy': [
          const GraphQLError(
            message: 'the medical disclaimer must be accepted',
            extensions: {'code': 'DISCLAIMER_REQUIRED'},
          ),
        ],
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Allergy'));
    await _settle(tester);

    expect(find.text('Medical & allergen disclaimer'), findsOneWidget);
    await tester.tap(find.text('I understand and accept'));
    await _settle(tester);

    expect(link.byName('AcceptDisclaimer'), hasLength(1));
    expect(link.byName('SetMyAllergy'), hasLength(2));
  });

  testWidgets('no registered allergens shows the empty note', (tester) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'MyAllergies': const {
        'allergens': [],
        'myAllergies': [],
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('No allergens are registered yet.'), findsOneWidget);
  });

  testWidgets('the searchable toggle calls updateMyProfile', (tester) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'UpdateProfile': const {
        'updateMyProfile': {'id': '2', 'isSearchable': false},
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Discoverable in member search'));
    await _settle(tester);

    final calls = link.byName('UpdateProfile');
    expect(calls, hasLength(1));
    expect(
      calls.single.variables['input'],
      {'isSearchable': false},
    );
  });

  testWidgets('a failed mutation surfaces the server message', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _CaptureLink(
      {
        'HouseholdScreen': _householdResponse(),
        'MyAllergies': const <String, dynamic>{},
      },
      errors: {
        'UpdateProfile': [
          const GraphQLError(message: 'profile write denied'),
        ],
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.tap(find.text('Discoverable in member search'));
    await _settle(tester);

    expect(find.text('profile write denied'), findsOneWidget);
  });

  testWidgets('member search finds and invites a user', (tester) async {
    _bigSurface(tester);
    final link = _link(extra: {
      'InviteMember': const {
        'inviteHouseholdMember': {'id': 'i9'}
      },
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    // Under 2 chars shows the hint and skips the search.
    await tester.enterText(find.byType(TextField).last, 'd');
    await tester.tap(find.byIcon(Icons.search));
    await _settle(tester);
    expect(
      find.text('Type at least 2 characters to search.'),
      findsOneWidget,
    );

    // Two+ chars refires the query with search enabled.
    link.responses['HouseholdScreen'] = _householdResponse(
      searchResults: const [
        {
          'id': 'u7',
          'displayName': 'Dana D',
          'firstName': null,
          'lastName': null,
        },
      ],
    );
    await tester.enterText(find.byType(TextField).last, 'dana');
    await tester.tap(find.byIcon(Icons.search));
    await _settle(tester);

    final searches = link
        .byName('HouseholdScreen')
        .where((r) => r.variables['search'] == true);
    expect(searches, isNotEmpty);

    expect(find.text('Dana D'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Invite'));
    await _settle(tester);

    final invites = link.byName('InviteMember');
    expect(invites, hasLength(1));
    expect(invites.single.variables['userId'], 'u7');
  });

  testWidgets('empty search result reports no users found', (tester) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    await tester.enterText(find.byType(TextField).last, 'zz');
    await tester.tap(find.byIcon(Icons.search));
    await _settle(tester);

    expect(find.text('No users found.'), findsOneWidget);
  });

  testWidgets('a full household disables inviting', (tester) async {
    final fullHousehold = {
      'id': '3',
      'name': 'Full house',
      'myRole': 'OWNER',
      'createdAt': '2026-01-01T00:00:00Z',
      'members': [
        for (var i = 0; i < 10; i++)
          {
            'user': {
              'id': 'u$i',
              'displayName': 'Member $i',
              'firstName': null,
              'lastName': null,
            },
            'role': i == 0 ? 'OWNER' : 'MEMBER',
            'isMe': i == 0,
          },
      ],
    };
    _bigSurface(tester);
    final link = _link(extra: {
      'HouseholdScreen': _householdResponse(household: fullHousehold),
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(
      find.text('This household is at the 10-member limit.'),
      findsOneWidget,
    );
    expect(find.text('Members — 10 of 10'), findsOneWidget);
  });

  testWidgets('the query error state renders', (tester) async {
    _bigSurface(tester);
    final link = _CaptureLink(
      const {},
      errors: {
        'HouseholdScreen': [const GraphQLError(message: 'boom')],
      },
    );
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.textContaining('Error:'), findsOneWidget);
  });

  testWidgets('danger zone renders the irreversible warning', (
    tester,
  ) async {
    _bigSurface(tester);
    final link = _link();
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('Danger zone'), findsOneWidget);
    expect(find.text('Delete my account'), findsOneWidget);
    expect(
        find.textContaining('permanent and cannot be undone'), findsOneWidget);
  });

  testWidgets('delete requires typing DELETE and signs out on success', (
    tester,
  ) async {
    _bigSurface(tester);
    final auth = _testAuth();
    // Response shape mirrors production: no __typename — the success check
    // must read the payload, not hasException (cache normalization always
    // tacks on a PartialDataException).
    final link = _link(extra: {
      'DeleteMyAccount': const {'deleteMyAccount': true},
    });
    await tester.pumpWidget(_app(link, auth: auth));
    await auth.ready;
    await _settle(tester);

    await tester.tap(find.text('Delete my account'));
    await _settle(tester);

    expect(find.text('Delete your account?'), findsOneWidget);
    expect(find.textContaining('irreversible'), findsOneWidget);
    final confirmButton =
        find.widgetWithText(FilledButton, 'Delete permanently');
    expect(tester.widget<FilledButton>(confirmButton).onPressed, isNull);

    await tester.enterText(find.byType(TextField).last, 'DELET');
    await _settle(tester);
    expect(tester.widget<FilledButton>(confirmButton).onPressed, isNull);

    await tester.enterText(find.byType(TextField).last, 'DELETE');
    await _settle(tester);
    expect(tester.widget<FilledButton>(confirmButton).onPressed, isNotNull);
    await tester.tap(confirmButton);
    await _settle(tester);
    // The mutation → signOut chain needs another event-loop turn past the
    // recorded request before the credentials clear.
    await _settle(tester);

    expect(link.byName('DeleteMyAccount'), hasLength(1));
    expect(auth.isSignedIn, isFalse);
  });

  testWidgets('delete failure shows the server message and stays signed in', (
    tester,
  ) async {
    _bigSurface(tester);
    final auth = _testAuth();
    final link = _CaptureLink(
      {
        'HouseholdScreen': _householdResponse(),
        'MyAllergies': const <String, dynamic>{},
      },
      errors: {
        'DeleteMyAccount': [
          const GraphQLError(message: 'last active admin'),
        ],
      },
    );
    await tester.pumpWidget(_app(link, auth: auth));
    await auth.ready;
    await _settle(tester);

    await tester.tap(find.text('Delete my account'));
    await _settle(tester);
    await tester.enterText(find.byType(TextField).last, 'DELETE');
    await _settle(tester);
    await tester.tap(find.widgetWithText(FilledButton, 'Delete permanently'));
    await _settle(tester);

    expect(find.text('last active admin'), findsOneWidget);
    expect(auth.isSignedIn, isTrue);
  });

  testWidgets('non-owner sees the household name but no inline editor', (
    tester,
  ) async {
    final memberView = {
      'id': '3',
      'name': 'Shared house',
      'myRole': 'MEMBER',
      'createdAt': '2026-01-01T00:00:00Z',
      'members': [
        {'user': _me, 'role': 'MEMBER', 'isMe': true},
        {'user': _mate, 'role': 'OWNER', 'isMe': false},
      ],
    };
    _bigSurface(tester);
    final link = _link(extra: {
      'HouseholdScreen': _householdResponse(household: memberView),
    });
    await tester.pumpWidget(_app(link));
    await _settle(tester);

    expect(find.text('Shared house'), findsWidgets);
    // Member can't manage the owner or themselves — no menus.
    expect(find.byIcon(Icons.more_vert), findsNothing);
  });
}
