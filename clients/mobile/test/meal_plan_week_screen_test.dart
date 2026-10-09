import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lena_mobile/screens/meal_plan_week_screen.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

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

Map<String, dynamic> planDetail({
  List<Map<String, dynamic>>? slots,
}) =>
    {
      'id': 'p1',
      'name': 'Late October',
      'weekStartDate': '2026-10-26',
      'isActive': true,
      'slots': slots ?? [],
    };

Map<String, dynamic> planSlot({
  String id = 's1',
  int dayOfWeek = 1,
  String mealType = 'Dinner',
  Map<String, dynamic>? recipe = const {'id': 'r1', 'name': 'Roast Chicken'},
  int? servings = 4,
  List<Map<String, dynamic>>? items,
}) =>
    {
      'id': id,
      'dayOfWeek': dayOfWeek,
      'mealType': mealType,
      'servings': servings,
      'replacementNote': null,
      'recipe': recipe,
      'items': items ?? [],
      'allergyWarnings': [],
      'allergens': [],
    };

class _Capture {
  final List<Map<String, String>> requests = [];

  http.Response? Function(String query)? onRequest;

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
    if (query.contains('addMealSlot')) {
      return gqlOk({
        'addMealSlot': {'id': 's9'}
      });
    }
    if (query.contains('removeMealSlot')) {
      return gqlOk({'removeMealSlot': true});
    }
    if (query.contains('mealPlan')) {
      return gqlOk({
        'mealPlan': planDetail(slots: [planSlot()])
      });
    }
    return gqlOk(const {});
  }

  int countOf(String fragment) =>
      requests.where((r) => r['query']!.contains(fragment)).length;

  Map<String, String>? lastMatching(String fragment) {
    final hits = requests.where((r) => r['query']!.contains(fragment));
    return hits.isEmpty ? null : hits.last;
  }
}

Widget _harness(_Capture cap) => GraphQLProvider(
      client: ValueNotifier(GraphQLClient(
        link: HttpLink('https://example.test/graphql',
            httpClient: MockClient(cap.responder)),
        cache: GraphQLCache(),
      )),
      child: const MaterialApp(home: MealPlanWeekScreen(mealPlanId: 'p1')),
    );

void main() {
  group('MealPlanWeekScreen', () {
    testWidgets('compact renders scrolling day sections with slot labels',
        (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture();
      await tester.pumpWidget(_harness(cap));
      expect(find.byType(SkeletonList), findsOneWidget);
      await tester.pumpAndSettle();
      expect(find.text('Sun'), findsWidgets);
      expect(find.text('Roast Chicken'), findsOneWidget);
      expect(find.text('Add Dinner'), findsWidgets);
    });

    testWidgets('legacy free-text mealType groups under Other', (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture()
        ..onRequest = (q) => q.contains('mealPlan')
            ? gqlOk({
                'mealPlan': planDetail(
                    slots: [planSlot(mealType: 'brunch', recipe: null)])
              })
            : null;
      await tester.pumpWidget(_harness(cap));
      await tester.pumpAndSettle();
      expect(find.text('Other'), findsWidgets);
      expect(find.text('Breakfast'), findsWidgets);
    });

    testWidgets('wide pane renders the 7-column grid', (tester) async {
      tester.view.physicalSize = const Size(1200, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture();
      await tester.pumpWidget(_harness(cap));
      await tester.pumpAndSettle();
      expect(find.text('Breakfast'), findsOneWidget);
      expect(find.text('Sat'), findsOneWidget);
    });

    testWidgets('tapping a slot opens detail and Remove mutates',
        (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture();
      await tester.pumpWidget(_harness(cap));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Roast Chicken'));
      await tester.pumpAndSettle();
      expect(find.text('Servings: 4'), findsOneWidget);
      await tester.tap(find.text('Remove'));
      await tester.pumpAndSettle();
      expect(cap.countOf('removeMealSlot'), 1);
      expect(cap.lastMatching('removeMealSlot')!['variables'],
          contains('"slotId":"s1"'));
    });

    testWidgets('suggest meals lists suggestions and Add creates a slot',
        (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture()
        ..onRequest = (q) => q.contains('suggestMeals')
            ? gqlOk({
                'suggestMeals': [
                  {
                    'recipe': {'id': 'r7', 'name': 'Sourdough Omelette'},
                    'dayOfWeek': 2,
                    'mealType': 'Breakfast',
                    'reason': 'Uses expiring eggs',
                    'usesExpiringItems': ['eggs'],
                  }
                ]
              })
            : null;
      await tester.pumpWidget(_harness(cap));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('Suggest meals'));
      await tester.pumpAndSettle();
      expect(find.text('Sourdough Omelette — Tue Breakfast'), findsOneWidget);
      expect(find.text('Uses expiring eggs'), findsOneWidget);
      expect(find.textContaining('Uses expiring: eggs'), findsOneWidget);
      await tester.tap(find.text('Add'));
      await tester.pumpAndSettle();
      expect(cap.countOf('addMealSlot'), 1);
      final vars = cap.lastMatching('addMealSlot')!['variables']!;
      expect(vars, contains('"dayOfWeek":2'));
      expect(vars, contains('"mealType":"Breakfast"'));
      expect(vars, contains('"recipeId":"r7"'));
    });

    testWidgets('nutrition sheet renders entries and warnings', (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture()
        ..onRequest = (q) => q.contains('nutrition')
            ? gqlOk({
                'nutrition': {
                  'entries': [
                    {'name': 'Protein', 'unit': 'g', 'amount': 45.2},
                  ],
                  'warnings': ['no linked item for "mystery"'],
                }
              })
            : null;
      await tester.pumpWidget(_harness(cap));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('Nutrition'));
      await tester.pumpAndSettle();
      expect(find.text('Protein'), findsOneWidget);
      expect(find.text('45.2 g'), findsOneWidget);
      expect(find.text('no linked item for "mystery"'), findsOneWidget);
    });

    testWidgets('add-meal dialog sends addMealSlot with day and meal type',
        (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      final cap = _Capture();
      await tester.pumpWidget(_harness(cap));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Add Breakfast').first);
      await tester.pumpAndSettle();
      expect(find.text('Add meal'), findsOneWidget);
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();
      expect(cap.countOf('addMealSlot'), 1);
      final vars = cap.lastMatching('addMealSlot')!['variables']!;
      expect(vars, contains('"mealType":"Breakfast"'));
    });
  });
}
