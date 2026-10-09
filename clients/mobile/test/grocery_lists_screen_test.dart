import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/screens/grocery_lists_screen.dart';
import 'package:lena_mobile/widgets/paged_list_view.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

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

  /// Captured request bodies whose operation text contains [needle].
  List<Map<String, dynamic>> matching(String needle) =>
      requests.where((b) => (b['query'] as String).contains(needle)).toList();
}

// The normalized cache requires __typename on every object the response
// returns; fields the fixture omits resolve as null.
Map<String, dynamic> _listsResponse(
  List<Map<String, dynamic>> lists, {
  int? totalCount,
}) =>
    {
      'data': {
        '__typename': 'Query',
        'groceryLists': {
          '__typename': 'GroceryListPage',
          'items': lists,
          'pageInfo': {
            '__typename': 'PageInfo',
            'pageNumber': 1,
            'pageSize': 25,
            'totalCount': totalCount ?? lists.length,
          },
        },
      },
    };

Map<String, dynamic> _list(String id, String generatedAt) => {
      '__typename': 'GroceryList',
      'id': id,
      'generatedAt': generatedAt,
    };

Future<void> _pump(WidgetTester tester, _MockGraphQL mock) {
  return tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(mock.client),
      child: const MaterialApp(home: GroceryListsScreen()),
    ),
  );
}

void main() {
  group('GroceryListsScreen', () {
    testWidgets('shows the skeleton while loading', (tester) async {
      final mock = _MockGraphQL(
          (_) => _listsResponse([_list('1', '2026-10-04T18:48:42Z')]));
      await _pump(tester, mock);
      expect(find.byType(SkeletonList), findsOneWidget);
      await tester.pumpAndSettle();
    });

    testWidgets('shows the error when the query fails', (tester) async {
      final mock = _MockGraphQL((_) => {
            'errors': [
              {'message': 'boom'},
            ],
          });
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      expect(find.textContaining('Error:'), findsOneWidget);
    });

    testWidgets('shows the empty state with no lists', (tester) async {
      final mock = _MockGraphQL((_) => _listsResponse(const []));
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      expect(find.text('No grocery lists yet'), findsOneWidget);
      expect(find.text('Generate one from a meal plan.'), findsOneWidget);
    });

    testWidgets('renders generated dates for a populated list', (tester) async {
      final mock = _MockGraphQL((_) => _listsResponse([
            _list('1', '2026-10-04T18:48:42Z'),
            _list('2', '2026-10-10T08:00:00Z'),
          ]));
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      expect(find.text('List 1'), findsOneWidget);
      expect(find.text('List 2'), findsOneWidget);
      expect(find.textContaining('Generated'), findsNWidgets(2));
    });

    testWidgets('tapping a list pushes the detail screen', (tester) async {
      final mock = _MockGraphQL((body) {
        final q = body['query'] as String;
        if (q.contains('groceryRouteGroups')) {
          return {
            'data': {
              '__typename': 'Query',
              'groceryList': {
                '__typename': 'GroceryList',
                'id': '1',
                'store': null,
              },
              'groceryStores': [],
              'shopperProviders': [],
              'groceryRouteGroups': [],
            },
          };
        }
        return _listsResponse([_list('1', '2026-10-04T18:48:42Z')]);
      });
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.tap(find.text('List 1'));
      await tester.pumpAndSettle();
      expect(find.text('Grocery List'), findsOneWidget);
    });

    testWidgets('expanded width keeps the list and shows the detail inline',
        (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      final mock = _MockGraphQL((body) {
        final q = body['query'] as String;
        if (q.contains('groceryRouteGroups')) {
          return {
            'data': {
              '__typename': 'Query',
              'groceryList': {
                '__typename': 'GroceryList',
                'id': '1',
                'store': null,
              },
              'groceryStores': [],
              'shopperProviders': [],
              'groceryRouteGroups': [],
            },
          };
        }
        return _listsResponse([_list('1', '2026-10-04T18:48:42Z')]);
      });
      await _pump(tester, mock);
      await tester.pumpAndSettle();

      // Nothing selected — placeholder occupies the detail pane.
      expect(find.text('Select a list'), findsOneWidget);

      await tester.tap(find.text('List 1'));
      await tester.pumpAndSettle();

      // Inline pane: list stays visible next to the detail screen.
      expect(find.text('List 1'), findsOneWidget);
      expect(find.text('Grocery List'), findsOneWidget);
      expect(find.text('Select a list'), findsNothing);
    });

    testWidgets('fetches page 2 when the list fills beyond the viewport',
        (tester) async {
      final page1 = [
        for (var i = 0; i < 25; i++) _list('${i + 1}', '2026-10-04T18:48:42Z'),
      ];
      final mock = _MockGraphQL((body) {
        final vars = body['variables'] as Map<String, dynamic>;
        if (vars['page'] == 2) {
          return {
            'data': {
              '__typename': 'Query',
              'groceryLists': {
                '__typename': 'GroceryListPage',
                'items': [_list('26', '2026-10-05T00:00:00Z')],
                'pageInfo': {
                  '__typename': 'PageInfo',
                  'pageNumber': 2,
                  'pageSize': 25,
                  'totalCount': 26,
                },
              },
            },
          };
        }
        return _listsResponse(page1, totalCount: 26);
      });
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      // Underfilled viewport auto-loads, otherwise drag to the bottom.
      if (mock.matching('groceryLists').length == 1) {
        await tester.drag(find.byType(PagedListView), const Offset(0, -6000));
        await tester.pumpAndSettle();
      }
      final pages = mock
          .matching('groceryLists')
          .map((b) => (b['variables'] as Map)['page'])
          .toSet();
      expect(pages, contains(2));
      expect(find.text('List 26'), findsOneWidget);
    });

    testWidgets('pull-to-refresh refetches the list', (tester) async {
      final mock = _MockGraphQL(
          (_) => _listsResponse([_list('1', '2026-10-04T18:48:42Z')]));
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.fling(
          find.byType(PagedListView), const Offset(0, 300), 1000);
      await tester.pumpAndSettle();
      expect(mock.matching('groceryLists').length, greaterThan(1));
    });
  });

  group('GenerateGroceryDialog', () {
    Map<String, dynamic> dialogResponse(Map<String, dynamic> body) {
      final q = body['query'] as String;
      if (q.contains('generateGroceryList')) {
        return {
          'data': {
            '__typename': 'Mutation',
            'generateGroceryList': {
              '__typename': 'GroceryList',
              'id': '99',
            },
          },
        };
      }
      if (q.contains('mealPlans')) {
        return {
          'data': {
            '__typename': 'Query',
            'mealPlans': {
              '__typename': 'MealPlanPage',
              'items': [
                {'__typename': 'MealPlan', 'id': '7', 'name': 'Week of Oct 5'},
                {'__typename': 'MealPlan', 'id': '8', 'name': 'Week of Oct 12'},
              ],
              'pageInfo': {'__typename': 'PageInfo', 'totalCount': 2},
            },
          },
        };
      }
      return _listsResponse(const []);
    }

    testWidgets('FAB opens the dialog and lists meal plans', (tester) async {
      final mock = _MockGraphQL(dialogResponse);
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();
      expect(find.text('Generate Grocery List'), findsOneWidget);
      await tester.tap(find.byType(DropdownButtonFormField<String?>));
      await tester.pumpAndSettle();
      expect(find.text('Week of Oct 5'), findsWidgets);
      expect(find.text('Week of Oct 12'), findsWidgets);
    });

    testWidgets('generating calls the mutation and dismisses the dialog',
        (tester) async {
      final mock = _MockGraphQL(dialogResponse);
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(DropdownButtonFormField<String?>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Week of Oct 5').last);
      await tester.pumpAndSettle();

      await tester.tap(find.text('Generate'));
      await tester.pumpAndSettle();

      final mutations = mock.matching('generateGroceryList');
      expect(mutations, hasLength(1));
      expect((mutations.single['variables'] as Map)['mealPlanId'], '7');
      // Dialog dismissed; refetch fired for the lists query.
      expect(find.text('Generate Grocery List'), findsNothing);
      expect(mock.matching('groceryLists').length, greaterThan(1));
    });

    testWidgets('generate without a selected plan sends no mutation',
        (tester) async {
      final mock = _MockGraphQL(dialogResponse);
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Generate'));
      await tester.pumpAndSettle();
      expect(mock.matching('generateGroceryList'), isEmpty);
      expect(find.text('Generate Grocery List'), findsOneWidget);
    });

    testWidgets('cancel dismisses without generating', (tester) async {
      final mock = _MockGraphQL(dialogResponse);
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(find.text('Generate Grocery List'), findsNothing);
      expect(mock.matching('generateGroceryList'), isEmpty);
    });

    testWidgets('shows the loading dialog while meal plans fetch',
        (tester) async {
      final mock = _MockGraphQL(dialogResponse);
      await _pump(tester, mock);
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsWidgets);
      await tester.pumpAndSettle();
    });
  });
}
