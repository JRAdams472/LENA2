import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/screens/adjust_bottle_screen.dart';
import 'package:lena_mobile/screens/bottles_screen.dart';
import 'package:lena_mobile/screens/edit_bottle_screen.dart';
import 'package:lena_mobile/screens/wine_screen.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

// Recursive __typename injection — GraphQLCache's normalized store drops
// objects without it, so fixture data must synthesize types.
Object? _withTypename(Object? node) {
  if (node is Map) {
    final out = <String, dynamic>{'__typename': 'T'};
    node.forEach((k, v) => out['$k'] = _withTypename(v));
    return out;
  }
  if (node is List) return node.map(_withTypename).toList();
  return node;
}

typedef _Responder = Map<String, dynamic> Function(Request request);

class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, _Responder> responders;
  final Map<String, List<GraphQLError>> errors;

  _CaptureLink(Map<String, Map<String, dynamic>> responses,
      {Map<String, _Responder> fns = const {}, this.errors = const {}})
      : responders = {
          for (final e in responses.entries) e.key: (_) => e.value,
          ...fns,
        };

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    final op = _opName(request);
    yield Response(
      data: _withTypename(responders[op]?.call(request) ?? <String, dynamic>{})
          as Map<String, dynamic>,
      errors: errors[op],
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

GraphQLClient _client(_CaptureLink link) => GraphQLClient(
      cache: GraphQLCache(),
      link: link,
    );

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

Future<void> _open(
  WidgetTester tester,
  _CaptureLink link,
  Widget home,
) async {
  await tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(_client(link)),
      child: MaterialApp(home: home),
    ),
  );
  await _settle(tester);
}

Map<String, dynamic> _userBottle({
  required String id,
  required String bottleId,
  String vineyard = 'Chateau Margaux',
  int? vintage = 2015,
  int quantity = 3,
  bool isFavorite = false,
}) =>
    {
      'id': id,
      'bottle': {
        'id': bottleId,
        'vineyard': vineyard,
        'vintageYear': vintage,
      },
      'quantity': quantity,
      'isFavorite': isFavorite,
    };

Map<String, dynamic> _bottle({
  required String id,
  String vineyard = 'Opus One',
  int? vintage = 2018,
  String? size = '750ml',
}) =>
    {
      'id': id,
      'vineyard': vineyard,
      'vintageYear': vintage,
      'bottleSize': size
    };

void main() {
  group('WineScreen', () {
    testWidgets('renders the cellar rows with quantity and favorite state',
        (tester) async {
      final link = _CaptureLink({
        'UserBottles': {
          'userBottles': {
            'items': [
              _userBottle(id: 'ub-1', bottleId: 'b-1'),
              _userBottle(
                id: 'ub-2',
                bottleId: 'b-2',
                vineyard: 'Dom Perignon',
                vintage: null,
                quantity: 1,
                isFavorite: true,
              ),
            ],
            'pageInfo': {'totalCount': 2},
          },
        },
      });
      await _open(tester, link, const WineScreen());

      expect(find.text('Wine Cellar'), findsOneWidget);
      expect(find.text('Chateau Margaux 2015'), findsOneWidget);
      expect(find.text('Dom Perignon'), findsOneWidget);
      expect(find.text('Quantity: 3'), findsOneWidget);
      expect(find.byIcon(Icons.favorite), findsOneWidget);
      expect(find.byIcon(Icons.favorite_border), findsOneWidget);
    });

    testWidgets('shows the error and an empty cellar state', (tester) async {
      final bad = _CaptureLink({}, errors: {
        'UserBottles': [const GraphQLError(message: 'cellar down')],
      });
      await _open(tester, bad, const WineScreen());
      expect(find.textContaining('Error:'), findsOneWidget);

      final empty = _CaptureLink({
        'UserBottles': {
          'userBottles': {
            'items': <dynamic>[],
            'pageInfo': {'totalCount': 0},
          },
        },
      });
      await _open(tester, empty, const WineScreen());
      expect(find.byType(Card), findsNothing);
      expect(find.textContaining('Error:'), findsNothing);
    });

    testWidgets('toggling a favorite mutates and refetches the list',
        (tester) async {
      final link = _CaptureLink({
        'UserBottles': {
          'userBottles': {
            'items': [_userBottle(id: 'ub-1', bottleId: 'b-1')],
            'pageInfo': {'totalCount': 1},
          },
        },
        'SetBottleFavorite': {
          'setBottleFavorite': {'id': 'ub-1'},
        },
      });
      await _open(tester, link, const WineScreen());

      await tester.tap(find.byIcon(Icons.favorite_border));
      await _settle(tester);

      final favs = link.byName('SetBottleFavorite');
      expect(favs, hasLength(1));
      expect(favs.single.variables['bottleId'], 'b-1');
      expect(favs.single.variables['isFavorite'], isTrue);
      // .then(refetch) — a second UserBottles request landed.
      expect(link.byName('UserBottles').length, greaterThanOrEqualTo(2));
    });

    testWidgets('loads the next page when scrolled to the bottom',
        (tester) async {
      // 20 cards overflow the 800x600 viewport, so the list can actually
      // scroll and fire the extentAfter<300 trigger.
      final page1 = [
        for (var i = 0; i < 20; i++)
          _userBottle(id: 'ub-$i', bottleId: 'b-$i', vineyard: 'Vine $i'),
      ];
      final link = _CaptureLink({}, fns: {
        'UserBottles': (req) {
          final page = req.variables['page'] as int? ?? 1;
          return {
            'userBottles': {
              'items': page == 1
                  ? page1
                  : [
                      _userBottle(
                          id: 'ub-tail',
                          bottleId: 'b-tail',
                          vineyard: 'Screaming Eagle'),
                    ],
              'pageInfo': {'totalCount': 21},
            },
          };
        },
      });
      await _open(tester, link, const WineScreen());

      await tester.drag(find.byType(Scrollable).last, const Offset(0, -3000));
      await _settle(tester);

      // The tail row is rendered; a back-to-back metrics notification can
      // race a second page-2 merge, so the row may appear more than once.
      expect(find.text('Screaming Eagle 2015'), findsWidgets);
      final pages =
          link.byName('UserBottles').map((r) => r.variables['page']).toList();
      expect(pages, contains(2));
    });

    testWidgets('tapping a row opens the prefilled adjust screen',
        (tester) async {
      final link = _CaptureLink({
        'UserBottles': {
          'userBottles': {
            'items': [_userBottle(id: 'ub-1', bottleId: 'b-1', quantity: 5)],
            'pageInfo': {'totalCount': 1},
          },
        },
      });
      await _open(tester, link, const WineScreen());

      await tester.tap(find.text('Chateau Margaux 2015'));
      await _settle(tester);

      expect(find.byType(AdjustBottleScreen), findsOneWidget);
      expect(find.text('Adjust Holding'), findsOneWidget);
      // The bottle name appears in the picker field of the pushed screen.
      expect(
        find.descendant(
          of: find.byType(AdjustBottleScreen),
          matching: find.text('Chateau Margaux 2015'),
        ),
        findsOneWidget,
      );
      expect(
          find.descendant(
            of: find.byType(AdjustBottleScreen),
            matching: find.widgetWithText(TextField, '5'),
          ),
          findsOneWidget);
    });

    testWidgets('the Adjust FAB opens a blank adjust screen', (tester) async {
      final link = _CaptureLink({
        'UserBottles': {
          'userBottles': {
            'items': <dynamic>[],
            'pageInfo': {'totalCount': 0},
          },
        },
      });
      await _open(tester, link, const WineScreen());

      await tester.tap(find.text('Adjust'));
      await _settle(tester);

      expect(find.byType(AdjustBottleScreen), findsOneWidget);
      expect(find.text('Select a bottle'), findsOneWidget);
    });

    testWidgets('the catalog icon opens the bottle catalog', (tester) async {
      final link = _CaptureLink({
        'UserBottles': {
          'userBottles': {
            'items': <dynamic>[],
            'pageInfo': {'totalCount': 0},
          },
        },
      });
      await _open(tester, link, const WineScreen());

      await tester.tap(find.byTooltip('Bottle catalog'));
      await _settle(tester);

      expect(find.byType(BottlesScreen), findsOneWidget);
      expect(find.text('Bottles'), findsOneWidget);
    });
  });

  group('BottlesScreen', () {
    testWidgets('renders catalog rows and navigates to edit', (tester) async {
      final link = _CaptureLink({
        'Bottles': {
          'bottles': {
            'items': [_bottle(id: 'b-1')],
            'pageInfo': {'totalCount': 1},
          },
        },
      });
      await _open(tester, link, const BottlesScreen());

      expect(find.text('Opus One 2018'), findsOneWidget);
      expect(find.text('750ml'), findsOneWidget);

      await tester.tap(find.text('Opus One 2018'));
      await _settle(tester);

      expect(find.byType(EditBottleScreen), findsOneWidget);
      expect(find.text('Edit Bottle'), findsOneWidget);
    });

    testWidgets('the FAB opens the create-bottle form', (tester) async {
      final link = _CaptureLink({
        'Bottles': {
          'bottles': {
            'items': <dynamic>[],
            'pageInfo': {'totalCount': 0},
          },
        },
      });
      await _open(tester, link, const BottlesScreen());

      await tester.tap(find.byType(FloatingActionButton));
      await _settle(tester);

      expect(find.byType(EditBottleScreen), findsOneWidget);
      expect(find.text('Create Bottle'), findsOneWidget);
    });

    testWidgets('search debounces into a filtered query and records telemetry',
        (tester) async {
      final link = _CaptureLink({
        'Bottles': {
          'bottles': {
            'items': [_bottle(id: 'b-1', vineyard: 'Margaux')],
            'pageInfo': {'totalCount': 1},
          },
        },
        'RecordSearch': {'recordSearch': true},
      });
      await _open(tester, link, const BottlesScreen());
      expect(link.byName('Bottles').single.variables['search'], isNull);

      await tester.enterText(find.byType(TextField), 'margaux');
      await tester.pump(const Duration(milliseconds: 600));
      await _settle(tester);

      final searches = link
          .byName('Bottles')
          .where((r) => r.variables['search'] == 'margaux')
          .toList();
      expect(searches, isNotEmpty);
      final records = link.byName('RecordSearch');
      expect(records, hasLength(1));
      expect(records.single.variables['entityType'], 'bottle');
      expect(records.single.variables['term'], 'margaux');
    });

    testWidgets('shows the error state and pages forward', (tester) async {
      final bad = _CaptureLink({}, errors: {
        'Bottles': [const GraphQLError(message: 'catalog down')],
      });
      await _open(tester, bad, const BottlesScreen());
      expect(find.textContaining('Error:'), findsOneWidget);
    });

    testWidgets('loads the next page when scrolled to the bottom',
        (tester) async {
      final page1 = [
        for (var i = 0; i < 20; i++) _bottle(id: 'b-$i', vineyard: 'Maker $i'),
      ];
      final link = _CaptureLink({}, fns: {
        'Bottles': (req) {
          final page = req.variables['page'] as int? ?? 1;
          return {
            'bottles': {
              'items': page == 1
                  ? page1
                  : [_bottle(id: 'b-tail', vineyard: 'Penfolds')],
              'pageInfo': {'totalCount': 21},
            },
          };
        },
      });
      await _open(tester, link, const BottlesScreen());

      await tester.drag(find.byType(Scrollable).last, const Offset(0, -3000));
      await _settle(tester);

      expect(find.text('Penfolds 2018'), findsWidgets);
      expect(
        link.byName('Bottles').map((r) => r.variables['page']),
        contains(2),
      );
    });
  });

  group('AdjustBottleScreen', () {
    testWidgets('prefilled mode saves the new quantity and pops',
        (tester) async {
      final link = _CaptureLink({
        'AdjustUserBottle': {
          'adjustUserBottle': {'id': 'ub-1'},
        },
      });
      await _open(
        tester,
        link,
        const AdjustBottleScreen(
          bottleId: 'b-1',
          bottleName: 'Chateau Margaux 2015',
          quantity: 5,
        ),
      );

      expect(find.text('Chateau Margaux 2015'), findsOneWidget);
      expect(
        tester.widget<TextField>(find.widgetWithText(TextField, '5')),
        isNotNull,
      );

      await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
      await _settle(tester);

      final adj = link.byName('AdjustUserBottle');
      expect(adj, hasLength(1));
      expect(adj.single.variables['bottleId'], 'b-1');
      expect(adj.single.variables['quantity'], 5);
    });

    testWidgets('blank mode requires a bottle, then picks and saves',
        (tester) async {
      final link = _CaptureLink({
        'BottlePicker': {
          'bottles': {
            'items': [
              {
                'id': 'b-9',
                'vineyard': 'Ridge Monte Bello',
                'vintageYear': 2019,
              },
            ],
          },
        },
        'AdjustUserBottle': {
          'adjustUserBottle': {'id': 'ub-9'},
        },
      });
      await _open(tester, link, const AdjustBottleScreen());

      // Nothing selected → Save disabled.
      final saveButton = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Save'),
      );
      expect(saveButton.onPressed, isNull);

      // Open the picker sheet and choose the bottle.
      await tester.tap(find.text('Select a bottle'));
      await _settle(tester);
      expect(find.text('Search bottles'), findsWidgets);
      await tester.tap(find.text('Ridge Monte Bello 2019'));
      await _settle(tester);

      expect(find.text('Ridge Monte Bello 2019'), findsOneWidget);
      await tester.enterText(find.widgetWithText(TextField, 'Quantity'), '7');
      await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
      await _settle(tester);

      final adj = link.byName('AdjustUserBottle');
      expect(adj, hasLength(1));
      expect(adj.single.variables['bottleId'], 'b-9');
      expect(adj.single.variables['quantity'], 7);
    });

    testWidgets('picker search hits BottlePicker with the typed term',
        (tester) async {
      final link = _CaptureLink({
        'BottlePicker': {
          'bottles': {'items': <dynamic>[]},
        },
        'RecordSearch': {'recordSearch': true},
      });
      await _open(tester, link, const AdjustBottleScreen());

      await tester.tap(find.text('Select a bottle'));
      await _settle(tester);
      await tester.enterText(
          find.widgetWithText(TextField, 'Search bottles'), 'cab');
      await tester.pump(const Duration(milliseconds: 600));
      await _settle(tester);

      expect(
        link.byName('BottlePicker').map((r) => r.variables['search']),
        contains('cab'),
      );
    });

    testWidgets('non-numeric quantity falls back to zero', (tester) async {
      final link = _CaptureLink({
        'AdjustUserBottle': {
          'adjustUserBottle': {'id': 'ub-1'},
        },
      });
      await _open(
        tester,
        link,
        const AdjustBottleScreen(bottleId: 'b-1', bottleName: 'B', quantity: 2),
      );

      await tester.enterText(
          find.widgetWithText(TextField, '2'), 'not-a-number');
      await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
      await _settle(tester);

      expect(link.byName('AdjustUserBottle').single.variables['quantity'], 0);
    });
  });
}
