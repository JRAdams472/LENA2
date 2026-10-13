import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/allergy.dart';
import 'package:lena_mobile/screens/disclaimer_screen.dart';
import 'package:lena_mobile/theme.dart';

// LEN-31: the disclaimer surfaces — consent sheet and full screen.
// (Pixel goldens for these text-heavy pages rasterize too differently
// across host OSes, so these assert content, not pixels.)
void main() {
  group('disclaimer surfaces', () {
    testWidgets('consent sheet shows the copy and both actions',
        (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: lenaTheme(),
          home: Builder(
            builder: (ctx) => Scaffold(
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

      expect(find.text('Medical & allergen disclaimer'), findsOneWidget);
      expect(find.textContaining('not medical'), findsOneWidget);
      expect(find.text('Read the full disclaimer'), findsOneWidget);
      expect(find.text('I understand and accept'), findsOneWidget);
      expect(find.text('Not now'), findsOneWidget);
    });

    testWidgets('full disclaimer screen renders every section',
        (tester) async {
      await tester.pumpWidget(
        MaterialApp(theme: lenaTheme(), home: const DisclaimerScreen()),
      );
      await tester.pumpAndSettle();

      expect(find.text('Medical & Allergen Disclaimer'), findsOneWidget);
      expect(find.text('Information, not advice'), findsOneWidget);

      // The body is a lazy ListView — scroll each deep section into view.
      for (final heading in [
        'Data may be incomplete or wrong',
        'Always check the label',
        'Liability',
        'Your records',
      ]) {
        await tester.scrollUntilVisible(find.text(heading), 200);
        expect(find.text(heading), findsOneWidget);
      }
    });
  });
}
