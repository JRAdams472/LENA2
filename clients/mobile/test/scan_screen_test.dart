import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/scan_screen.dart';

void main() {
  group('resolvedIngredientOf', () {
    test('household override wins over the catalog link', () {
      final item = {
        'ingredient': {'id': '7', 'name': 'corn'},
        'householdIngredient': {'id': '9', 'name': 'maize'},
      };
      expect(resolvedIngredientOf(item)?['id'], '9');
      expect(resolvedIngredientOf(item)?['name'], 'maize');
    });

    test('falls back to the catalog ingredient', () {
      final item = {
        'ingredient': {'id': '7', 'name': 'corn'},
        'householdIngredient': null,
      };
      expect(resolvedIngredientOf(item)?['name'], 'corn');
    });

    test('returns null when neither link exists', () {
      final item = {'ingredient': null, 'householdIngredient': null};
      expect(resolvedIngredientOf(item), isNull);
    });
  });

  testWidgets('ScanScreen renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: ScanScreen()),
      ),
    );
    await tester.pump();

    expect(find.text('Scan Item'), findsOneWidget);
  });
}
