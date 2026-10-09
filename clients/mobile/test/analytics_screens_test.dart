import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/screens/bottles_screen.dart';
import 'package:lena_mobile/screens/edit_meal_plan_screen.dart';
import 'package:lena_mobile/screens/items_screen.dart';
import 'package:lena_mobile/screens/pantry_screen.dart';
import 'package:lena_mobile/widgets/recipe_picker.dart';

/// graphql_flutter leaves [Operation.operationName] null on single-operation
/// documents, so the name has to come from the document AST.
String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

/// Records every outgoing request and answers it with canned data keyed by
/// operation name, so widget tests can assert on the GraphQL traffic a screen
/// generates (search args + analytics mutations).
class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, Map<String, dynamic>> responses;

  _CaptureLink(this.responses);

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    final data = responses[_opName(request)] ?? <String, dynamic>{};
    yield Response(data: data, response: const <String, dynamic>{});
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

Widget _app(_CaptureLink link, Widget home) => GraphQLProvider(
      client: ValueNotifier(_client(link)),
      child: MaterialApp(home: home),
    );

void main() {
  testWidgets('ItemsScreen renders search field and item rows', (tester) async {
    final link = _CaptureLink({
      'Items': {
        '__typename': 'Query',
        'items': {
          '__typename': 'ItemPage',
          'items': [
            {
              '__typename': 'Item',
              'id': '1',
              'name': 'Milk',
              'unit': 'gal',
              'brand': null,
              'category': null,
            }
          ],
          'pageInfo': {'__typename': 'PageInfo', 'totalCount': 1},
        },
      },
    });
    await tester.pumpWidget(_app(link, const ItemsScreen()));
    await tester.pumpAndSettle();

    expect(find.text('Items'), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
    expect(find.text('Milk'), findsOneWidget);
  });

  testWidgets('ItemsScreen search sends search var and records item search',
      (tester) async {
    final link = _CaptureLink({
      'Items': {
        'items': {
          'items': <dynamic>[],
          'pageInfo': {'totalCount': 0}
        },
      },
      'RecordSearch': {'recordSearch': true},
    });
    await tester.pumpWidget(_app(link, const ItemsScreen()));
    await tester.pump();

    await tester.enterText(find.byType(TextField), 'milk');
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pump();

    final queries = link.byName('Items');
    expect(queries.any((r) => r.variables['search'] == 'milk'), isTrue);

    final searches = link.byName('RecordSearch');
    expect(searches, hasLength(1));
    expect(searches.single.variables['entityType'], 'item');
    expect(searches.single.variables['term'], 'milk');
  });

  testWidgets('PantryScreen renders search field', (tester) async {
    final link = _CaptureLink({
      'Pantry': {
        'userItems': {
          'items': <dynamic>[],
          'pageInfo': {'totalCount': 0},
        },
      },
    });
    await tester.pumpWidget(_app(link, const PantryScreen()));
    await tester.pump();

    expect(find.text('Pantry'), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
  });

  testWidgets('PantryScreen search sends userItems search var and records it',
      (tester) async {
    final link = _CaptureLink({
      'Pantry': {
        'userItems': {
          'items': <dynamic>[],
          'pageInfo': {'totalCount': 0},
        },
      },
      'RecordSearch': {'recordSearch': true},
    });
    await tester.pumpWidget(_app(link, const PantryScreen()));
    await tester.pump();

    await tester.enterText(find.byType(TextField), 'rice');
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pump();

    final queries = link.byName('Pantry');
    expect(queries.any((r) => r.variables['search'] == 'rice'), isTrue);

    final searches = link.byName('RecordSearch');
    expect(searches, hasLength(1));
    expect(searches.single.variables['entityType'], 'item');
  });

  testWidgets('BottlesScreen search records a bottle search', (tester) async {
    final link = _CaptureLink({
      'Bottles': {
        'bottles': {
          'items': <dynamic>[],
          'pageInfo': {'totalCount': 0}
        },
      },
      'RecordSearch': {'recordSearch': true},
    });
    await tester.pumpWidget(_app(link, const BottlesScreen()));
    await tester.pump();

    await tester.enterText(find.byType(TextField), 'merlot');
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pump();

    final queries = link.byName('Bottles');
    expect(queries.any((r) => r.variables['search'] == 'merlot'), isTrue);

    final searches = link.byName('RecordSearch');
    expect(searches, hasLength(1));
    expect(searches.single.variables['entityType'], 'bottle');
    expect(searches.single.variables['term'], 'merlot');
  });

  testWidgets(
      'EditMealPlanScreen recipe picker queries server-side with mealType',
      (tester) async {
    final link = _CaptureLink({
      'MealPlan': {
        'mealPlan': {
          '__typename': 'MealPlan',
          'id': '1',
          'name': 'Week',
          'weekStartDate': '2026-09-28',
          'isActive': true,
          'slots': <dynamic>[],
        },
      },
      'RecipePicker': {
        'recipes': {
          '__typename': 'RecipePage',
          'items': [
            {'__typename': 'Recipe', 'id': '9', 'name': 'Pancakes'},
          ],
        },
      },
      'RecipeCategoryGroups': {
        'recipeCategoryGroups': <dynamic>[],
      },
      'Items': {
        'items': {'__typename': 'ItemPage', 'items': <dynamic>[]},
      },
    });
    await tester
        .pumpWidget(_app(link, const EditMealPlanScreen(mealPlanId: '1')));
    await tester.pump();

    // Meal type is a dropdown now — pick Lunch so the picker's
    // server-side mealType variable is provably driven by it.
    await tester.dragUntilVisible(
      find.text('Meal type'),
      find.byType(ListView).last,
      const Offset(0, -200),
    );
    await tester.tap(find.text('Meal type'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Lunch').last);
    await tester.pumpAndSettle();

    // No eager recipes fetch — the picker queries on open with the meal
    // type as a server-side variable.
    expect(link.byName('Recipes'), isEmpty);
    await tester.ensureVisible(find.byType(RecipePickerField));
    await tester.tap(find.byType(RecipePickerField));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    final picks = link.byName('RecipePicker');
    expect(picks, isNotEmpty);
    expect(picks.first.variables['mealType'], 'Lunch');
    expect(find.text('Pancakes'), findsOneWidget);
  });

  testWidgets('debounce collapses rapid input into one RecordSearch',
      (tester) async {
    final link = _CaptureLink({
      'Items': {
        'items': {
          'items': <dynamic>[],
          'pageInfo': {'totalCount': 0}
        },
      },
      'RecordSearch': {'recordSearch': true},
    });
    await tester.pumpWidget(_app(link, const ItemsScreen()));
    await tester.pump();

    await tester.enterText(find.byType(TextField), 'm');
    await tester.pump(const Duration(milliseconds: 100));
    await tester.enterText(find.byType(TextField), 'mi');
    await tester.pump(const Duration(milliseconds: 100));
    await tester.enterText(find.byType(TextField), 'mil');
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pump();

    final searches = link.byName('RecordSearch');
    expect(searches, hasLength(1));
    expect(searches.single.variables['term'], 'mil');
  });
}
