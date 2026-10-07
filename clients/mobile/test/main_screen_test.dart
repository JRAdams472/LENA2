import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/main_screen.dart';
import 'package:lena_mobile/theme_mode.dart';
import 'package:provider/provider.dart';

Widget _app({required Widget home}) => GraphQLProvider(
      client: ValueNotifier(graphQLClient),
      child: ChangeNotifierProvider.value(
        value: ThemeModeController(),
        child: MaterialApp(home: home),
      ),
    );

void main() {
  testWidgets(
    'MainScreen bottom nav has Home, Grocery, Events, Scan, Pantry, People, Ask Dot, and More',
    (tester) async {
      await tester.pumpWidget(
        _app(home: const MainScreen()),
      );
      await tester.pump();

      final nav =
          tester.widget<BottomNavigationBar>(find.byType(BottomNavigationBar));
      expect(nav.items.map((i) => i.label).toList(), [
        'Home',
        'Grocery',
        'Events',
        'Scan',
        'Pantry',
        'People',
        'Ask Dot',
        'More',
      ]);
      // Eight fixed destinations can't fit labels — only the selected
      // tab's label is visible.
      expect(nav.showUnselectedLabels, isFalse);
    },
  );

  testWidgets(
    'the More tab links to recipes, meal plans, wine, and items',
    (tester) async {
      await tester.pumpWidget(
        _app(home: const MainScreen()),
      );
      await tester.pump();

      await tester.tap(find.descendant(
        of: find.byType(BottomNavigationBar),
        matching: find.byIcon(Icons.more_horiz),
      ));
      await tester.pump();

      expect(find.text('Recipes'), findsOneWidget);
      expect(find.text('Meal plans'), findsOneWidget);
      expect(find.text('Wine cellar'), findsOneWidget);
      expect(find.text('Items'), findsOneWidget);
    },
  );

  testWidgets(
    'tapping the People tab shows the household screen',
    (tester) async {
      await tester.pumpWidget(
        _app(home: const MainScreen()),
      );
      await tester.pump();

      await tester.tap(find.descendant(
        of: find.byType(BottomNavigationBar),
        matching: find.byIcon(Icons.group),
      ));
      await tester.pump();

      expect(find.text('Household'), findsWidgets);
    },
  );

  testWidgets(
    'unvisited tabs are not built — Scan never mounts at launch',
    (tester) async {
      await tester.pumpWidget(
        _app(home: const MainScreen()),
      );
      await tester.pump();

      // ScanScreen builds lazily on first visit so its CAMERA
      // permission isn't requested at launch.
      expect(find.byType(NavigationBar), findsNothing);
      expect(find.text('Center a barcode in the camera view'), findsNothing);

      await tester.tap(find.descendant(
        of: find.byType(BottomNavigationBar),
        matching: find.byIcon(Icons.qr_code_scanner),
      ));
      await tester.pump();

      // After visiting, the tab is mounted.
      expect(
        find.descendant(
          of: find.byType(IndexedStack),
          matching: find.byType(Scaffold),
        ),
        findsWidgets,
      );
    },
  );
}
