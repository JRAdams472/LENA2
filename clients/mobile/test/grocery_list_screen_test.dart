import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:plugin_platform_interface/plugin_platform_interface.dart';
import 'package:url_launcher_platform_interface/link.dart';
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/grocery_list_screen.dart';

Map<String, dynamic> routeGroup(
        Object? aisleId, String name, List<String> itemIds) =>
    {
      'aisle': aisleId == null ? null : {'id': aisleId, 'name': name},
      'items': [
        for (final id in itemIds)
          {
            'suggested': false,
            'item': {'id': id},
          },
      ],
    };

const _linkUrl = 'https://instacart.example.com/list/abc123';

/// A real GraphQLClient whose HttpLink terminates at a MockClient — the
/// Query widget and client.mutate run their normal pipelines.
class _MockGraphQL {
  _MockGraphQL(this.responder);
  final Map<String, dynamic> Function(Map<String, dynamic> body) responder;
  final requests = <Map<String, dynamic>>[];
  late final client = GraphQLClient(
    cache: GraphQLCache(),
    link: HttpLink(
      'http://test/graphql',
      httpClient: http_testing.MockClient((req) async {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        requests.add(body);
        return http.Response(
          jsonEncode(responder(body)),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    ),
  );
}

// The normalized cache requires __typename on every object the response
// returns; fields the fixture omits resolve as null.
Map<String, dynamic> _routingResponse(
  List<String> providers, {
  bool checked = false,
}) =>
    {
      'data': {
        '__typename': 'Query',
        'groceryList': {'__typename': 'GroceryList', 'id': '1', 'store': null},
        'groceryStores': [],
        'shopperProviders': providers,
        'groceryRouteGroups': [
          {
            '__typename': 'GroceryRouteGroup',
            'aisle': null,
            'items': [
              {
                '__typename': 'GroceryRouteItem',
                'suggested': false,
                'item': {
                  '__typename': 'GroceryListItem',
                  'id': '10',
                  'isChecked': checked,
                  'manualItemName': 'Milk',
                },
              },
            ],
          },
        ],
      },
    };

Future<void> _pumpShop(WidgetTester tester, _MockGraphQL mock) {
  return tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(mock.client),
      child: const MaterialApp(home: GroceryListScreen(listId: '1')),
    ),
  );
}

void main() {
  group('groceryReorderEntries', () {
    final groups = [
      routeGroup('10', 'Produce', ['a', 'b']),
      routeGroup('20', 'Dairy', ['c']),
      routeGroup(null, '', ['d']),
    ];

    test('same-group reorder keeps aisleId unset for all entries', () {
      final entries = groceryReorderEntries(groups, 'a', 0, 1);
      expect(
        entries.map((e) => e['groceryListItemId']).toList(),
        ['b', 'a', 'c', 'd'],
      );
      // Reorder within a group still stamps the moved item's aisle so the
      // server writes rank against the right walk order.
      expect(entries[1]['aisleId'], '10');
      expect(entries[0]['aisleId'], isNull);
      expect(entries[2]['aisleId'], isNull);
    });

    test('cross-group move carries the target aisle on the moved entry', () {
      final entries = groceryReorderEntries(groups, 'a', 1, 0);
      expect(
        entries.map((e) => e['groceryListItemId']).toList(),
        ['b', 'a', 'c', 'd'],
      );
      expect(entries[1]['aisleId'], '20');
      expect(entries.where((e) => e['aisleId'] != null), hasLength(1));
    });

    test('drop at end of a group appends after its last item', () {
      final entries = groceryReorderEntries(groups, 'd', 0, 2);
      expect(
        entries.map((e) => e['groceryListItemId']).toList(),
        ['a', 'b', 'd', 'c'],
      );
      expect(entries[2]['aisleId'], '10');
    });

    test('drop into the unassigned bucket leaves aisleId null', () {
      final entries = groceryReorderEntries(groups, 'a', 2, 0);
      expect(
        entries.map((e) => e['groceryListItemId']).toList(),
        ['b', 'c', 'a', 'd'],
      );
      // Null means "no change" to the server — unassignment is handled by a
      // separate assignItemToAisle call, so no entry here claims an aisle.
      expect(entries[2]['aisleId'], isNull);
    });

    test('unknown moved id yields no entries', () {
      expect(groceryReorderEntries(groups, 'zzz', 0, 0), isEmpty);
    });
  });

  group('needsBrandPick', () {
    Map<String, dynamic> line({
      bool checked = false,
      Map<String, dynamic>? item,
      Map<String, dynamic>? ingredient,
      Map<String, dynamic>? usual,
    }) =>
        {
          'isChecked': checked,
          'item': item,
          'ingredient': ingredient,
          'usualBrand': usual,
        };

    test('ingredient-only line with no usual needs the pick', () {
      expect(
        needsBrandPick(line(ingredient: {'id': '7', 'name': 'corn'})),
        isTrue,
      );
    });

    test('bound item line checks off normally', () {
      expect(
        needsBrandPick(line(
          item: {'id': '5'},
          ingredient: {'id': '7'},
        )),
        isFalse,
      );
    });

    test('ingredient line with a usual brand checks off normally', () {
      expect(
        needsBrandPick(line(
          ingredient: {'id': '7'},
          usual: {'id': '42'},
        )),
        isFalse,
      );
    });

    test('manual line checks off normally', () {
      expect(needsBrandPick(line()), isFalse);
    });

    test('already-checked line unchecks normally', () {
      expect(
        needsBrandPick(line(checked: true, ingredient: {'id': '7'})),
        isFalse,
      );
    });
  });

  testWidgets('GroceryListScreen renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: GroceryListScreen(listId: '1')),
      ),
    );
    await tester.pump();

    expect(find.text('Grocery List'), findsOneWidget);
    expect(find.byType(Scaffold), findsOneWidget);
  });

  group('Shop with Instacart', () {
    testWidgets('hides the action when no provider is configured',
        (tester) async {
      final mock = _MockGraphQL((body) =>
          body['query'].toString().contains('groceryRouteGroups')
              ? _routingResponse([])
              : {});
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      expect(find.byTooltip('Shop with Instacart'), findsNothing);
    });

    testWidgets('creates the link and shows it in a dialog', (tester) async {
      final mock = _MockGraphQL((body) {
        if (body['query'].toString().contains('groceryRouteGroups')) {
          return _routingResponse(['INSTACART']);
        }
        if (body['query'].toString().contains('createShoppingLink')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'createShoppingLink': {
                '__typename': 'ShoppingLink',
                'provider': 'INSTACART',
                'url': _linkUrl
              },
            },
          };
        }
        return {};
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Shop with Instacart'));
      await tester.pumpAndSettle();

      final mutation = mock.requests.firstWhere(
        (r) => r['query'].toString().contains('createShoppingLink'),
      );
      expect(mutation['variables']['groceryListId'], '1');
      expect(mutation['variables']['provider'], 'INSTACART');
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (w) => w is SelectableText && w.data == _linkUrl,
        ),
        findsOneWidget,
      );
    });

    testWidgets('notes when checked items were left off the link',
        (tester) async {
      final mock = _MockGraphQL((body) {
        if (body['query'].toString().contains('groceryRouteGroups')) {
          return _routingResponse(['INSTACART'], checked: true);
        }
        return {
          'data': {
            '__typename': 'Mutation',
            'createShoppingLink': {
              '__typename': 'ShoppingLink',
              'provider': 'INSTACART',
              'url': _linkUrl
            },
          },
        };
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Shop with Instacart'));
      await tester.pumpAndSettle();

      expect(
        find.textContaining('Checked items were left off'),
        findsOneWidget,
      );
    });

    testWidgets('opens the link externally and copies it', (tester) async {
      final launched = <String>[];
      PreferredLaunchMode? launchedMode;
      final fakeLauncher = _FakeUrlLauncher(launched, (m) => launchedMode = m);
      UrlLauncherPlatform.instance = fakeLauncher;
      addTearDown(() => UrlLauncherPlatform.instance = _previousLauncher);

      String? copied;
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(SystemChannels.platform, (call) async {
        if (call.method == 'Clipboard.setData') {
          copied = call.arguments['text'] as String;
        }
        return null;
      });
      addTearDown(
        () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(SystemChannels.platform, null),
      );

      final mock = _MockGraphQL((body) {
        if (body['query'].toString().contains('groceryRouteGroups')) {
          return _routingResponse(['INSTACART']);
        }
        return {
          'data': {
            '__typename': 'Mutation',
            'createShoppingLink': {
              '__typename': 'ShoppingLink',
              'provider': 'INSTACART',
              'url': _linkUrl
            },
          },
        };
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Shop with Instacart'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Open Instacart'));
      await tester.pump();
      expect(launched, [_linkUrl]);
      expect(launchedMode, PreferredLaunchMode.externalApplication);

      await tester.tap(find.text('Copy link'));
      await tester.pump();
      expect(copied, _linkUrl);
      expect(find.text('Link copied'), findsOneWidget);
    });

    testWidgets('shows a snackbar when the mutation fails', (tester) async {
      final mock = _MockGraphQL((body) {
        if (body['query'].toString().contains('groceryRouteGroups')) {
          return _routingResponse(['INSTACART']);
        }
        return {
          'errors': [
            {'message': 'shopping provider unavailable'},
          ],
        };
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Shop with Instacart'));
      await tester.pumpAndSettle();

      expect(find.text('shopping provider unavailable'), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);
    });
  });

  group('list interactions', () {
    Map<String, dynamic> aisle(String id, String name, int position) => {
          '__typename': 'StoreAisle',
          'id': id,
          'name': name,
          'position': position,
        };

    Map<String, dynamic> routeItem(
      String id, {
      Map<String, dynamic>? item,
      Map<String, dynamic>? ingredient,
      Map<String, dynamic>? usualBrand,
      bool checked = false,
      bool suggested = false,
    }) =>
        {
          '__typename': 'GroceryRouteItem',
          'suggested': suggested,
          'item': {
            '__typename': 'GroceryListItem',
            'id': id,
            'item': item,
            'ingredient': ingredient,
            'usualBrand': usualBrand,
            'manualItemName':
                item == null && ingredient == null ? 'Manual $id' : null,
            'quantityNeeded': 2,
            'unitOfMeasure': 'cup',
            'source': 'manual',
            'isChecked': checked,
            'allergyWarnings': [],
            'allergens': [],
          },
        };

    Map<String, dynamic> detailResponse({
      Map<String, dynamic>? store,
      List<Map<String, dynamic>> stores = const [],
      List<Map<String, dynamic>> groups = const [],
    }) =>
        {
          'data': {
            '__typename': 'Query',
            'groceryList': {
              '__typename': 'GroceryList',
              'id': '1',
              'store': store,
            },
            'groceryStores': stores,
            'shopperProviders': [],
            'groceryRouteGroups': groups,
          },
        };

    Map<String, dynamic> storeWithAisles() => {
          '__typename': 'GroceryStore',
          'id': '7',
          'name': 'Costco',
          'aisles': [aisle('1', 'Produce', 0), aisle('2', 'Dairy', 1)],
        };

    List<Map<String, dynamic>> matching(_MockGraphQL mock, String needle) =>
        mock.requests
            .where((b) => (b['query'] as String).contains(needle))
            .toList();

    testWidgets('shows the error state when the query fails', (tester) async {
      final mock = _MockGraphQL((_) => {
            'errors': [
              {'message': 'boom'},
            ],
          });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();
      expect(find.textContaining('Error:'), findsOneWidget);
    });

    testWidgets('checking a bound item toggles it', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('toggleGroceryItemChecked')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'toggleGroceryItemChecked': {
                '__typename': 'GroceryListItem',
                'id': '10',
                'isChecked': true,
              },
            },
          };
        }
        return detailResponse(groups: [
          {
            '__typename': 'GroceryRouteGroup',
            'aisle': null,
            'items': [
              routeItem('10', item: {
                '__typename': 'Item',
                'id': '5',
                'name': 'Milk',
              }),
            ],
          },
        ]);
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.textContaining('Milk'));
      await tester.pumpAndSettle();

      final toggles = matching(mock, 'toggleGroceryItemChecked');
      expect(toggles, hasLength(1));
      expect((toggles.single['variables'] as Map)['groceryListItemId'], '10');
      // Refetch fired after the mutation.
      expect(matching(mock, 'groceryRouteGroups').length, greaterThan(1));
    });

    testWidgets('ingredient-only line prompts for a brand pick',
        (tester) async {
      final mock = _MockGraphQL((body) {
        final q = body['query'] as String;
        if (q.contains('checkGroceryItemWithBrand')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'checkGroceryItemWithBrand': {
                '__typename': 'GroceryListItem',
                'id': '11',
                'isChecked': true,
              },
            },
          };
        }
        if (q.contains('items(')) {
          return {
            'data': {
              '__typename': 'Query',
              'items': {
                '__typename': 'ItemPage',
                'items': [
                  {
                    '__typename': 'Item',
                    'id': '20',
                    'name': 'Roma Tomatoes',
                    'brand': {
                      '__typename': 'Brand',
                      'id': '8',
                      'name': 'Muir Glen',
                    },
                  },
                ],
              },
            },
          };
        }
        return detailResponse(groups: [
          {
            '__typename': 'GroceryRouteGroup',
            'aisle': null,
            'items': [
              routeItem('11', ingredient: {
                '__typename': 'Ingredient',
                'id': '4',
                'name': 'Tomato',
              }),
            ],
          },
        ]);
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.textContaining('Tomato'));
      await tester.pumpAndSettle();

      expect(find.text('Which Tomato did you buy?'), findsOneWidget);
      expect(find.text('Roma Tomatoes'), findsOneWidget);
      expect(find.text('Muir Glen'), findsOneWidget);

      await tester.tap(find.text('Roma Tomatoes'));
      await tester.pumpAndSettle();

      final checks = matching(mock, 'checkGroceryItemWithBrand');
      expect(checks, hasLength(1));
      final vars = checks.single['variables'] as Map;
      expect(vars['groceryListItemId'], '11');
      expect(vars['itemId'], '20');
    });

    testWidgets('cancelling the brand pick sends no mutation', (tester) async {
      final mock = _MockGraphQL((body) {
        final q = body['query'] as String;
        if (q.contains('items(')) {
          return {
            'data': {
              '__typename': 'Query',
              'items': {'__typename': 'ItemPage', 'items': []},
            },
          };
        }
        return detailResponse(groups: [
          {
            '__typename': 'GroceryRouteGroup',
            'aisle': null,
            'items': [
              routeItem('11', ingredient: {
                '__typename': 'Ingredient',
                'id': '4',
                'name': 'Tomato',
              }),
            ],
          },
        ]);
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.textContaining('Tomato'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();

      expect(matching(mock, 'checkGroceryItemWithBrand'), isEmpty);
    });

    testWidgets('adds a manual item', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('addGroceryItem')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'addGroceryItem': {
                '__typename': 'GroceryListItem',
                'id': '30',
              },
            },
          };
        }
        return detailResponse(groups: const []);
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField).at(0), 'Flour');
      await tester.enterText(find.byType(TextField).at(1), '2');
      await tester.enterText(find.byType(TextField).at(2), 'cup');
      await tester.ensureVisible(find.text('Add'));
      await tester.tap(find.text('Add'));
      await tester.pumpAndSettle();

      final adds = matching(mock, 'addGroceryItem');
      expect(adds, hasLength(1));
      final input = (adds.single['variables'] as Map)['input'] as Map;
      expect(input['groceryListId'], '1');
      expect(input['manualItemName'], 'Flour');
      expect(input['quantity'], 2);
      expect(input['unit'], 'cup');
    });

    testWidgets('empty add form is a no-op', (tester) async {
      final mock = _MockGraphQL((_) => detailResponse(groups: const []));
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Add'));
      await tester.tap(find.text('Add'));
      await tester.pumpAndSettle();
      expect(matching(mock, 'addGroceryItem'), isEmpty);
    });

    testWidgets('store picker sets the list store', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('setGroceryListStore')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'setGroceryListStore': {
                '__typename': 'GroceryList',
                'id': '1',
              },
            },
          };
        }
        return detailResponse(stores: [storeWithAisles()]);
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.byType(DropdownButton<String?>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Costco').last);
      await tester.pumpAndSettle();

      final sets = matching(mock, 'setGroceryListStore');
      expect(sets, hasLength(1));
      final vars = sets.single['variables'] as Map;
      expect(vars['groceryListId'], '1');
      expect(vars['storeId'], '7');
    });

    testWidgets('move-to-unassigned calls assignItemToAisle then reorders',
        (tester) async {
      final mock = _MockGraphQL((body) {
        final q = body['query'] as String;
        if (q.contains('assignItemToAisle')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'assignItemToAisle': true,
            },
          };
        }
        if (q.contains('reorderGroceryListItems')) {
          return {
            'data': {
              '__typename': 'Mutation',
              'reorderGroceryListItems': true,
            },
          };
        }
        return detailResponse(
          store: {'__typename': 'GroceryStore', 'id': '7', 'name': 'Costco'},
          stores: [storeWithAisles()],
          groups: [
            {
              '__typename': 'GroceryRouteGroup',
              'aisle': aisle('1', 'Produce', 0),
              'items': [
                routeItem('10', item: {
                  '__typename': 'Item',
                  'id': '5',
                  'name': 'Milk',
                }),
              ],
            },
            {
              '__typename': 'GroceryRouteGroup',
              'aisle': null,
              'items': [
                routeItem('12', item: {
                  '__typename': 'Item',
                  'id': '6',
                  'name': 'Bread',
                }),
              ],
            },
          ],
        );
      });
      await _pumpShop(tester, mock);
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Move to aisle').first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Move to unassigned'));
      await tester.pumpAndSettle();

      final assigns = matching(mock, 'assignItemToAisle');
      expect(assigns, hasLength(1));
      final avars = assigns.single['variables'] as Map;
      expect(avars['storeId'], '7');
      expect(avars['aisleId'], isNull);
      expect(avars['itemId'], '5');

      final reorders = matching(mock, 'reorderGroceryListItems');
      expect(reorders, hasLength(1));
      final entries = (reorders.single['variables'] as Map)['entries'] as List;
      final moved = entries
          .firstWhere((e) => (e as Map)['groceryListItemId'] == '10') as Map;
      expect(moved['aisleId'], isNull);
    });
  });
}

class _FakeUrlLauncher extends UrlLauncherPlatform
    with MockPlatformInterfaceMixin {
  _FakeUrlLauncher(this.urls, this.onMode);
  final List<String> urls;
  final void Function(PreferredLaunchMode) onMode;

  @override
  LinkDelegate? get linkDelegate => null;

  @override
  Future<bool> launchUrl(String url, LaunchOptions options) async {
    urls.add(url);
    onMode(options.mode);
    return true;
  }
}

UrlLauncherPlatform get _previousLauncher => _platformInstance;

// url_launcher_platform_interface keeps a single static instance; capture it
// once so the test can restore the real implementation afterwards.
final UrlLauncherPlatform _platformInstance = UrlLauncherPlatform.instance;
