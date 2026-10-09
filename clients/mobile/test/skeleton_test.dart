import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

void main() {
  testWidgets('SkeletonBox sizes and rounds the placeholder', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SkeletonBox(width: 50, height: 20, radius: 4),
        ),
      ),
    );

    final box = tester.widget<Container>(
      find.descendant(
        of: find.byType(SkeletonBox),
        matching: find.byType(Container),
      ),
    );
    final decoration = box.decoration as BoxDecoration;
    expect(box.constraints?.maxWidth, 50);
    expect(decoration.borderRadius, BorderRadius.circular(4));
    expect(decoration.color, isNotNull);
  });

  testWidgets('SkeletonCard renders the row placeholder shape', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: SkeletonCard())),
    );

    expect(find.byType(Card), findsOneWidget);
    // Leading block + two text-line boxes.
    expect(find.byType(SkeletonBox), findsNWidgets(3));
  });

  testWidgets('SkeletonList renders [count] cards with custom padding', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SkeletonList(count: 3, padding: EdgeInsets.all(4)),
        ),
      ),
    );

    expect(find.byType(SkeletonCard), findsNWidgets(3));
    final list = tester.widget<ListView>(find.byType(ListView));
    expect(list.padding, const EdgeInsets.all(4));
    expect(list.physics, isA<NeverScrollableScrollPhysics>());
  });

  testWidgets('SkeletonList defaults to six cards', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: SkeletonList())),
    );
    expect(find.byType(SkeletonCard), findsNWidgets(6));
  });

  testWidgets('SkeletonForm renders label + field pairs per field', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: SkeletonForm(fields: 2))),
    );

    // 2 label bars + 2 field bars.
    expect(find.byType(SkeletonBox), findsNWidgets(4));
    expect(find.byType(ListView), findsOneWidget);
  });

  testWidgets('LenaSplash renders the branded boot splash', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: LenaSplash()));

    expect(find.text('L'), findsOneWidget);
    expect(find.text('Lena'), findsOneWidget);
    expect(find.byType(Scaffold), findsOneWidget);
  });
}
