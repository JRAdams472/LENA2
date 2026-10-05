import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/format.dart';

void main() {
  group('fmtIso', () {
    test('formats an ISO timestamp', () {
      expect(fmtIso('2026-10-04T18:48:42.199322Z'), contains('Oct'));
      expect(fmtIso('2026-10-04T18:48:42.199322Z'), contains('2026'));
    });

    test('passes through unparseable input', () {
      expect(fmtIso('not a date'), 'not a date');
      expect(fmtIso(''), '');
      expect(fmtIso(null), '');
    });
  });

  group('brandedName', () {
    test('prefixes brand when the name lacks it', () {
      expect(brandedName('Ahold', 'Garlic Salt'), 'Ahold Garlic Salt');
    });

    test('does not double a brand already in the name', () {
      expect(brandedName('Ahold', 'Ahold Garlic Salt'), 'Ahold Garlic Salt');
      expect(brandedName('ahold', 'Ahold Garlic Salt'), 'Ahold Garlic Salt');
    });

    test('handles empty/null parts', () {
      expect(brandedName(null, 'Corn'), 'Corn');
      expect(brandedName('', 'Corn'), 'Corn');
      expect(brandedName('Ahold', null), 'Ahold');
    });
  });

  group('weekdayName', () {
    test('maps 0-6 to weekday names', () {
      expect(weekdayName(0), 'Sun');
      expect(weekdayName(6), 'Sat');
    });

    test('out-of-range days degrade to a label', () {
      expect(weekdayName(7), 'Day 7');
      expect(weekdayName(null), 'Day null');
    });
  });

  group('humanSource', () {
    test('humanizes known sources', () {
      expect(humanSource('manual'), 'Added manually');
      expect(humanSource('mealplan'), 'From meal plan');
      expect(humanSource('pantry'), 'From pantry');
      expect(humanSource('recipe'), 'From recipe');
    });

    test('passes through unknown sources', () {
      expect(humanSource('import'), 'import');
      expect(humanSource(null), '');
    });
  });
}
