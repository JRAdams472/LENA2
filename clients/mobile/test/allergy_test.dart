import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/allergy.dart';

Map<String, dynamic> warning({
  String memberKind = 'allergy',
  String entityKind = 'contains',
  String? displayName = 'Ada',
  String? firstName,
  String? lastName,
  String allergenName = 'Peanuts',
}) =>
    {
      'memberKind': memberKind,
      'entityKind': entityKind,
      'member': {
        'id': '8',
        'displayName': displayName,
        'firstName': firstName,
        'lastName': lastName,
      },
      'allergen': {'id': '1', 'name': allergenName},
    };

void main() {
  group('allergyWarningsOf / allergenFlagsOf', () {
    test('reads lists off the entity map', () {
      final entity = {
        'allergyWarnings': [warning()],
        'allergens': [
          {
            'kind': 'contains',
            'allergen': {'id': '1', 'name': 'Peanuts'}
          },
        ],
      };
      expect(allergyWarningsOf(entity), hasLength(1));
      expect(allergenFlagsOf(entity), hasLength(1));
    });

    test('missing or null fields yield empty lists', () {
      expect(allergyWarningsOf({}), isEmpty);
      expect(allergyWarningsOf(null), isEmpty);
      expect(allergenFlagsOf({'allergens': null}), isEmpty);
    });
  });

  group('allergyWarningText', () {
    test('names member, allergen, and both kinds', () {
      expect(
          allergyWarningText(warning()), 'Ada — Peanuts (allergy; contains)');
    });

    test('falls back to first/last name then a generic label', () {
      expect(
        allergyWarningText(
            warning(displayName: null, firstName: 'Bob', lastName: 'Jones')),
        'Bob Jones — Peanuts (allergy; contains)',
      );
      expect(
        allergyWarningText(warning(displayName: null)),
        'Household member — Peanuts (allergy; contains)',
      );
    });

    test('renders advisory kinds', () {
      expect(
        allergyWarningText(
            warning(memberKind: 'dietary', entityKind: 'may_contain')),
        'Ada — Peanuts (dietary; may contain)',
      );
    });
  });

  group('allergyWarningSevere', () {
    test('true only for allergy + contains', () {
      expect(allergyWarningSevere([warning()]), isTrue);
      expect(
          allergyWarningSevere([warning(entityKind: 'may_contain')]), isFalse);
      expect(allergyWarningSevere([warning(memberKind: 'dietary')]), isFalse);
      expect(allergyWarningSevere([]), isFalse);
    });
  });

  group('AllergyWarningBadge', () {
    testWidgets('renders nothing when there are no warnings', (tester) async {
      await tester.pumpWidget(const MaterialApp(
        home: Scaffold(body: AllergyWarningBadge(warnings: [])),
      ));
      expect(find.byIcon(Icons.warning_amber_rounded), findsNothing);
    });

    testWidgets('shows an icon and a detail dialog on tap', (tester) async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: AllergyWarningBadge(warnings: [warning()])),
      ));

      expect(find.byIcon(Icons.warning_amber_rounded), findsOneWidget);

      await tester.tap(find.byIcon(Icons.warning_amber_rounded));
      await tester.pumpAndSettle();

      expect(find.text('Allergy warnings'), findsOneWidget);
      expect(find.text('Ada — Peanuts (allergy; contains)'), findsOneWidget);
    });
  });
}
