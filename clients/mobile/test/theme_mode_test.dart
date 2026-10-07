import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/screens/more_screen.dart';
import 'package:lena_mobile/theme.dart';
import 'package:lena_mobile/theme_mode.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  group('lenaDarkTheme', () {
    test('uses the design-spec warm charcoal palette', () {
      final theme = lenaDarkTheme();
      expect(theme.brightness, Brightness.dark);
      expect(theme.scaffoldBackgroundColor, lenaDarkCanvas);
      expect(theme.colorScheme.surface, lenaDarkPaper);
      expect(theme.colorScheme.primary, lenaDarkSage);
      expect(theme.colorScheme.onSurface, lenaDarkInk);
      // Hairlines, not shadows: dark cards sit flat with a border.
      expect(theme.cardTheme.elevation, 0);
      expect(
        (theme.cardTheme.shape as RoundedRectangleBorder).side.color,
        lenaDarkDivider,
      );
    });

    test('accent lists differ per brightness but stay four-strong', () {
      expect(lenaAccentsFor(Brightness.dark), lenaDarkAccents);
      expect(lenaAccentsFor(Brightness.light), lenaAccents);
      expect(lenaDarkAccents, hasLength(lenaAccents.length));
    });
  });

  group('ThemeModeController', () {
    test('defaults to system when nothing is persisted', () async {
      SharedPreferences.setMockInitialValues({});
      final controller = ThemeModeController();
      await controller.load();
      expect(controller.mode, ThemeMode.system);
    });

    test('load restores a persisted choice', () async {
      SharedPreferences.setMockInitialValues({'theme_mode': 'dark'});
      final controller = ThemeModeController();
      await controller.load();
      expect(controller.mode, ThemeMode.dark);
    });

    test('setMode notifies and persists', () async {
      SharedPreferences.setMockInitialValues({});
      final controller = ThemeModeController();
      var notified = 0;
      controller.addListener(() => notified++);
      await controller.setMode(ThemeMode.dark);
      expect(controller.mode, ThemeMode.dark);
      expect(notified, 1);
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('theme_mode'), 'dark');
      // No-op re-selection doesn't churn listeners.
      await controller.setMode(ThemeMode.dark);
      expect(notified, 1);
    });
  });

  testWidgets('More screen Appearance selector switches and persists',
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

    expect(find.text('Appearance'), findsOneWidget);

    await tester.tap(find.text('Dark'));
    await tester.pump();
    expect(controller.mode, ThemeMode.dark);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('theme_mode'), 'dark');

    await tester.tap(find.text('Light'));
    await tester.pump();
    expect(controller.mode, ThemeMode.light);
    expect(prefs.getString('theme_mode'), 'light');
  });
}
