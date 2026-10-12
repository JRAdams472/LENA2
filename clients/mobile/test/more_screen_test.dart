import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/screens/more_screen.dart';
import 'package:lena_mobile/theme_mode.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  // LEN-140: administration is web-only. The More screen is the app's
  // catalog of secondary destinations, so this pins the member surface —
  // no admin management tiles may appear here.
  testWidgets('More screen exposes member destinations and no admin tiles',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final controller = ThemeModeController();
    await controller.load();

    await tester.pumpWidget(
      ChangeNotifierProvider.value(
        value: controller,
        child: const MaterialApp(home: MoreScreen()),
      ),
    );
    await tester.pump();

    for (final title in ['Recipes', 'Meal plans', 'Wine cellar', 'Items']) {
      expect(find.text(title), findsOneWidget);
    }

    for (final adminLabel in [
      'Categories',
      'Pending Reviews',
      'Ingredients',
      'Allergens',
      'Allergen Suggestions',
      'Brands',
      'Food Flavors',
      'Food Nutrients',
      'Nutrient Types',
      'Flavor Profiles',
      'Countries',
      'Regions',
      'Types',
      'Vintages',
      'Grape Varieties',
      'Wine Flavor Profiles',
      'Users',
      'Pending Items',
    ]) {
      expect(find.text(adminLabel), findsNothing);
    }

    // Exactly the four member destination tiles (each renders a chevron).
    expect(find.byIcon(Icons.chevron_right), findsNWidgets(4));
  });
}
