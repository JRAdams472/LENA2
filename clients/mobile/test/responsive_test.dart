import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/responsive.dart';

void main() {
  Future<void> pumpAt(
      WidgetTester tester, Size size, Widget Function(BuildContext) build) {
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    return tester.pumpWidget(MaterialApp(home: Builder(builder: build)));
  }

  group('windowSizeClassOf', () {
    testWidgets('compact below 600', (tester) async {
      WindowSizeClass? cls;
      bool? wide, expanded;
      await pumpAt(tester, const Size(599, 800), (context) {
        cls = context.windowSize;
        wide = context.isWide;
        expanded = context.isExpanded;
        return const SizedBox();
      });
      expect(cls, WindowSizeClass.compact);
      expect(wide, isFalse);
      expect(expanded, isFalse);
    });

    testWidgets('medium from 600 to 839', (tester) async {
      WindowSizeClass? cls;
      bool? wide, expanded;
      await pumpAt(tester, const Size(700, 800), (context) {
        cls = context.windowSize;
        wide = context.isWide;
        expanded = context.isExpanded;
        return const SizedBox();
      });
      expect(cls, WindowSizeClass.medium);
      expect(wide, isTrue);
      expect(expanded, isFalse);
    });

    testWidgets('expanded at 840 and above', (tester) async {
      WindowSizeClass? cls;
      bool? expanded;
      await pumpAt(tester, const Size(840, 800), (context) {
        cls = context.windowSize;
        expanded = context.isExpanded;
        return const SizedBox();
      });
      expect(cls, WindowSizeClass.expanded);
      expect(expanded, isTrue);
    });
  });

  group('ConstrainedContent', () {
    BoxConstraints constraintsOf(WidgetTester tester) => tester
        .widget<ConstrainedBox>(find.byType(ConstrainedBox).last)
        .constraints;

    testWidgets('passes through untouched on compact', (tester) async {
      await pumpAt(
          tester,
          const Size(500, 800),
          (_) => const ConstrainedContent(
                child: SizedBox(key: Key('c')),
              ));
      expect(find.byType(Align), findsNothing);
      expect(find.byKey(const Key('c')), findsOneWidget);
    });

    testWidgets('caps child width on expanded', (tester) async {
      await pumpAt(
          tester,
          const Size(1280, 800),
          (_) => const ConstrainedContent(
                child: SizedBox(key: Key('c')),
              ));
      expect(find.byType(Align), findsOneWidget);
      expect(constraintsOf(tester).maxWidth, 840);
    });

    testWidgets('honors a custom maxWidth', (tester) async {
      await pumpAt(
          tester,
          const Size(1280, 800),
          (_) => const ConstrainedContent(
                maxWidth: 400,
                child: SizedBox(key: Key('c')),
              ));
      expect(constraintsOf(tester).maxWidth, 400);
    });
  });
}
