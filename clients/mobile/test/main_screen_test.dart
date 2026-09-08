import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/main_screen.dart';

void main() {
  testWidgets(
    'MainScreen bottom nav has Dashboard, Grocery, Scan, and Pantry',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: MainScreen()),
        ),
      );
      await tester.pump();

      final nav = find.byType(BottomNavigationBar);
      expect(find.descendant(of: nav, matching: find.text('Dashboard')), findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Grocery')), findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Scan')), findsOneWidget);
      expect(find.descendant(of: nav, matching: find.text('Pantry')), findsOneWidget);
    },
  );
}
