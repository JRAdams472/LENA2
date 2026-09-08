import 'package:flutter/material.dart';

class ScanScreen extends StatelessWidget {
  const ScanScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.qr_code_scanner, size: 64),
            SizedBox(height: 16),
            Text('Scan Item'),
            SizedBox(height: 8),
            Text(
              'Barcode scanning will be enabled in mobile-redesign-p4.',
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}
