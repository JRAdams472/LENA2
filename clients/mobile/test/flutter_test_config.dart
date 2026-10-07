import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';

/// Tolerates sub-1% pixel diffs in golden tests: Skia rasterizes
/// anti-aliased edges differently across host OSes (dev machines on
/// Windows, CI on Linux), which trips LocalFileComparator's exact match.
/// Anything beyond the tolerance still fails and writes failure feedback.
class _TolerantGoldenComparator extends LocalFileComparator {
  _TolerantGoldenComparator(super.testFile);

  static const double _kTolerance = 0.01; // fraction of pixels

  @override
  Future<bool> compare(Uint8List imageBytes, Uri golden) async {
    final result = await GoldenFileComparator.compareLists(
      imageBytes,
      await getGoldenBytes(golden),
    );
    if (!result.passed && result.diffPercent > _kTolerance) {
      final error = await generateFailureOutput(result, golden, basedir);
      throw FlutterError(error);
    }
    return true;
  }
}

Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  if (goldenFileComparator is LocalFileComparator) {
    final basedir = (goldenFileComparator as LocalFileComparator).basedir;
    // _getBasedir dirnames the constructor arg — resolve a dummy file inside
    // basedir so the wrapped comparator lands on the same directory.
    goldenFileComparator =
        _TolerantGoldenComparator(basedir.resolve('_dummy_test.dart'));
  }
  await testMain();
}
