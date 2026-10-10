import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:lena_mobile/main.dart' as app;

/// Compact cook-mode capture (LEN-117): phone-width shots of cook mode's
/// ingredients bottom sheet for the wiki's Mobile-Cook-Mode page. Run on a
/// phone-profile AVD against the lena2shots stack:
///
///   flutter drive --driver=test_driver/integration_test.dart \
///     --target=integration_test/compact_cook_mode_test.dart \
///     -d emulator-5556 \
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

  Future<bool> tapIfExists(WidgetTester tester, Finder f, String label) async {
    if (f.evaluate().isNotEmpty) {
      await tester.tap(f.first);
      return true;
    }
    debugPrint('tap target missing: $label');
    return false;
  }

  testWidgets('compact cook mode capture', (WidgetTester tester) async {
    app.main();
    await tester.pump();
    await binding.convertFlutterSurfaceToImage();
    await settle(tester, const Duration(seconds: 18));

    // More tab -> Recipes -> first recipe -> Cook mode.
    await tapIfExists(
        tester,
        find.descendant(
            of: find.byType(BottomNavigationBar),
            matching: find.byIcon(Icons.more_horiz)),
        'more tab');
    await settle(tester, const Duration(seconds: 3));
    await tapIfExists(tester, find.text('Recipes'), 'recipes tile');
    await settle(tester, const Duration(seconds: 6));
    if (!await tapIfExists(tester, find.byType(ListTile), 'first recipe')) {
      return;
    }
    await settle(tester, const Duration(seconds: 6));
    if (!await tapIfExists(tester, find.byTooltip('Cook mode'), 'cook mode')) {
      return;
    }
    await shot(tester, 'c-11-cook-mode', const Duration(seconds: 4));

    // Ingredients bottom sheet (compact variant of the wide ingredients rail).
    for (final f in [
      find.widgetWithText(FloatingActionButton, 'Ingredients'),
      find.text('Ingredients'),
      find.byIcon(Icons.list),
    ]) {
      if (await tapIfExists(tester, f, 'ingredients button')) break;
    }
    await shot(
        tester, 'c-12-cook-mode-ingredients', const Duration(seconds: 3));
  });
}
