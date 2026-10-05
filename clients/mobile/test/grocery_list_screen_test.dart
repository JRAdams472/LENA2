import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
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
}
