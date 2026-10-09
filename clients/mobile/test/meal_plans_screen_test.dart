import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lena_mobile/screens/edit_meal_plan_screen.dart';
import 'package:lena_mobile/screens/meal_plans_screen.dart';
import 'package:lena_mobile/widgets/paged_list_view.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

/// The normalized cache drops objects without `__typename`; inject one
/// per object keyed by the field that produced it.
String _typename(String key) {
  var s = key;
  if (s.endsWith('ies')) {
    s = '${s.substring(0, s.length - 3)}y';
  } else if (s.endsWith('s')) {
    s = s.substring(0, s.length - 1);
  }
  return s[0].toUpperCase() + s.substring(1);
}

dynamic _typed(String key, dynamic value) {
  if (value is Map) {
    final out = <String, dynamic>{'__typename': _typename(key)};
    for (final e in value.entries) {
      out[e.key as String] = _typed(e.key as String, e.value);
    }
    return out;
  }
  if (value is List) {
    return value.map((v) => _typed(key, v)).toList();
  }
  return value;
}

Map<String, dynamic> _withTypenames(Map<String, dynamic> data) {
  final out = <String, dynamic>{'__typename': 'Query'};
  for (final e in data.entries) {
    out[e.key] = _typed(e.key, e.value);
  }
  return out;
}

http.Response gqlOk(Map<String, dynamic> data) => http.Response(
      jsonEncode({'data': _withTypenames(data)}),
      200,
      headers: {'content-type': 'application/json'},
    );

http.Response gqlErr() => http.Response(
      jsonEncode({
        'errors': [
          {'message': 'boom'}
        ]
      }),
      200,
      headers: {'content-type': 'application/json'},
    );

Map<String, dynamic> planItem({
  String id = 'p1',
  String name = 'Late October',
  String weekStartDate = '2026-10-26',
  bool isActive = true,
}) =>
    {
      'id': id,
      'name': name,
      'weekStartDate': weekStartDate,
      'isActive': isActive,
    };

Map<String, dynamic> planDetail({
  String id = 'p1',
  String name = 'Late October',
  String weekStartDate = '2026-10-26',
  List<Map<String, dynamic>>? slots,
}) =>
    {
      'id': id,
      'name': name,
      'weekStartDate': weekStartDate,
      'isActive': true,
      'slots': slots ?? [],
    };

Map<String, dynamic> planSlot({
  String id = 's1',
  int dayOfWeek = 1,
  String mealType = 'dinner',
  Map<String, dynamic>? recipe = const {'id': 'r1', 'name': 'Roast Chicken'},
  int? servings = 4,
  String? replacementNote = 'No dairy',
  List<Map<String, dynamic>>? items,
}) =>
    {
      'id': id,
      'dayOfWeek': dayOfWeek,
      'mealType': mealType,
      'servings': servings,
      'replacementNote': replacementNote,
      'recipe': recipe,
      'items': items ?? [],
      'allergyWarnings': [],
      'allergens': [],
    };

/// Captures requests; per-test `onRequest` override runs first, then
/// canned mutation responses, then `onQuery` for read paths.
class _Capture {
  final List<Map<String, String>> requests = [];

  http.Response? Function(String query)? onRequest;
  http.Response? Function(String query)? onQuery;

  Future<http.Response> responder(http.Request req) async {
    final body = jsonDecode(utf8.decode(req.bodyBytes)) as Map<String, dynamic>;
    final query = body['query'] as String;
    requests.add({'query': query, 'variables': jsonEncode(body['variables'])});
    final custom = onRequest?.call(query);
    if (custom != null) return custom;
    return defaults(query);
  }

  http.Response defaults(String query) {
    if (query.contains('recordSelection')) {
      return gqlOk({'recordSelection': true});
    }
    if (query.contains('recordSearch')) {
      return gqlOk({'recordSearch': true});
    }
    if (query.contains('addMealSlotItem')) {
      return gqlOk({
        'addMealSlotItem': {'id': 'si9'}
      });
    }
    if (query.contains('addMealSlot')) {
      return gqlOk({
        'addMealSlot': {'id': 's9'}
      });
    }
    if (query.contains('removeMealSlotItem')) {
      return gqlOk({'removeMealSlotItem': true});
    }
    if (query.contains('removeMealSlot')) {
      return gqlOk({'removeMealSlot': true});
    }
    if (query.contains('createMealPlan')) {
      return gqlOk({
        'createMealPlan': {'id': 'p9'}
      });
    }
    if (query.contains('updateMealPlan')) {
      return gqlOk({
        'updateMealPlan': {'id': 'p1'}
      });
    }
    return onQuery?.call(query) ?? gqlOk(const {});
  }

  int countOf(String fragment) =>
      requests.where((r) => r['query']!.contains(fragment)).length;

  Map<String, String>? lastMatching(String fragment) {
    final hits = requests.where((r) => r['query']!.contains(fragment));
    return hits.isEmpty ? null : hits.last;
  }
}

GraphQLClient _makeClient(_Capture cap) => GraphQLClient(
      link: HttpLink('https://example.test/graphql',
          httpClient: MockClient(cap.responder)),
      cache: GraphQLCache(),
    );

Widget _harness(_Capture cap, Widget child) => GraphQLProvider(
      client: ValueNotifier(_makeClient(cap)),
      child: MaterialApp(home: child),
    );

class _PushHost extends StatelessWidget {
  final WidgetBuilder builder;
  const _PushHost({required this.builder});

  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(
          child: TextButton(
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: builder),
            ),
            child: const Text('go'),
          ),
        ),
      );
}

/// Answers the three startup queries EditMealPlanScreen always makes:
/// recipeCategoryGroups, items, and mealPlan (in edit mode).
_Capture _editorCap({Map<String, dynamic>? plan, List<dynamic>? items}) =>
    _Capture()
      ..onQuery = (query) {
        if (query.contains('recipeCategoryGroups')) {
          return gqlOk({
            'recipeCategoryGroups': [
              {
                'id': 'g1',
                'name': 'Cuisine',
                'categories': [
                  {'id': 'c1', 'name': 'Italian'}
                ]
              }
            ]
          });
        }
        if (query.contains('mealPlan(')) {
          return gqlOk({'mealPlan': plan});
        }
        if (query.contains('items(')) {
          return gqlOk({
            'items': {
              'items': items ??
                  [
                    {'id': 'i1', 'name': 'Milk'},
                    {'id': 'i2', 'name': 'Bread'},
                  ]
            }
          });
        }
        if (query.contains('recipes(')) {
          return gqlOk({
            'recipes': {
              'items': [
                {'id': 'r7', 'name': 'Sheet Pan Chicken'}
              ]
            }
          });
        }
        return gqlOk(const {});
      };

void main() {
  group('MealPlansScreen', () {
    testWidgets('shows a loading skeleton while the query runs',
        (tester) async {
      final cap = _Capture()
        ..onQuery = (query) => query.contains('mealPlans(')
            ? gqlOk({
                'mealPlans': {
                  'items': [planItem()],
                  'pageInfo': {'totalCount': 1}
                }
              })
            : gqlOk(const {});
      await tester.pumpWidget(_harness(cap, const MealPlansScreen()));
      expect(find.byType(SkeletonList), findsOneWidget);
      await tester.pumpAndSettle();
      expect(find.text('Late October'), findsOneWidget);
    });

    testWidgets('shows an error when the query fails', (tester) async {
      final cap = _Capture()..onRequest = (_) => gqlErr();
      await tester.pumpWidget(_harness(cap, const MealPlansScreen()));
      await tester.pumpAndSettle();
      expect(find.textContaining('Error:'), findsOneWidget);
    });

    testWidgets('renders plans and prettifies auto-named dates',
        (tester) async {
      final cap = _Capture()
        ..onQuery = (_) => gqlOk({
              'mealPlans': {
                'items': [
                  planItem(id: 'p1', name: 'Week of 2026-10-26'),
                  planItem(id: 'p2', name: 'Named Plan', isActive: false),
                ],
                'pageInfo': {'totalCount': 2}
              }
            });
      await tester.pumpWidget(_harness(cap, const MealPlansScreen()));
      await tester.pumpAndSettle();
      // Auto-named: raw ISO date swapped for the localized form.
      expect(find.text('Week of Oct 26, 2026'), findsOneWidget);
      // Named plan keeps its subtitle with the pretty week-start date.
      expect(find.text('Named Plan'), findsOneWidget);
      expect(find.text('Week starting Oct 26, 2026'), findsOneWidget);
      expect(find.text('Active'), findsOneWidget);
    });

    testWidgets('loads the next page via Load more', (tester) async {
      final cap = _Capture();
      cap.onRequest = (query) {
        if (query.contains('mealPlans(') &&
            cap.requests.last['variables']!.contains('"page":2')) {
          return gqlOk({
            'mealPlans': {
              'items': [planItem(id: 'p26', name: 'Page Two Plan')],
              'pageInfo': {'totalCount': 26}
            }
          });
        }
        if (query.contains('mealPlans(')) {
          return gqlOk({
            'mealPlans': {
              'items': List.generate(
                  25, (i) => planItem(id: 'p$i', name: 'Plan $i')),
              'pageInfo': {'totalCount': 26}
            }
          });
        }
        return null;
      };
      await tester.pumpWidget(_harness(cap, const MealPlansScreen()));
      await tester.pumpAndSettle();
      // PagedListView auto-loads near the bottom — drag to the end.
      await tester.drag(find.byType(PagedListView), const Offset(0, -6000));
      await tester.pumpAndSettle();
      final pages = cap.requests
          .where((r) => r['query']!.contains('mealPlans('))
          .map((r) => jsonDecode(r['variables']!)['page'])
          .toSet();
      expect(pages, contains(2));
      expect(find.text('Page Two Plan'), findsOneWidget);
    });

    testWidgets('FAB pushes EditMealPlanScreen in create mode', (tester) async {
      final cap = _Capture()
        ..onRequest = (query) {
          if (query.contains('mealPlans(')) {
            return gqlOk({
              'mealPlans': {
                'items': const [],
                'pageInfo': {'totalCount': 0}
              }
            });
          }
          return null;
        }
        ..onQuery = _editorCap().onQuery;
      await tester.pumpWidget(_harness(cap, const MealPlansScreen()));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();
      expect(find.byType(EditMealPlanScreen), findsOneWidget);
      expect(find.text('Create Meal Plan'), findsOneWidget);
    });

    testWidgets('tapping a plan pushes EditMealPlanScreen in edit mode',
        (tester) async {
      final cap = _Capture()
        ..onRequest = (query) {
          if (query.contains('mealPlans(')) {
            return gqlOk({
              'mealPlans': {
                'items': [planItem()],
                'pageInfo': {'totalCount': 1}
              }
            });
          }
          return null;
        }
        ..onQuery = _editorCap(plan: planDetail()).onQuery;
      await tester.pumpWidget(_harness(cap, const MealPlansScreen()));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Late October'));
      await tester.pumpAndSettle();
      expect(find.byType(EditMealPlanScreen), findsOneWidget);
      expect(find.text('Edit Meal Plan'), findsOneWidget);
    });
  });

  group('EditMealPlanScreen', () {
    Future<void> pumpEditor(WidgetTester tester, _Capture cap,
        {String? mealPlanId}) async {
      await tester.pumpWidget(_harness(
        cap,
        _PushHost(
          builder: (_) => EditMealPlanScreen(mealPlanId: mealPlanId),
        ),
      ));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
    }

    testWidgets('creates a plan with name, date, and week start day',
        (tester) async {
      final cap = _editorCap();
      await pumpEditor(tester, cap);
      expect(find.text('Create Meal Plan'), findsOneWidget);

      await tester.enterText(
          find.widgetWithText(TextField, 'Name'), 'November week 1');
      await tester.enterText(
          find.widgetWithText(TextField, 'Week start date (YYYY-MM-DD)'),
          '2026-11-02');

      await tester.ensureVisible(find.text('Sun'));
      await tester.tap(find.text('Sun'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Mon').last);
      await tester.pumpAndSettle();

      await tester.ensureVisible(find.text('Save'));
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('createMealPlan')!;
      expect(req['variables'], contains('"name":"November week 1"'));
      expect(req['variables'], contains('"weekStartDate":"2026-11-02"'));
      expect(req['variables'], contains('"weekStartDayOfWeek":1'));
      expect(find.byType(EditMealPlanScreen), findsNothing);
    });

    testWidgets('loads an existing plan and updates it', (tester) async {
      final cap = _editorCap(plan: planDetail());
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      expect(find.text('Edit Meal Plan'), findsOneWidget);
      expect(
          tester
              .widget<TextField>(find.widgetWithText(TextField, 'Name'))
              .controller!
              .text,
          'Late October');

      await tester.ensureVisible(find.text('Save'));
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('updateMealPlan')!;
      expect(req['variables'], contains('"id":"p1"'));
      expect(req['variables'], contains('"weekStartDate":"2026-10-26"'));
    });

    testWidgets('renders slots with recipe, servings, note, and items',
        (tester) async {
      final cap = _editorCap(
        plan: planDetail(slots: [
          planSlot(items: [
            {
              'id': 'si1',
              'item': {'id': 'i9', 'name': 'Cheddar'},
              'ingredient': null,
              'quantity': 200,
              'unit': 'g',
              'isFromRecipe': true,
            },
            {
              'id': 'si2',
              'item': null,
              'ingredient': {'id': 'g2', 'name': 'Salt'},
              'quantity': 1,
              'unit': 'tsp',
              'isFromRecipe': false,
            },
          ]),
        ]),
      );
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.text('Mon — dinner'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      expect(find.text('Mon — dinner'), findsOneWidget);
      expect(find.text('Recipe: Roast Chicken'), findsOneWidget);
      expect(find.text('Servings: 4'), findsOneWidget);
      expect(find.text('Note: No dairy'), findsOneWidget);
      expect(find.textContaining('Cheddar 200 g'), findsOneWidget);
      expect(find.textContaining('Salt 1 tsp'), findsOneWidget);
    });

    testWidgets('adds a slot with a picked recipe', (tester) async {
      final cap = _editorCap(plan: planDetail());
      await pumpEditor(tester, cap, mealPlanId: 'p1');

      await tester.dragUntilVisible(
        find.widgetWithText(TextField, 'Meal type'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      await tester.enterText(
          find.widgetWithText(TextField, 'Meal type'), 'brunch');

      // Pick a recipe through the search sheet — InputDecorator renders
      // the label twice, so take .first; tapAt bypasses hit-test misses
      // on the decorated text.
      await tester.ensureVisible(find.text('Recipe (optional)').first);
      await tester
          .tapAt(tester.getCenter(find.text('Recipe (optional)').first));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Sheet Pan Chicken'));
      await tester.pumpAndSettle();

      await tester.ensureVisible(find.widgetWithText(TextField, 'Servings'));
      await tester.enterText(find.widgetWithText(TextField, 'Servings'), '6');
      await tester
          .ensureVisible(find.widgetWithText(TextField, 'Replacement note'));
      await tester.enterText(
          find.widgetWithText(TextField, 'Replacement note'), 'Light lunch');

      // 'Add Slot' is both the section title and the button label.
      final addSlotButton = find.widgetWithText(ElevatedButton, 'Add Slot');
      await tester.ensureVisible(addSlotButton);
      await tester.tap(addSlotButton);
      await tester.pumpAndSettle();

      final req = cap.lastMatching('addMealSlot')!;
      expect(req['variables'], contains('"mealPlanId":"p1"'));
      expect(req['variables'], contains('"mealType":"brunch"'));
      expect(req['variables'], contains('"recipeId":"r7"'));
      expect(req['variables'], contains('"servings":6'));
      expect(req['variables'], contains('"replacementNote":"Light lunch"'));
      expect(cap.countOf('recordSelection'), 1);
      // Adding a slot re-runs _loadData; the mealPlan query itself is
      // cacheFirst, so it may be served without a second request.
      expect(cap.countOf('mealPlan('), greaterThanOrEqualTo(1));
    });

    testWidgets('adds a slot without a recipe', (tester) async {
      final cap = _editorCap(plan: planDetail());
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.widgetWithText(TextField, 'Meal type'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      await tester.enterText(
          find.widgetWithText(TextField, 'Meal type'), 'snack');
      final addSlotButton = find.widgetWithText(ElevatedButton, 'Add Slot');
      await tester.ensureVisible(addSlotButton);
      await tester.tap(addSlotButton);
      await tester.pumpAndSettle();

      final req = cap.lastMatching('addMealSlot')!;
      expect(req['variables'], contains('"mealType":"snack"'));
      expect(req['variables'], contains('"recipeId":null'));
      expect(req['variables'], contains('"servings":null'));
      expect(cap.countOf('recordSelection'), 0);
    });

    testWidgets('removes a slot via its delete button', (tester) async {
      final cap = _editorCap(plan: planDetail(slots: [planSlot()]));
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.text('Mon — dinner'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      // The slot header's delete icon lives in the title Row.
      await tester.tap(find.descendant(
          of: find.widgetWithText(Row, 'Mon — dinner'),
          matching: find.byIcon(Icons.delete)));
      await tester.pumpAndSettle();
      expect(cap.countOf('removeMealSlot'), 1);
      expect(cap.lastMatching('removeMealSlot')!['variables'],
          contains('"slotId":"s1"'));
    });

    testWidgets('adds an item to a slot with quantity and unit',
        (tester) async {
      final cap = _editorCap(plan: planDetail(slots: [planSlot()]));
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.widgetWithText(TextField, 'Qty'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );

      // Select Milk via the slot item dropdown's onChanged — driving the
      // callback exercises the same state path without the overlay tap.
      final itemDropdown = find.byWidgetPredicate((w) =>
          w is DropdownButtonFormField &&
          w.decoration.labelText == 'Item');
      tester
          .widget<DropdownButtonFormField<String?>>(itemDropdown)
          .onChanged
          ?.call('i1');
      await tester.pump();

      await tester.enterText(find.widgetWithText(TextField, 'Qty'), '2');
      await tester.enterText(find.widgetWithText(TextField, 'Unit'), 'cup');
      tester
          .widget<IconButton>(find.widgetWithIcon(IconButton, Icons.add))
          .onPressed
          ?.call();
      await tester.pumpAndSettle();

      final req = cap.lastMatching('addMealSlotItem')!;
      expect(req['variables'], contains('"slotId":"s1"'));
      expect(req['variables'], contains('"itemId":"i1"'));
      expect(req['variables'], contains('"quantity":2'));
      expect(req['variables'], contains('"unit":"cup"'));
      expect(cap.countOf('recordSelection'), 1);
    });

    testWidgets('item add is a no-op without selection or quantity',
        (tester) async {
      final cap = _editorCap(plan: planDetail(slots: [planSlot()]));
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.byIcon(Icons.add),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      // The add IconButton renders at the trailing edge — invoke its
      // onPressed so the early-return guard actually executes.
      tester
          .widget<IconButton>(find.widgetWithIcon(IconButton, Icons.add))
          .onPressed
          ?.call();
      await tester.pumpAndSettle();
      expect(cap.countOf('addMealSlotItem'), 0);
    });

    testWidgets('removes a slot item via its delete button', (tester) async {
      final cap = _editorCap(
        plan: planDetail(slots: [
          planSlot(items: [
            {
              'id': 'si1',
              'item': {'id': 'i9', 'name': 'Cheddar'},
              'ingredient': null,
              'quantity': 200,
              'unit': 'g',
              'isFromRecipe': true,
            },
          ]),
        ]),
      );
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.textContaining('Cheddar 200 g'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      // Item tiles carry their own delete IconButtons — invoke the
      // trailing one directly; the trailing edge sits outside the 800x600
      // viewport's hit region even when the row is visible.
      final deletes = find.widgetWithIcon(IconButton, Icons.delete);
      tester.widget<IconButton>(deletes.last).onPressed?.call();
      await tester.pumpAndSettle();
      expect(cap.countOf('removeMealSlotItem'), 1);
      expect(cap.lastMatching('removeMealSlotItem')!['variables'],
          contains('"slotItemId":"si1"'));
    });

    testWidgets('searches items and records the search', (tester) async {
      final cap = _editorCap(plan: planDetail(slots: [planSlot()]));
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.widgetWithText(TextField, 'Search items'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      await tester.enterText(
          find.widgetWithText(TextField, 'Search items'), 'mil');
      // Debouncer delay is a few hundred ms.
      await tester.pump(const Duration(milliseconds: 600));
      await tester.pumpAndSettle();

      final searches =
          cap.requests.where((r) => r['query']!.contains('items('));
      expect(searches.any((r) => r['variables']!.contains('"search":"mil"')),
          isTrue);
      expect(cap.countOf('recordSearch'), 1);
    });

    testWidgets('filters recipes by category', (tester) async {
      final cap = _editorCap(plan: planDetail());
      await pumpEditor(tester, cap, mealPlanId: 'p1');
      await tester.dragUntilVisible(
        find.text('Filter recipes by category'),
        find.byType(ListView).last,
        const Offset(0, -200),
      );
      await tester.tap(find.text('All categories'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Cuisine: Italian').last);
      await tester.pumpAndSettle();
      // The picker re-queries with categoryIds on next open.
      await tester.ensureVisible(find.text('Recipe (optional)').first);
      await tester
          .tapAt(tester.getCenter(find.text('Recipe (optional)').first));
      await tester.pumpAndSettle();
      expect(find.text('Sheet Pan Chicken'), findsWidgets);
    });
  });
}
