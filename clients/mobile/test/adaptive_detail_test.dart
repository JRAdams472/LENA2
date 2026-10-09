import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/widgets/adaptive_detail.dart';

void main() {
  Future<void> pumpAt(WidgetTester tester, Size size, Widget child) async {
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(MaterialApp(home: child));
  }

  group('AdaptiveDetail', () {
    testWidgets('compact renders only the list pane', (tester) async {
      await pumpAt(
        tester,
        const Size(400, 800),
        const AdaptiveDetail(
          list: Text('list-pane'),
          detail: Text('detail-pane'),
        ),
      );
      expect(find.text('list-pane'), findsOneWidget);
      expect(find.text('detail-pane'), findsNothing);
      expect(find.byType(VerticalDivider), findsNothing);
    });

    testWidgets('medium renders only the list pane', (tester) async {
      await pumpAt(
        tester,
        const Size(700, 800),
        const AdaptiveDetail(
          list: Text('list-pane'),
          detail: Text('detail-pane'),
        ),
      );
      expect(find.text('list-pane'), findsOneWidget);
      expect(find.text('detail-pane'), findsNothing);
    });

    testWidgets('expanded renders list + detail side by side', (tester) async {
      await pumpAt(
        tester,
        const Size(1280, 800),
        const AdaptiveDetail(
          list: Text('list-pane'),
          detail: Text('detail-pane'),
        ),
      );
      expect(find.text('list-pane'), findsOneWidget);
      expect(find.text('detail-pane'), findsOneWidget);
      expect(find.byType(VerticalDivider), findsOneWidget);
      // List pane is fixed-width.
      final divider = tester.getTopLeft(find.byType(VerticalDivider));
      expect(divider.dx, 360);
    });

    testWidgets('expanded with null detail shows the placeholder',
        (tester) async {
      await pumpAt(
        tester,
        const Size(1280, 800),
        const AdaptiveDetail(
          list: Text('list-pane'),
          detail: null,
          placeholderIcon: Icons.restaurant_menu,
          placeholderTitle: 'Select a recipe',
        ),
      );
      expect(find.text('list-pane'), findsOneWidget);
      expect(find.text('Select a recipe'), findsOneWidget);
      expect(find.byIcon(Icons.restaurant_menu), findsOneWidget);
    });
  });

  group('openOrSelect', () {
    testWidgets('compact pushes a route', (tester) async {
      var selected = false;
      await pumpAt(
        tester,
        const Size(400, 800),
        Builder(
          builder: (context) => TextButton(
            onPressed: () => openOrSelect(
              context,
              select: () => selected = true,
              builder: (_) => const Text('pushed-route'),
            ),
            child: const Text('open'),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(selected, isFalse);
      expect(find.text('pushed-route'), findsOneWidget);
    });

    testWidgets('expanded selects instead of pushing', (tester) async {
      var selected = false;
      await pumpAt(
        tester,
        const Size(1280, 800),
        Builder(
          builder: (context) => TextButton(
            onPressed: () => openOrSelect(
              context,
              select: () => selected = true,
              builder: (_) => const Text('pushed-route'),
            ),
            child: const Text('open'),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(selected, isTrue);
      expect(find.text('pushed-route'), findsNothing);
    });
  });
}
