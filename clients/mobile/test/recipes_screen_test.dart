import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/recipes_screen.dart';
import 'package:lena_mobile/screens/recipe_view_screen.dart';
import 'package:lena_mobile/screens/edit_recipe_screen.dart';
import 'package:lena_mobile/screens/edit_meal_plan_screen.dart';
import 'package:lena_mobile/widgets/paged_list_view.dart';

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

class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, Map<String, dynamic>> responses;
  final Map<String, List<GraphQLError>> errors;
  final Map<String, Map<String, dynamic> Function(Request)> fns;

  _CaptureLink(this.responses,
      {this.errors = const {}, this.fns = const {}});

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    final op = _opName(request);
    yield Response(
      data: _withTypename(
              fns[op]?.call(request) ?? responses[op] ?? <String, dynamic>{})
          as Map<String, dynamic>,
      errors: errors[op],
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

Map<String, dynamic> _recipe({
  required String id,
  String? name,
  String? description,
  int? servings,
  bool isFavorite = false,
}) =>
    {
      'id': id,
      'name': name ?? 'Recipe $id',
      'description': description,
      'servings': servings,
      'prepTimeMinutes': null,
      'cookTimeMinutes': null,
      'isFavorite': isFavorite,
    };

Map<String, dynamic> _list(List<Map<String, dynamic>> items, int total) => {
      'recipes': {
        'items': items,
        'pageInfo': {'totalCount': total},
      },
    };

const _groups = {
  'recipeCategoryGroups': [
    {
      'id': 'g1',
      'name': 'Course',
      'exclusive': true,
      'displayOrder': 1,
      'categories': [
        {'id': 'c1', 'name': 'Dinner'},
        {'id': 'c2', 'name': 'Lunch'},
      ],
    },
    {
      'id': 'g2',
      'name': 'Cuisine',
      'exclusive': false,
      'displayOrder': 2,
      'categories': [
        {'id': 'c3', 'name': 'Italian'},
      ],
    },
  ],
};

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
      child: const MaterialApp(home: RecipesScreen()),
    );

void main() {
  testWidgets('RecipesScreen renders search field and filter actions',
      (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: RecipesScreen()),
      ),
    );
    await tester.pump();

    expect(find.text('Recipes'), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
    expect(find.byTooltip('Favorites only'), findsOneWidget);
    expect(find.byTooltip('Filter by category'), findsOneWidget);
  });

  testWidgets('RecipesScreen search field accepts input', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: RecipesScreen()),
      ),
    );
    await tester.pump();

    await tester.enterText(find.byType(TextField), 'pasta');
    await tester.pump();
    expect(find.text('pasta'), findsOneWidget);
  });

  testWidgets('RecipesScreen filter button opens the category sheet',
      (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: RecipesScreen()),
      ),
    );
    await tester.pump();

    await tester.tap(find.byTooltip('Filter by category'));
    await tester.pump();

    expect(find.text('Filter by category'), findsWidgets);
    expect(find.text('Done'), findsOneWidget);
  });

  testWidgets('EditRecipeScreen create form renders without category section',
      (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: EditRecipeScreen()),
      ),
    );
    await tester.pump();

    expect(find.text('Create Recipe'), findsOneWidget);
    expect(find.text('Categories'), findsNothing);
  });

  testWidgets('EditRecipeScreen edit mode renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: EditRecipeScreen(recipeId: '1')),
      ),
    );
    await tester.pump();

    expect(find.text('Edit Recipe'), findsOneWidget);
  });

  testWidgets('EditMealPlanScreen renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: EditMealPlanScreen(mealPlanId: '1')),
      ),
    );
    await tester.pump();

    expect(find.byType(Scaffold), findsOneWidget);
  });

  group('with mocked GraphQL', () {
    testWidgets('renders recipe rows with servings and descriptions', (
      tester,
    ) async {
      final link = _CaptureLink({
        'Recipes': _list([
          _recipe(
            id: 'r1',
            name: 'Pasta',
            description: 'Boil and sauce',
            servings: 4,
          ),
          _recipe(id: 'r2', name: 'Soup', servings: null, isFavorite: true),
        ], 2),
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      expect(find.text('Pasta'), findsOneWidget);
      expect(find.text('Boil and sauce'), findsOneWidget);
      expect(find.text('Serves 4'), findsOneWidget);
      expect(find.text('Serves -'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(Card),
          matching: find.byIcon(Icons.star),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byType(Card),
          matching: find.byIcon(Icons.star_border),
        ),
        findsOneWidget,
      );
    });

    testWidgets('empty result shows the empty state', (tester) async {
      final link = _CaptureLink({'Recipes': _list(const [], 0)});
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      expect(find.text('No recipes found'), findsOneWidget);
    });

    testWidgets('query error renders the error text', (tester) async {
      final link = _CaptureLink(
        const {},
        errors: {
          'Recipes': [const GraphQLError(message: 'boom')],
        },
      );
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      expect(find.textContaining('Error:'), findsOneWidget);
    });

    testWidgets('row star toggles setRecipeFavorite and refetches', (
      tester,
    ) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1', name: 'Pasta')], 1),
        'SetRecipeFavorite': {'setRecipeFavorite': true},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      final before = link.byName('Recipes').length;
      await tester.tap(
        find.descendant(
          of: find.byType(Card),
          matching: find.byIcon(Icons.star_border),
        ),
      );
      await tester.pumpAndSettle();

      final favs = link.byName('SetRecipeFavorite');
      expect(favs, hasLength(1));
      expect(favs.single.variables['recipeId'], 'r1');
      expect(favs.single.variables['isFavorite'], isTrue);
      expect(link.byName('Recipes').length, greaterThan(before));
    });

    testWidgets('app-bar star filters to favorites only', (tester) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1')], 1),
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Favorites only'));
      await tester.pumpAndSettle();

      final favRequests = link
          .byName('Recipes')
          .where((r) => r.variables['isFavorite'] == true);
      expect(favRequests, isNotEmpty);
    });

    testWidgets('search input debounces into the query and records', (
      tester,
    ) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1')], 1),
        'RecordSearch': {'recordSearch': true},
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField), 'pasta');
      await tester.pump(const Duration(milliseconds: 600));
      await tester.pumpAndSettle();

      final searched =
          link.byName('Recipes').where((r) => r.variables['search'] == 'pasta');
      expect(searched, isNotEmpty);
      expect(link.byName('RecordSearch'), hasLength(1));

      // Clear button resets the search.
      await tester.tap(find.byIcon(Icons.clear));
      await tester.pump(const Duration(milliseconds: 600));
      await tester.pumpAndSettle();
      expect(
        link
            .byName('Recipes')
            .where((r) => r.variables['search'] == null)
            .length,
        greaterThan(1),
      );
    });

    testWidgets('filter sheet toggles category chips', (tester) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1')], 1),
        'RecipeCategoryGroups': _groups,
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Filter by category'));
      await tester.pumpAndSettle();

      expect(find.text('Course (pick one)'), findsOneWidget);
      expect(find.text('Cuisine'), findsOneWidget);

      // Exclusive group: selecting Dinner then Lunch leaves only Lunch.
      await tester.tap(find.text('Dinner'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Lunch'));
      await tester.pumpAndSettle();

      // Non-exclusive FilterChip can stack with the exclusive pick.
      await tester.tap(find.text('Italian'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(ElevatedButton, 'Done'));
      await tester.pumpAndSettle();

      final filtered = link.byName('Recipes').where(
            (r) =>
                r.variables['categoryIds'] is List &&
                (r.variables['categoryIds'] as List).contains('c3'),
          );
      expect(filtered, isNotEmpty);
      // Badge shows 2 active filters.
      expect(find.text('2'), findsOneWidget);

      // Reopen + Clear empties the selection.
      await tester.tap(find.byTooltip('Filter by category'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(TextButton, 'Clear'));
      await tester.pumpAndSettle();
      expect(find.text('2'), findsNothing);
    });

    testWidgets('pagination fetches page 2 on scroll', (tester) async {
      final page1 = [
        for (var i = 0; i < 25; i++) _recipe(id: 'r$i', name: 'Recipe $i'),
      ];
      final page2Items = [
        for (var i = 25; i < 50; i++) _recipe(id: 'r$i', name: 'Recipe $i'),
      ];
      final link = _CaptureLink({}, fns: {
        'Recipes': (req) =>
            _list(req.variables['page'] == 2 ? page2Items : page1, 50),
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      // Fling to the bottom — onLoadMore fires once extentAfter < 300.
      await tester.fling(
        find.byType(PagedListView),
        const Offset(0, -1000),
        2000,
      );
      await tester.pumpAndSettle();

      final page2 =
          link.byName('Recipes').where((r) => r.variables['page'] == 2);
      expect(page2, isNotEmpty);
    });

    testWidgets('row tap navigates to RecipeViewScreen', (tester) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1', name: 'Pasta')], 1),
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pasta'));
      await tester.pump();
      await tester.pump();

      expect(find.byType(RecipeViewScreen), findsOneWidget);
      expect(find.byType(EditRecipeScreen), findsNothing);
    });

    testWidgets('recipe view shows ingredients and steps', (tester) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1', name: 'Pasta')], 1),
        'RecipeView': {
          'recipe': {
            'id': 'r1',
            'name': 'Pasta',
            'items': [
              {
                'id': 'i1',
                'section': 'For the sauce',
                'displayOrder': 0,
                'quantity': 2,
                'unit': 'cups',
                'isOptional': false,
                'notes': null,
                'item': {'id': 'x', 'name': 'Flour'},
                'ingredient': null,
              },
              {
                'id': 'i2',
                'section': null,
                'displayOrder': 1,
                'quantity': 1,
                'unit': 'tsp',
                'isOptional': true,
                'notes': null,
                'item': null,
                'ingredient': {'id': 'y', 'name': 'salt'},
              },
            ],
            'steps': [
              {
                'id': 's2',
                'stepNumber': 2,
                'instruction': 'Serve',
                'durationMinutes': null,
              },
              {
                'id': 's1',
                'stepNumber': 1,
                'instruction': 'Boil water',
                'durationMinutes': 10,
              },
            ],
          },
        },
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pasta'));
      await tester.pumpAndSettle();

      expect(find.byType(RecipeViewScreen), findsOneWidget);
      expect(find.text('Ingredients'), findsOneWidget);
      expect(find.text('For the sauce'), findsOneWidget);
      expect(find.text('2 cups Flour'), findsOneWidget);
      expect(find.text('1 tsp salt (optional)'), findsOneWidget);
      expect(find.text('1. Boil water'), findsOneWidget);
      expect(find.text('10 min'), findsOneWidget);
      expect(find.text('2. Serve'), findsOneWidget);
    });

    testWidgets('recipe view Edit action pushes the editor', (tester) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1', name: 'Pasta')], 1),
        'RecipeView': {
          'recipe': {
            'id': 'r1',
            'name': 'Pasta',
            'items': const <dynamic>[],
            'steps': const <dynamic>[],
          },
        },
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pasta'));
      await tester.pumpAndSettle();
      expect(find.byType(RecipeViewScreen), findsOneWidget);

      await tester.tap(find.byTooltip('Edit'));
      await tester.pump();
      await tester.pump();

      expect(find.byType(EditRecipeScreen), findsOneWidget);
    });

    testWidgets(
        'back from the view preserves search text, results, and scroll',
        (tester) async {
      final page1 = [
        for (var i = 0; i < 25; i++)
          _recipe(id: 'r$i', name: 'Pasta $i'),
      ];
      final page2 = [
        for (var i = 25; i < 40; i++)
          _recipe(id: 'r$i', name: 'Pasta $i'),
      ];
      final link = _CaptureLink(
        {
          'RecordSearch': {'recordSearch': true},
          'RecipeView': {
            'recipe': {
              'id': 'r1',
              'name': 'Pasta 24',
              'items': const <dynamic>[],
              'steps': const <dynamic>[],
            },
          },
        },
        fns: {
          'Recipes': (req) =>
              _list(req.variables['page'] == 2 ? page2 : page1, 40),
        },
      );
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      // Search text + a scroll position to preserve.
      await tester.enterText(find.byType(TextField), 'pasta');
      await tester.pump(const Duration(milliseconds: 600));
      await tester.pumpAndSettle();
      final scrollable = find
          .descendant(
            of: find.byType(PagedListView),
            matching: find.byType(Scrollable),
          )
          .first;
      await tester.scrollUntilVisible(
        find.text('Pasta 24'),
        200,
        scrollable: scrollable,
      );
      await tester.pumpAndSettle();
      final before = tester.state<ScrollableState>(scrollable).position.pixels;
      expect(before, greaterThan(0));

      // Open the view (pushed route on compact widths), then go back.
      await tester.tap(find.text('Pasta 24'));
      await tester.pump();
      await tester.pump();
      expect(find.byType(RecipeViewScreen), findsOneWidget);

      await tester.tap(find.byType(BackButton));
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();

      expect(find.byType(RecipeViewScreen), findsNothing);
      // The list widget stayed mounted — controller text, filtered rows,
      // and scroll offset are untouched.
      expect(find.text('pasta'), findsOneWidget);
      expect(tester.state<ScrollableState>(scrollable).position.pixels,
          before);
      // And the search results are the filtered set, not a refetched list.
      expect(find.text('Pasta 24'), findsOneWidget);
      expect(
        link.byName('Recipes').where((r) => r.variables['search'] == 'pasta'),
        isNotEmpty,
      );
    });

    testWidgets('FAB navigates to the create form', (tester) async {
      final link = _CaptureLink({
        'Recipes': _list([_recipe(id: 'r1')], 1),
      });
      await tester.pumpWidget(_app(link));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(FloatingActionButton));
      await tester.pump();
      await tester.pump();

      expect(find.byType(EditRecipeScreen), findsOneWidget);
      expect(find.text('Create Recipe'), findsOneWidget);
    });
  });
}
