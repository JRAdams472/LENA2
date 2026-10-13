import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lena_mobile/screens/recipe_view_screen.dart';

Map<String, dynamic> _recipe() => {
      '__typename': 'Recipe',
      'id': 'r1',
      'name': 'Roast Chicken',
      'items': [
        {
          '__typename': 'RecipeItem',
          'id': 'i1',
          'section': null,
          'displayOrder': 1,
          'quantity': 1.5,
          'unit': 'kg',
          'isOptional': false,
          'notes': null,
          'item': {'__typename': 'Item', 'id': 'it1', 'name': 'Whole chicken'},
          'ingredient': null,
        },
      ],
      'steps': [
        {
          '__typename': 'RecipeStep',
          'id': 's1',
          'stepNumber': 1,
          'instruction': 'Preheat the oven.',
          'durationMinutes': 10,
        },
      ],
    };

Widget _harness() => GraphQLProvider(
      client: ValueNotifier(GraphQLClient(
        link: HttpLink(
          'https://example.test/graphql',
          httpClient: MockClient((req) async {
            return http.Response(
              jsonEncode({
                'data': {
                  '__typename': 'Query',
                  'recipe': _recipe(),
                },
              }),
              200,
              headers: {'content-type': 'application/json'},
            );
          }),
        ),
        cache: GraphQLCache(),
      )),
      child: const MaterialApp(home: RecipeViewScreen(recipeId: 'r1')),
    );

void main() {
  group('RecipeViewScreen', () {
    testWidgets('renders ingredients, steps, and the disclaimer caption',
        (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();

      expect(find.text('Roast Chicken'), findsOneWidget);
      expect(find.textContaining('Whole chicken'), findsOneWidget);
      expect(find.text('1. Preheat the oven.'), findsOneWidget);
      expect(
        find.textContaining("check the product's label"),
        findsOneWidget,
      );
    });
  });
}
