import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:lena_mobile/main.dart' as app;

/// LEN-48 audit: captures every reachable screen on a seeded e2e stack.
///
/// Run against the lena2shots compose project (see docs/wiki-screenshots.md):
///   flutter drive \
///     --driver=test_driver/integration_test.dart \
///     --target=integration_test/screenshot_test.dart \
///     -d emulator-5554 \
///     --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
///     --dart-define=LENA_DEBUG_ID_TOKEN=<test-issuer token>
///
/// The debug token signs the app in at startup; every shot lands as
/// mobile-shots/<name>.png on the host.
void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> settle([Duration wait = const Duration(seconds: 4)]) async {
    await Future<void>.delayed(wait);
  }

  Future<void> shot(WidgetTester tester, String name,
      [Duration wait = const Duration(seconds: 4)]) async {
    await settle(wait);
    await tester.pump();
    await binding.takeScreenshot(name);
    debugPrint('shot: $name');
  }

  Future<void> nav(WidgetTester tester, String label) async {
    final tab = find.descendant(
      of: find.byType(BottomNavigationBar),
      matching: find.text(label),
    );
    if (tab.evaluate().isNotEmpty) {
      await tester.tap(tab);
    } else {
      debugPrint('nav target missing: $label');
    }
  }

  bool exists(Finder f) => f.evaluate().isNotEmpty;

  /// System-back pop â€” works regardless of whether the AppBar renders a
  /// BackButton, which `tester.pageBack` requires.
  Future<void> back(WidgetTester tester) async {
    await tester.binding.handlePopRoute();
    await tester.pump();
    await Future<void>.delayed(const Duration(milliseconds: 500));
    await tester.pump();
  }

  /// Pops a modal route (dialog/bottom sheet) — system-back works on
  /// any route, unlike a barrier tap that can land on sheet content.
  Future<void> dismissModal(WidgetTester tester) => back(tester);

  Future<void> tapIfExists(WidgetTester tester, Finder f, String label) async {
    if (exists(f)) {
      await tester.tap(f);
    } else {
      debugPrint('tap target missing: $label');
    }
  }

  /// `.first` on an empty finder throws before `exists()` can check;
  /// evaluate the plain finder first, then tap the first match.
  Future<void> tapFirst(WidgetTester tester, Finder f, String label) async {
    if (exists(f)) {
      await tester.tap(f.first);
    } else {
      debugPrint('tap target missing: $label');
    }
  }

  testWidgets('capture all screens', (WidgetTester tester) async {
    app.main();
    await binding.convertFlutterSurfaceToImage();
    await shot(tester, '00-boot', const Duration(seconds: 1));
    await shot(tester, '01-dashboard', const Duration(seconds: 10));

    // â”€â”€ Bottom-nav tabs â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
    await nav(tester, 'Grocery');
    await shot(tester, '02-grocery-lists');

    await tapIfExists(
        tester, find.byType(FloatingActionButton), 'generate dialog FAB');
    await shot(tester, '03-generate-grocery-dialog',
        const Duration(seconds: 2));
    await dismissModal(tester);

    await tapIfExists(
        tester, find.text('List 1'), 'first grocery list tile');
    await shot(tester, '04-grocery-list', const Duration(seconds: 6));
    await back(tester);

    await nav(tester, 'Events');
    await shot(tester, '05-events');

    await tapFirst(
        tester, find.byType(ListTile), 'first event card');
    await shot(tester, '06-event-detail', const Duration(seconds: 6));

    await tapIfExists(tester, find.text('View cooking timeline'),
        'timeline button');
    await shot(tester, '07-event-timeline', const Duration(seconds: 6));
    await back(tester);

    await tapIfExists(tester, find.byTooltip('Edit event'), 'edit event');
    await shot(tester, '08-edit-event');
    await back(tester);
    await back(tester);

    await nav(tester, 'Pantry');
    await shot(tester, '09-pantry');

    await nav(tester, 'Household');
    await shot(tester, '10-household');

    await tapIfExists(tester, find.byTooltip('Notification settings'),
        'notification settings gear');
    await shot(tester, '11-notification-settings');
    await back(tester);

    await nav(tester, 'Ask Dot');
    await shot(tester, '12-assistant');

    // â”€â”€ More tab destinations â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
    await nav(tester, 'More');
    await shot(tester, '13-more');

    await tapIfExists(tester, find.text('Recipes'), 'recipes tile');
    await shot(tester, '14-recipes', const Duration(seconds: 6));

    await tapIfExists(tester, find.byTooltip('Filter by category'),
        'recipe filter');
    await shot(tester, '15-recipe-filter-sheet',
        const Duration(seconds: 2));
    await dismissModal(tester);

    await tapFirst(
        tester, find.byType(ListTile), 'first recipe card');
    await shot(tester, '16-edit-recipe', const Duration(seconds: 6));
    await back(tester);

    await tapIfExists(
        tester, find.byType(FloatingActionButton), 'new recipe FAB');
    await shot(tester, '17-new-recipe');
    await back(tester);

    await back(tester); // back to More

    await nav(tester, 'More');
    await tapIfExists(tester, find.text('Meal plans'), 'meal plans tile');
    await shot(tester, '18-meal-plans');

    await tapFirst(
        tester, find.byType(ListTile), 'first meal plan card');
    await shot(tester, '19-edit-meal-plan', const Duration(seconds: 6));
    await back(tester);
    await back(tester);

    await nav(tester, 'More');
    await tapIfExists(tester, find.text('Wine cellar'), 'wine tile');
    await shot(tester, '20-wine', const Duration(seconds: 6));

    await tapIfExists(tester, find.byWidgetPredicate(
        (w) => w is FloatingActionButton && w.heroTag == 'adjust'),
        'adjust bottle FAB');
    await shot(tester, '21-adjust-bottle');
    await back(tester);

    await tapIfExists(tester, find.byWidgetPredicate(
        (w) => w is FloatingActionButton && w.heroTag == 'bottles'),
        'bottles FAB');
    await shot(tester, '22-bottles', const Duration(seconds: 6));

    await tapFirst(
        tester, find.byType(ListTile), 'first bottle card');
    await shot(tester, '23-edit-bottle', const Duration(seconds: 6));
    await back(tester);
    await back(tester);
    await back(tester);

    await nav(tester, 'More');
    await tapIfExists(tester, find.text('Items'), 'items tile');
    await shot(tester, '24-items', const Duration(seconds: 6));

    await tapFirst(
        tester, find.byType(ListTile), 'first item card');
    await shot(tester, '25-edit-item', const Duration(seconds: 6));
    await back(tester);
    await back(tester);

    // â”€â”€ Scan last: camera behavior on emulator is itself a finding â”€â”€â”€
    await nav(tester, 'Scan');
    await shot(tester, '26-scan', const Duration(seconds: 6));

    debugPrint('done');
  }, timeout: const Timeout(Duration(minutes: 15)));
}
