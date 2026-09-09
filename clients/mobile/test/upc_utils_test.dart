import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/scan/upc_utils.dart';

void main() {
  group('normalizeUpc', () {
    test('returns 12-digit codes unchanged', () {
      expect(normalizeUpc('123456789012'), '123456789012');
      expect(normalizeUpc('123456-789012'), '123456789012');
    });

    test('left-pads 13-digit EAN to GTIN-14', () {
      expect(normalizeUpc('1234567890123'), '01234567890123');
    });

    test('returns 14-digit codes unchanged', () {
      expect(normalizeUpc('01234567890123'), '01234567890123');
    });

    test('rejects invalid lengths', () {
      expect(normalizeUpc('123'), isNull);
      expect(normalizeUpc('abcdefghij'), isNull);
      expect(normalizeUpc(''), isNull);
    });
  });
}
