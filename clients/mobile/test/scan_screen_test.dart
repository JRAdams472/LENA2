import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/screens/scan_screen.dart';

void main() {
  testWidgets('Scan screen shows placeholder for p4', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: ScanScreen()));
    await tester.pump();

    expect(find.text('Scan Item'), findsOneWidget);
    expect(
      find.text('Barcode scanning will be enabled in mobile-redesign-p4.'),
      findsOneWidget,
    );
  });
}
