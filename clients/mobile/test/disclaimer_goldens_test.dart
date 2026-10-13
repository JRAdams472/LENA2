import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/allergy.dart';
import 'package:lena_mobile/screens/disclaimer_screen.dart';
import 'package:lena_mobile/theme.dart';

// LEN-31: golden captures of the new disclaimer surfaces — they double as
// wiki/proof screenshots since they're real renders of the shipped widgets.
void main() {
  group('disclaimer goldens', () {
    testWidgets('consent sheet', (tester) async {
      tester.view.physicalSize = const Size(420, 840);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(
        MaterialApp(
          theme: lenaTheme(),
          home: Builder(
            builder: (ctx) => Scaffold(
              appBar: AppBar(title: const Text('Household')),
              body: Center(
                child: FilledButton(
                  onPressed: () => showDisclaimerConsent(ctx),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.text('I understand and accept'), findsOneWidget);
      await expectLater(
        find.byType(MaterialApp),
        matchesGoldenFile('goldens/disclaimer_consent_sheet.png'),
      );
    });

    testWidgets('full disclaimer screen', (tester) async {
      tester.view.physicalSize = const Size(420, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(
        MaterialApp(theme: lenaTheme(), home: const DisclaimerScreen()),
      );
      await tester.pumpAndSettle();

      await expectLater(
        find.byType(DisclaimerScreen),
        matchesGoldenFile('goldens/disclaimer_screen.png'),
      );
    });
  });
}
