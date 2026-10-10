import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:lena_mobile/main.dart' as app;

/// Tablet screenshot walk (LEN-14 P5): a focused pass over the adaptive
/// surfaces — rail shell, two-pane list/detail, meal-plan week grid,
/// suggest/nutrition sheets, cook mode. Run on a tablet-profile AVD:
///
///   flutter drive --driver=test_driver/integration_test.dart \
///     --target=integration_test/tablet_screenshot_test.dart \
///     -d emulator-5554 \
///     --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
///     --dart-define=LENA_DEBUG_ID_TOKEN=<testissuer token>
void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> settle(WidgetTester tester,
      [Duration d = const Duration(seconds: 2)]) async {
    final end = DateTime.now().add(d);
    while (DateTime.now().isBefore(end)) {
      await tester.pump(const Duration(milliseconds: 100));
    }
  }

  Future<void> shot(WidgetTester tester, String name,
      [Duration wait = const Duration(seconds: 4)]) async {
    await settle(tester, wait);
    await binding.takeScreenshot(name);
    debugPrint('shot: $name');
  }

  bool exists(Finder f) => f.evaluate().isNotEmpty;

  /// Tap a destination icon in whichever nav chrome is present — bottom
  /// bar on compact, rail on medium/expanded.
  Future<void> nav(WidgetTester tester, IconData icon, String label) async {
    var target = find.descendant(
        of: find.byType(BottomNavigationBar), matching: find.byIcon(icon));
    if (target.evaluate().isEmpty) {
      target = find.descendant(
          of: find.byType(NavigationRail), matching: find.byIcon(icon));
    }
    if (exists(target)) {
      await tester.tap(target.first);
      for (var i = 0; i < 15; i++) {
        await tester.pump(const Duration(milliseconds: 100));
      }
    } else {
      debugPrint('nav target missing: $label');
    }
  }

  Future<bool> tapIfExists(WidgetTester tester, Finder f, String label) async {
    if (exists(f)) {
      await tester.tap(f.first);
      return true;
    }
    debugPrint('tap target missing: $label');
    return false;
  }

  testWidgets('tablet capture', (WidgetTester tester) async {
    app.main();
    await tester.pump();
    await binding.convertFlutterSurfaceToImage();
    await shot(tester, 't-00-boot', const Duration(seconds: 8));
    // Sign-in + first dashboard render.
    await shot(tester, 't-01-dashboard-rail', const Duration(seconds: 10));

    // Grocery: two-pane list + detail.
    await nav(tester, Icons.shopping_cart, 'Grocery');
    await shot(tester, 't-02-grocery-lists', const Duration(seconds: 6));
    if (await tapIfExists(
        tester, find.byType(ListTile), 'first grocery list')) {
      await shot(tester, 't-03-grocery-two-pane', const Duration(seconds: 6));
    }

    // Meal plans: week view + suggest + nutrition (mock AI on lena2shots).
    await nav(tester, Icons.more_horiz, 'More');
    await shot(tester, 't-04-more', const Duration(seconds: 3));
    await tapIfExists(tester, find.text('Meal plans'), 'meal plans tile');
    await shot(tester, 't-05-meal-plans', const Duration(seconds: 4));
    if (await tapIfExists(tester, find.byType(ListTile), 'first meal plan')) {
      await shot(tester, 't-06-meal-plan-week', const Duration(seconds: 6));
      if (await tapIfExists(
          tester, find.byTooltip('Suggest meals'), 'suggest meals')) {
        await shot(tester, 't-07-suggest-meals', const Duration(seconds: 4));
        // Dismiss via the barrier — a system back can pop the detail
        // pane's nested navigator and clear the week view.
        await tester.tapAt(const Offset(800, 300));
        await settle(tester);
      }
      if (await tapIfExists(tester, find.byTooltip('Nutrition'), 'nutrition')) {
        await shot(tester, 't-08-nutrition', const Duration(seconds: 4));
        await tester.tapAt(const Offset(800, 300));
        await settle(tester);
      }
    }

    // Recipes: pushed screens cover the rail, so return to the More tab
    // with the appbar back button before switching tiles.
    await tapIfExists(tester, find.byIcon(Icons.arrow_back), 'meal plans back');
    await settle(tester);
    await tapIfExists(tester, find.text('Recipes'), 'recipes tile');
    await shot(tester, 't-09-recipes', const Duration(seconds: 6));
    if (await tapIfExists(tester, find.byType(ListTile), 'first recipe')) {
      await shot(tester, 't-10-recipe-two-pane', const Duration(seconds: 6));
      if (await tapIfExists(tester, find.byTooltip('Cook mode'), 'cook mode')) {
        await shot(tester, 't-11-cook-mode', const Duration(seconds: 4));
        await tapIfExists(
            tester, find.byIcon(Icons.chevron_right), 'next step');
        await shot(tester, 't-12-cook-mode-step2', const Duration(seconds: 2));
        // Pop cook mode via its own back button.
        await tapIfExists(
            tester, find.byIcon(Icons.arrow_back), 'cook mode back');
        await settle(tester);
      }
    }

    // Pop recipes back to the More tab so the rail is reachable again.
    await tapIfExists(tester, find.byIcon(Icons.arrow_back), 'recipes back');
    await settle(tester);

    // Assistant tab on the rail for completeness.
    await nav(tester, Icons.auto_awesome, 'Ask Dot');
    await shot(tester, 't-13-assistant', const Duration(seconds: 6));
  });
}
