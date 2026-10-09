import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lena_mobile/screens/cook_mode_screen.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

Map<String, dynamic> _recipe({List<Map<String, dynamic>>? steps}) => {
      '__typename': 'Recipe',
      'id': 'r1',
      'name': 'Roast Chicken',
      'servings': 4,
      'items': [
        {
          '__typename': 'RecipeItem',
          'quantity': 1.5,
          'unit': 'kg',
          'displayOrder': 1,
          'notes': null,
          'isOptional': false,
          'item': {'__typename': 'Item', 'name': 'Whole chicken'},
          'ingredient': null,
        },
      ],
      'steps': steps ??
          [
            {
              '__typename': 'RecipeStep',
              'stepNumber': 1,
              'instruction': 'Preheat the oven to 220°C.',
              'durationMinutes': 10,
              'stepType': 'prep',
              'isPassive': true,
            },
            {
              '__typename': 'RecipeStep',
              'stepNumber': 2,
              'instruction': 'Season the chicken.',
              'durationMinutes': null,
              'stepType': 'cook',
              'isPassive': false,
            },
          ],
    };

Widget _harness({Map<String, dynamic>? recipe}) => GraphQLProvider(
      client: ValueNotifier(GraphQLClient(
        link: HttpLink(
          'https://example.test/graphql',
          httpClient: MockClient((req) async {
            return http.Response(
              jsonEncode({
                'data': {
                  '__typename': 'Query',
                  'recipe': recipe ?? _recipe(),
                }
              }),
              200,
              headers: {'content-type': 'application/json'},
            );
          }),
        ),
        cache: GraphQLCache(),
      )),
      child: const MaterialApp(home: CookModeScreen(recipeId: 'r1')),
    );

void main() {
  group('CookModeScreen', () {
    testWidgets('renders step counter, instruction, badges, duration chip',
        (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_harness());
      expect(find.byType(SkeletonList), findsOneWidget);
      await tester.pumpAndSettle();
      expect(find.text('1 / 2'), findsOneWidget);
      expect(find.text('Preheat the oven to 220°C.'), findsOneWidget);
      expect(find.text('prep'), findsOneWidget);
      expect(find.text('Hands-off'), findsOneWidget);
      expect(find.text('10 min'), findsOneWidget);
    });

    testWidgets('chevron advances the pager', (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.chevron_right));
      await tester.pumpAndSettle();
      expect(find.text('2 / 2'), findsOneWidget);
      expect(find.text('Season the chicken.'), findsOneWidget);
    });

    testWidgets('duration chip starts a countdown', (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();
      await tester.tap(find.text('10 min'));
      await tester.pump();
      expect(find.text('10:00'), findsOneWidget);
      await tester.pump(const Duration(seconds: 1));
      expect(find.text('9:59'), findsOneWidget);
      // Drain the timer so it can't fire after teardown.
      await tester.pump(const Duration(minutes: 10));
    });

    testWidgets('compact shows ingredients via a bottom-sheet FAB',
        (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ingredients'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Whole chicken'), findsOneWidget);
    });

    testWidgets('wide pane shows the ingredients rail', (tester) async {
      tester.view.physicalSize = const Size(1100, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();
      expect(find.textContaining('Whole chicken'), findsOneWidget);
      expect(find.byType(FloatingActionButton), findsNothing);
    });
  });
}
