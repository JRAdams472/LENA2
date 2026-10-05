import 'dart:io';

import 'package:integration_test/integration_test_driver_extended.dart';

/// Screenshot driver for the LEN-48 UI audit. Each `takeScreenshot` call
/// in the test writes a PNG under `MOBILE_SHOTS_OUT` (default `mobile-shots/`),
/// resolved against the directory `flutter drive` was launched from.
Future<void> main() => integrationDriver(
      onScreenshot: (String name, List<int> bytes,
          [Map<String, Object?>? args]) async {
        try {
          final dir = Directory(
            Platform.environment['MOBILE_SHOTS_OUT'] ?? 'mobile-shots',
          );
          await dir.create(recursive: true);
          final f = File('${dir.path}/$name.png');
          await f.writeAsBytes(bytes);
          stderr.writeln('screenshot -> ${f.absolute.path} (${bytes.length}B)');
          return true;
        } catch (e, st) {
          stderr.writeln('screenshot FAILED: $name: $e\n$st');
          return false;
        }
      },
    );
