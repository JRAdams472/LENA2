import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/screens/edit_recipe_screen.dart';

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

  _CaptureLink(this.responses, {this.errors = const {}});

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    final op = _opName(request);
    yield Response(
      data: _withTypename(responses[op] ?? <String, dynamic>{})
          as Map<String, dynamic>,
      errors: errors[op],
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

const _catalogs = {
  'Items': {
    'items': {
      'items': [
        {'id': 'i1', 'name': 'Barilla'},
        {'id': 'i2', 'name': 'De Cecco'},
      ],
      'pageInfo': {'totalCount': 2},
    },
  },
  'Ingredients': {
    'ingredients': {
      'items': [
        {'id': 'g1', 'name': 'pasta'},
        {'id': 'g9', 'name': 'GF pasta'},
      ],
      'pageInfo': {'totalCount': 2},
    },
  },
  'Units': {
    'units': [
      {'id': 'u1', 'name': 'cup'},
      {'id': 'u2', 'name': 'lb'},
    ],
  },
  'RecipeCategoryGroups': {
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
    ],
  },
};

Map<String, dynamic> _recipeItem({
  String id = 'ri1',
  String? deltaKind,
  String? ingredientName,
}) =>
    {
      'id': id,
      'deltaKind': deltaKind,
      'item': {'id': 'i1', 'name': 'Barilla'},
      'ingredient': {'id': 'g1', 'name': ingredientName ?? 'pasta'},
      'quantity': 2,
      'unit': 'lb',
      'section': null,
      'displayOrder': 1,
      'notes': null,
      'isOptional': false,
    };

Map<String, dynamic> _recipeStep({String id = 's1', String? deltaKind}) => {
      'id': id,
      'stepNumber': 1,
      'instruction': 'Boil water',
      'durationMinutes': 8,
      'stepType': null,
      'isPassive': false,
      'dependsOnStepNumber': null,
      'appliance': null,
      'deltaKind': deltaKind,
    };

Map<String, dynamic> _recipeResponse({
  Map<String, dynamic>? delta,
  bool withWarnings = false,
  bool household = true,
}) =>
    {
      'recipe': {
        'id': 'r1',
        'name': 'Pasta',
        'description': 'Boil and sauce',
        'servings': 4,
        'prepTimeMinutes': 10,
        'cookTimeMinutes': 20,
        'isFavorite': false,
        'categories': [
          {
            'id': 'c1',
            'name': 'Dinner',
            'group': {
              'id': 'g1',
              'name': 'Course',
              'exclusive': true,
              'displayOrder': 1,
            },
          },
        ],
        'items': [_recipeItem()],
        'steps': [_recipeStep()],
        'householdDelta': delta,
        'allergyWarnings': withWarnings
            ? [
                {
                  'memberKind': 'allergy',
                  'entityKind': 'contains',
                  'member': {
                    'id': '2',
                    'displayName': 'Ada',
                    'firstName': null,
                    'lastName': null,
                  },
                  'allergen': {'id': 'a1', 'name': 'Peanuts'},
                },
              ]
            : const [],
        'allergens': [
          {
            'kind': 'contains',
            'allergen': {'id': 'a1', 'name': 'Peanuts'},
          },
          {
            'kind': 'may_contain',
            'allergen': {'id': 'a2', 'name': 'Soy'},
          },
        ],
      },
      'me': {
        'id': '2',
        'household': household ? {'id': 'h1', 'myRole': 'MEMBER'} : null,
      },
    };

_CaptureLink _link({Map<String, Map<String, dynamic>> extra = const {}}) =>
    _CaptureLink({..._catalogs, ...extra});

Widget _app(_CaptureLink link, {Widget? home, String? recipeId}) =>
    GraphQLProvider(
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
      child: MaterialApp(
        home: home ??
            Scaffold(
              body: Builder(
                builder: (context) => TextButton(
                  onPressed: () => Navigator.push(
                    context,
                    MaterialPageRoute(
                      builder: (_) => EditRecipeScreen(recipeId: recipeId),
                    ),
                  ),
                  child: const Text('open'),
                ),
              ),
            ),
      ),
    );

// The edit form is a long lazy ListView — a tall surface builds every
// section so finders don't depend on scrolling.
void _bigSurface(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 3600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

Future<void> _open(WidgetTester tester) async {
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
}

/// The edit form is a long ListView — scroll [finder] into view.
Future<void> _reveal(WidgetTester tester, Finder finder) =>
    tester.ensureVisible(finder);

void main() {
  group('create mode', () {
    testWidgets('shows the form after catalogs load', (tester) async {
      _bigSurface(tester);
      _bigSurface(tester);
      await tester.pumpWidget(_app(_link()));
      await _open(tester);

      expect(find.text('Create Recipe'), findsOneWidget);
      expect(find.widgetWithText(TextField, ''), findsWidgets);
      expect(find.text('Ingredient'), findsWidgets);
      expect(find.text('Step 1'), findsOneWidget);
      // No household-delta section or categories in create mode.
      expect(find.text('Contents (household view)'), findsNothing);
      expect(find.text('Categories'), findsNothing);
    });

    testWidgets('saves a new recipe and pops', (tester) async {
      final link = _link(extra: {
        'CreateRecipe': const {
          'createRecipe': {'id': 'r-new'},
        },
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link));
      await _open(tester);

      await tester.enterText(
        find.widgetWithText(TextField, 'Name'),
        'Soup',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'Servings'),
        '6',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'Quantity'),
        '3',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'Unit'),
        'cup',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'Instruction'),
        'Simmer',
      );

      await _reveal(tester, find.widgetWithText(ElevatedButton, 'Save'));
      await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
      await tester.pumpAndSettle();

      final calls = link.byName('CreateRecipe');
      expect(calls, hasLength(1));
      final input = calls.single.variables['input'] as Map<String, dynamic>;
      expect(input['name'], 'Soup');
      expect(input['servings'], 6);
      expect((input['items'] as List).single['quantity'], 3.0);
      expect((input['steps'] as List).single['instruction'], 'Simmer');
      // Popped back to the launching route.
      expect(find.text('open'), findsOneWidget);
    });

    testWidgets(
      'ingredient search debounces, selects, and can create a new one',
      (tester) async {
        final link = _link(extra: {
          'GetOrCreateIngredient': const {
            'getOrCreateIngredient': {'id': 'g-new', 'name': 'weird herb'},
          },
          'RecordSearch': const {'recordSearch': true},
          'RecordSelection': const {'recordSelection': true},
        });
        _bigSurface(tester);
        await tester.pumpWidget(_app(link));
        await _open(tester);

        // Debounced search hits the Ingredients query + records it.
        await tester.enterText(
          find.widgetWithText(TextField, 'Search ingredients'),
          'weird',
        );
        await tester.pump(const Duration(milliseconds: 600));
        await tester.pumpAndSettle();

        expect(
          link
              .byName('Ingredients')
              .where((r) => r.variables['search'] == 'weird'),
          isNotEmpty,
        );
        expect(link.byName('RecordSearch'), hasLength(1));

        // The "create" option appears while the search box has text.
        final ingredientDropdown =
            tester.widget<DropdownButtonFormField<String?>>(
          find.byType(DropdownButtonFormField<String?>).first,
        );
        ingredientDropdown.onChanged!('__create__');
        await tester.pumpAndSettle();

        final created = link.byName('GetOrCreateIngredient');
        expect(created, hasLength(1));
        expect(
          (created.single.variables['input'] as Map)['name'],
          'weird',
        );
        // recordSelection fires with the created ingredient's id.
        final sel = link.byName('RecordSelection');
        expect(sel, isNotEmpty);

        // Selecting an existing ingredient records the pick too.
        ingredientDropdown.onChanged!('g9');
        await tester.pumpAndSettle();
        expect(link.byName('RecordSelection').length, greaterThan(1));
      },
    );

    testWidgets('item search debounces and records the brand pick', (
      tester,
    ) async {
      final link = _link(extra: {
        'RecordSearch': const {'recordSearch': true},
        'RecordSelection': const {'recordSelection': true},
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link));
      await _open(tester);

      await tester.enterText(
        find.widgetWithText(TextField, 'Search items'),
        'barilla',
      );
      await tester.pump(const Duration(milliseconds: 600));
      await tester.pumpAndSettle();

      expect(
        link.byName('Items').where((r) => r.variables['search'] == 'barilla'),
        isNotEmpty,
      );
      final searches = link.byName('RecordSearch');
      expect(searches, hasLength(1));
      expect(searches.single.variables['entityType'], 'item');

      final itemDropdown = tester.widget<DropdownButtonFormField<String?>>(
        find.byType(DropdownButtonFormField<String?>).at(1),
      );
      itemDropdown.onChanged!('i2');
      await tester.pumpAndSettle();

      final sel = link
          .byName('RecordSelection')
          .where((r) => r.variables['entityId'] == 'i2');
      expect(sel, isNotEmpty);
    });
  });

  group('edit mode', () {
    testWidgets('prefills the form, flags, warnings, and categories', (
      tester,
    ) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(withWarnings: true),
        'RecordView': const {'recordView': true},
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      expect(find.text('Edit Recipe'), findsOneWidget);

      final name = tester.widget<TextField>(
        find.widgetWithText(TextField, 'Pasta'),
      );
      expect(name.controller?.text, 'Pasta');
      final servings = tester.widget<TextField>(
        find.widgetWithText(TextField, 'Servings'),
      );
      expect(servings.controller?.text, '4');

      // Allergen chips + the member warning card.
      expect(find.text('Peanuts'), findsWidgets);
      expect(find.text('Soy (may contain)'), findsOneWidget);
      expect(
        find.text('Ada — Peanuts (allergy; contains)'),
        findsOneWidget,
      );

      // Category chip seeded from the recipe.
      expect(find.text('Dinner'), findsWidgets);
      expect(find.text('Course (pick one)'), findsOneWidget);

      // Household view contents rows.
      expect(find.text('Contents (household view)'), findsOneWidget);
      expect(find.text('2 lb pasta'), findsOneWidget);
      expect(find.text('1. Boil water'), findsOneWidget);

      // recordView fired for the recipe.
      final views = link.byName('RecordView');
      expect(views, hasLength(1));
      expect(views.single.variables['entityId'], 'r1');
    });

    testWidgets('saving sends updateRecipe', (tester) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(),
        'UpdateRecipe': const {
          'updateRecipe': {'id': 'r1'},
        },
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      await tester.enterText(
        find.widgetWithText(TextField, 'Pasta'),
        'Pasta v2',
      );

      await _reveal(tester, find.widgetWithText(ElevatedButton, 'Save'));
      await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
      await tester.pumpAndSettle();

      final calls = link.byName('UpdateRecipe');
      expect(calls, hasLength(1));
      expect(calls.single.variables['id'], 'r1');
      expect(
        (calls.single.variables['input'] as Map)['name'],
        'Pasta v2',
      );
      expect(find.text('open'), findsOneWidget);
    });

    testWidgets('the app-bar star toggles the favorite', (tester) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(),
        'SetRecipeFavorite': const {'setRecipeFavorite': true},
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      await tester.tap(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.byIcon(Icons.star_border),
        ),
      );
      await tester.pumpAndSettle();

      final calls = link.byName('SetRecipeFavorite');
      expect(calls, hasLength(1));
      expect(calls.single.variables['recipeId'], 'r1');
      expect(calls.single.variables['isFavorite'], isTrue);
    });

    testWidgets('category chips save through setRecipeCategories', (
      tester,
    ) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(),
        'SetRecipeCategories': const {
          'setRecipeCategories': {'id': 'r1'},
        },
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      // Dinner is initially selected (exclusive group) — tapping Lunch
      // swaps the selection to just Lunch.
      await _reveal(tester, find.text('Lunch'));
      await tester.tap(find.text('Lunch'));
      await tester.pumpAndSettle();

      final calls = link.byName('SetRecipeCategories');
      expect(calls, hasLength(1));
      expect(calls.single.variables['recipeId'], 'r1');
      expect(calls.single.variables['categoryIds'], ['c2']);
    });

    testWidgets('a rejected category change rolls back with a snackbar', (
      tester,
    ) async {
      final link = _CaptureLink(
        {..._catalogs, 'Recipe': _recipeResponse()},
        errors: {
          'SetRecipeCategories': [
            const GraphQLError(message: 'admin only'),
          ],
        },
      );
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      await _reveal(tester, find.text('Lunch'));
      await tester.tap(find.text('Lunch'));
      await tester.pumpAndSettle();

      expect(
        find.text('Could not update categories'),
        findsOneWidget,
      );
      // Rolled back: Dinner still selected in the chip row.
      final dinner = tester.widget<ChoiceChip>(
        find.widgetWithText(ChoiceChip, 'Dinner'),
      );
      expect(dinner.selected, isTrue);
    });
  });

  group('household delta', () {
    Map<String, dynamic> deltaFixture() => {
          'id': 'd1',
          'stale': true,
          'orphanedItemCount': 1,
          'orphanedStepCount': 0,
          'updatedAt': '2026-01-05T00:00:00Z',
          'items': [
            {
              'id': 'di1',
              'kind': 'substitute',
              'recipeItemId': 'ri1',
              'item': null,
              'ingredient': {'id': 'g9', 'name': 'GF pasta'},
              'quantity': 3,
              'unit': 'lb',
              'unitId': null,
              'section': null,
              'displayOrder': null,
              'notes': null,
              'isOptional': false,
              'orphaned': true,
            },
          ],
          'steps': const [],
        };

    testWidgets(
      'stale delta shows the review card and acknowledges it',
      (tester) async {
        final link = _link(extra: {
          'Recipe': _recipeResponse(delta: deltaFixture()),
          'AcknowledgeRecipeDelta': const {
            'acknowledgeRecipeDelta': {'id': 'd1'},
          },
        });
        _bigSurface(tester);
        await tester.pumpWidget(_app(link, recipeId: 'r1'));
        await _open(tester);

        expect(find.text('Household version'), findsOneWidget);
        expect(
          find.textContaining('mark as checked when happy'),
          findsOneWidget,
        );
        expect(
          find.textContaining('Some tweaks no longer apply'),
          findsOneWidget,
        );
        // The orphaned draft shows its description + chip.
        expect(find.text('Swap pasta for GF pasta'), findsOneWidget);
        expect(find.text('No longer applies'), findsOneWidget);

        await _reveal(tester, find.text('Mark reviewed'));
        await tester.tap(find.text('Mark reviewed'));
        await tester.pumpAndSettle();

        final calls = link.byName('AcknowledgeRecipeDelta');
        expect(calls, hasLength(1));
        expect(calls.single.variables['recipeId'], 'r1');
      },
    );

    testWidgets('the Original segment reloads the canonical view', (
      tester,
    ) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(delta: deltaFixture()),
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      await _reveal(tester, find.text('Original'));
      await tester.tap(find.text('Original'));
      await tester.pumpAndSettle();

      final canonical = link
          .byName('Recipe')
          .where((r) => r.variables['view'] == 'canonical');
      expect(canonical, isNotEmpty);
    });

    testWidgets('Clear all removes the household delta', (tester) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(delta: deltaFixture()),
        'ClearRecipeDelta': const {'clearRecipeDelta': true},
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      await _reveal(tester, find.text('Clear all'));
      await tester.tap(find.text('Clear all'));
      await tester.pumpAndSettle();

      final calls = link.byName('ClearRecipeDelta');
      expect(calls, hasLength(1));
      expect(calls.single.variables['recipeId'], 'r1');

      // Drafts list collapses back to the empty state.
      expect(
        find.textContaining('No tweaks yet'),
        findsOneWidget,
      );
    });

    testWidgets('removing a draft row drops it from the change set', (
      tester,
    ) async {
      final link = _link(extra: {
        'Recipe': _recipeResponse(delta: deltaFixture()),
      });
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      expect(find.text('Swap pasta for GF pasta'), findsOneWidget);
      await _reveal(tester, find.byTooltip('Remove tweak'));
      await tester.tap(find.byTooltip('Remove tweak'));
      await tester.pumpAndSettle();

      expect(find.text('Swap pasta for GF pasta'), findsNothing);
      expect(find.textContaining('No tweaks yet'), findsOneWidget);
      expect(find.text('Unsaved changes'), findsOneWidget);
    });

    testWidgets(
      'tweaking a contents line through the sheet creates a draft',
      (tester) async {
        final link = _link(extra: {
          'Recipe': _recipeResponse(),
        });
        _bigSurface(tester);
        await tester.pumpWidget(_app(link, recipeId: 'r1'));
        await _open(tester);

        // The item contents row gets a Tweak button in effective view.
        await _reveal(tester, find.text('Tweak').first);
        await tester.tap(find.text('Tweak').first);
        await tester.pumpAndSettle();

        expect(find.text('Tweak pasta'), findsOneWidget);
        // Remove is a one-tap draft — no fields required.
        await tester.tap(find.text('Remove'));
        await tester.pump();
        await tester.tap(find.widgetWithText(FilledButton, 'Apply tweak'));
        await tester.pumpAndSettle();

        expect(find.text('Remove pasta'), findsOneWidget);
        expect(find.text('Unsaved changes'), findsOneWidget);

        // Save the change set.
        link.responses['SetRecipeDelta'] = const {
          'setRecipeDelta': {'id': 'd-new'},
        };
        await _reveal(
          tester,
          find.widgetWithText(FilledButton, 'Save tweaks'),
        );
        await tester.tap(
          find.widgetWithText(FilledButton, 'Save tweaks'),
        );
        await tester.pumpAndSettle();

        final calls = link.byName('SetRecipeDelta');
        expect(calls, hasLength(1));
        final items = calls.single.variables['items'] as List;
        expect(items.single['kind'], 'remove');
        expect(items.single['recipeItemId'], 'ri1');
      },
    );

    testWidgets(
      'add-ingredient and add-step sheets create drafts, then discard',
      (tester) async {
        final link = _link(extra: {
          'Recipe': _recipeResponse(),
        });
        _bigSurface(tester);
        await tester.pumpWidget(_app(link, recipeId: 'r1'));
        await _open(tester);

        // Add-ingredient sheet: pick an ingredient and apply.
        await _reveal(
          tester,
          find.widgetWithText(OutlinedButton, 'Add ingredient'),
        );
        await tester.tap(
          find.widgetWithText(OutlinedButton, 'Add ingredient'),
        );
        await tester.pumpAndSettle();

        expect(find.text('Add an ingredient line'), findsOneWidget);
        final sheetIngredient = tester.widget<DropdownButtonFormField<String?>>(
          find
              .descendant(
                of: find.byType(BottomSheet),
                matching: find.byType(DropdownButtonFormField<String?>),
              )
              .first,
        );
        sheetIngredient.onChanged!('g9');
        await tester.pump();
        await tester.tap(find.widgetWithText(FilledButton, 'Apply tweak'));
        await tester.pumpAndSettle();
        expect(find.text('Add GF pasta'), findsOneWidget);

        // Add-step sheet: fill directions and apply.
        await _reveal(
          tester,
          find.widgetWithText(OutlinedButton, 'Add step'),
        );
        await tester.tap(
          find.widgetWithText(OutlinedButton, 'Add step'),
        );
        await tester.pumpAndSettle();

        expect(find.text('Add a step'), findsWidgets);
        await tester.enterText(
          find.widgetWithText(TextField, 'Directions'),
          'Fold gently',
        );
        await tester.pump();
        await tester.tap(find.widgetWithText(FilledButton, 'Apply tweak'));
        await tester.pumpAndSettle();
        expect(find.text('Add a step at 2'), findsOneWidget);

        // Discard restores the pristine change set.
        await _reveal(tester, find.widgetWithText(TextButton, 'Discard'));
        await tester.tap(find.widgetWithText(TextButton, 'Discard'));
        await tester.pumpAndSettle();
        expect(find.textContaining('No tweaks yet'), findsOneWidget);
        expect(find.text('Add GF pasta'), findsNothing);
      },
    );

    testWidgets('a failed delta save shows the snackbar', (tester) async {
      final link = _CaptureLink(
        {..._catalogs, 'Recipe': _recipeResponse(delta: deltaFixture())},
        errors: {
          'SetRecipeDelta': [const GraphQLError(message: 'denied')],
        },
      );
      _bigSurface(tester);
      await tester.pumpWidget(_app(link, recipeId: 'r1'));
      await _open(tester);

      // Removing the existing draft makes the set dirty.
      await _reveal(tester, find.byTooltip('Remove tweak'));
      await tester.tap(find.byTooltip('Remove tweak'));
      await tester.pumpAndSettle();

      await _reveal(
        tester,
        find.widgetWithText(FilledButton, 'Save tweaks'),
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Save tweaks'));
      await tester.pumpAndSettle();

      expect(find.text('Could not save tweaks'), findsOneWidget);
    });

    testWidgets(
      'without a household the tweaks section stays hidden',
      (tester) async {
        final link = _link(extra: {
          'Recipe': _recipeResponse(household: false),
        });
        _bigSurface(tester);
        await tester.pumpWidget(_app(link, recipeId: 'r1'));
        await _open(tester);

        expect(find.text('Edit Recipe'), findsOneWidget);
        expect(find.text('Contents (household view)'), findsNothing);
        expect(find.text('Household tweaks'), findsNothing);
      },
    );
  });
}
