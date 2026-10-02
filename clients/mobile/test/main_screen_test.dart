import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/main_screen.dart';

void main() {
  testWidgets(
    'MainScreen bottom nav has Dashboard, Grocery, Events, Scan, Pantry, Household, Ask Dot, and More',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: MainScreen()),
        ),
      );
      await tester.pump();

      final nav = find.byType(BottomNavigationBar);
      expect(find.descendant(of: nav, matching: find.text('Dashboard')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Grocery')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Events')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Scan')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Pantry')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Household')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Ask Dot')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('More')),
          findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Assistant')),
          findsNothing);
    },
  );

  testWidgets(
    'the More tab links to recipes, meal plans, wine, and items',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: MainScreen()),
        ),
      );
      await tester.pump();

      await tester.tap(find.text('More'));
      await tester.pump();

      expect(find.text('Recipes'), findsOneWidget);
      expect(find.text('Meal plans'), findsOneWidget);
      expect(find.text('Wine cellar'), findsOneWidget);
      expect(find.text('Items'), findsOneWidget);
    },
  );

  testWidgets(
    'tapping the Household tab shows the household screen',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: MainScreen()),
        ),
      );
      await tester.pump();

      await tester.tap(find.text('Household'));
      await tester.pump();

      expect(find.text('Household'), findsWidgets);
    },
  );
}
