import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/recipe_delta.dart';
import 'package:lena_mobile/screens/recipe_tweaks.dart';
import 'package:lena_mobile/theme.dart';

void main() {
  setUpAll(() async {
    // Real fonts so golden captures render legible text instead of boxes.
    final loader = FontLoader('Nunito');
    for (final f in [
      'Nunito-Regular.ttf',
      'Nunito-Medium.ttf',
      'Nunito-SemiBold.ttf',
      'Nunito-Bold.ttf',
    ]) {
      loader.addFont(Future.value(
          File('assets/fonts/$f').readAsBytesSync().buffer.asByteData()));
    }
    await loader.load();
  });
  group('DeltaItemDraft.fromRow', () {
    test('maps a substitute row with target names', () {
      final d = DeltaItemDraft.fromRow({
        'id': '11',
        'recipeItemId': '10',
        'kind': 'substitute',
        'item': null,
        'ingredient': {'id': '5', 'name': 'shallot'},
        'quantity': 2.0,
        'unit': 'clove',
        'unitId': '33',
        'section': null,
        'displayOrder': null,
        'notes': null,
        'isOptional': null,
        'orphaned': false,
      });
      expect(d.id, '11');
      expect(d.recipeItemId, '10');
      expect(d.kind, 'substitute');
      expect(d.ingredientId, '5');
      expect(d.targetLabel, 'shallot');
      expect(d.quantity, 2.0);
      expect(d.unit, 'clove');
      expect(d.unitId, '33');
    });

    test('keeps orphan flag on rows whose anchor was replaced', () {
      final d = DeltaItemDraft.fromRow({
        'id': '12',
        'recipeItemId': null,
        'kind': 'substitute',
        'item': null,
        'ingredient': {'id': '5', 'name': 'shallot'},
        'quantity': null,
        'unit': null,
        'unitId': null,
        'section': null,
        'displayOrder': null,
        'notes': null,
        'isOptional': null,
        'orphaned': true,
      });
      expect(d.orphaned, isTrue);
      expect(d.recipeItemId, isNull);
    });
  });

  group('describeItemChange', () {
    DeltaItemDraft draft(String kind,
            {String? target, double? qty, String? unit}) =>
        DeltaItemDraft(
          kind: kind,
          ingredientName: target,
          quantity: qty,
          unit: unit,
        );

    test('substitute names base and target', () {
      expect(describeItemChange(draft('substitute', target: 'shallot'), 'garlic'),
          'Swap garlic for shallot');
    });
    test('remove names the base line', () {
      expect(describeItemChange(draft('remove'), 'salt'), 'Remove salt');
    });
    test('adjust shows new quantity and unit', () {
      expect(describeItemChange(draft('adjust', qty: 0.5, unit: 'lb'), 'pasta'),
          'Adjust pasta — 0.5 lb');
    });
    test('add names the target', () {
      expect(describeItemChange(draft('add', target: 'red pepper flakes'), null),
          'Add red pepper flakes');
    });
    test('orphaned anchors fall back to a generic label', () {
      expect(describeItemChange(draft('substitute', target: 'shallot'), null),
          'Swap the original line for shallot');
    });
  });

  group('describeStepChange', () {
    test('replace/remove/add describe the anchor or position', () {
      expect(
          describeStepChange(DeltaStepDraft(kind: 'replace'), 3),
          'Replace step 3');
      expect(describeStepChange(DeltaStepDraft(kind: 'remove'), 2),
          'Remove step 2');
      expect(
          describeStepChange(
              DeltaStepDraft(kind: 'add', stepNumber: 2), null),
          'Add a step at 2');
      expect(describeStepChange(DeltaStepDraft(kind: 'replace'), null),
          'Replace the original step');
    });
  });

  group('serialization', () {
    test('substitute carries target + patch fields', () {
      final input = toDeltaItemInput(DeltaItemDraft(
        recipeItemId: '10',
        kind: 'substitute',
        ingredientId: '5',
        quantity: 1.5,
        unitId: '33',
        notes: 'diced',
        isOptional: false,
      ));
      expect(input, {
        'recipeItemId': '10',
        'kind': 'substitute',
        'itemId': null,
        'ingredientId': '5',
        'quantity': 1.5,
        'unitId': '33',
        'section': null,
        'displayOrder': null,
        'notes': 'diced',
        'isOptional': false,
      });
    });

    test('remove sends only the anchor + kind', () {
      expect(
          toDeltaItemInput(
              DeltaItemDraft(recipeItemId: '12', kind: 'remove')),
          {'recipeItemId': '12', 'kind': 'remove'});
    });

    test('step add carries position + fields', () {
      expect(
          toDeltaStepInput(DeltaStepDraft(
              kind: 'add', stepNumber: 2, instruction: 'Reserve pasta water')),
          {
            'stepId': null,
            'kind': 'add',
            'stepNumber': 2,
            'instruction': 'Reserve pasta water',
            'durationMinutes': null,
            'stepType': null,
            'isPassive': null,
            'dependsOnStepNumber': null,
            'appliance': null,
          });
    });
  });

  group('draft equality (dirty check)', () {
    final row = {
      'id': '11',
      'recipeItemId': '10',
      'kind': 'substitute',
      'item': null,
      'ingredient': {'id': '5', 'name': 'shallot'},
      'quantity': 2.0,
      'unit': 'clove',
      'unitId': '33',
      'section': null,
      'displayOrder': null,
      'notes': null,
      'isOptional': null,
      'orphaned': false,
    };

    test('identical rows round-trip equal', () {
      expect(
          itemDraftsEqual(
              [DeltaItemDraft.fromRow(row)], [DeltaItemDraft.fromRow(row)]),
          isTrue);
    });

    test('a changed field marks the set dirty', () {
      final edited = DeltaItemDraft.fromRow(row)..quantity = 3;
      expect(
          itemDraftsEqual([DeltaItemDraft.fromRow(row)], [edited]), isFalse);
    });

    test('added/removed rows mark the set dirty', () {
      expect(itemDraftsEqual([], [DeltaItemDraft.fromRow(row)]), isFalse);
    });
  });

  group('RecipeTweaksCard', () {
    Widget card({
      List<DeltaItemDraft>? items,
      List<DeltaStepDraft>? steps,
      bool dirty = false,
    }) {
      return MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: RecipeTweaksCard(
              itemDrafts: items ?? const [],
              stepDrafts: steps ?? const [],
              itemBaseLabels: const {'10': 'garlic'},
              stepBaseNumbers: const {'20': 1},
              dirty: dirty,
              saving: false,
              onAddLineTweak: () {},
              onAddStepTweak: () {},
              onRemoveDraft: (_, __) {},
              onSave: () {},
              onDiscard: () {},
              onClearAll: () {},
            ),
          ),
        ),
      );
    }

    testWidgets('renders the empty state', (tester) async {
      await tester.pumpWidget(card());
      expect(find.text('Household tweaks'), findsOneWidget);
      expect(find.textContaining('No tweaks yet'), findsOneWidget);
      expect(find.text('Add ingredient'), findsOneWidget);
      expect(find.text('Add step'), findsOneWidget);
    });

    testWidgets('lists draft rows with orphan chips', (tester) async {
      await tester.pumpWidget(card(
        items: [
          DeltaItemDraft(
              recipeItemId: '10', kind: 'substitute', ingredientName: 'shallot'),
          DeltaItemDraft(
              kind: 'substitute', ingredientName: 'miso', orphaned: true),
        ],
        steps: [DeltaStepDraft(stepId: '20', kind: 'replace')],
        dirty: true,
      ));
      expect(find.text('Swap garlic for shallot'), findsOneWidget);
      expect(find.text('Swap the original line for miso'), findsOneWidget);
      expect(find.text('Replace step 1'), findsOneWidget);
      expect(find.text('No longer applies'), findsOneWidget);
      expect(find.text('Unsaved changes'), findsOneWidget);
      // Save enabled only when dirty.
      expect(
          tester
              .widget<FilledButton>(
                  find.widgetWithText(FilledButton, 'Save tweaks'))
              .enabled,
          isTrue);
    });
  });

  group('tweak sheets', () {
    Widget harness(Widget child) => GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: MaterialApp(home: Scaffold(body: child)),
        );

    testWidgets('line tweak sheet offers Swap/Adjust/Remove', (tester) async {
      await tester.pumpWidget(harness(Builder(
        builder: (context) => TextButton(
          onPressed: () => showItemTweakSheet(
              context: context, anchorId: '10', anchorLabel: 'garlic'),
          child: const Text('open'),
        ),
      )));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Tweak garlic'), findsOneWidget);
      expect(find.text('Swap'), findsOneWidget);
      expect(find.text('Adjust'), findsOneWidget);
      expect(find.text('Remove'), findsOneWidget);
      expect(find.text('Apply tweak'), findsOneWidget);
    });

    testWidgets('step tweak sheet offers Replace/Remove', (tester) async {
      await tester.pumpWidget(harness(Builder(
        builder: (context) => TextButton(
          onPressed: () => showStepTweakSheet(
              context: context, anchorStepId: '20', anchorNumber: 2),
          child: const Text('open'),
        ),
      )));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Tweak step 2'), findsOneWidget);
      expect(find.text('Replace'), findsOneWidget);
      expect(find.text('Directions'), findsOneWidget);
    });
  });

  group('goldens', () {
    testWidgets('tweaks card with a populated change set', (tester) async {
      await tester.pumpWidget(MaterialApp(
        theme: lenaTheme(),
        home: Scaffold(
          body: SingleChildScrollView(
            child: RecipeTweaksCard(
              itemDrafts: [
                DeltaItemDraft(
                    recipeItemId: '10',
                    kind: 'substitute',
                    ingredientName: 'shallot'),
                DeltaItemDraft(
                    recipeItemId: '11', kind: 'adjust', quantity: 0.5, unit: 'lb'),
                DeltaItemDraft(recipeItemId: '12', kind: 'remove'),
                DeltaItemDraft(kind: 'add', ingredientName: 'red pepper flakes'),
              ],
              stepDrafts: [
                DeltaStepDraft(stepId: '20', kind: 'replace'),
                DeltaStepDraft(kind: 'add', stepNumber: 2),
              ],
              itemBaseLabels: const {
                '10': 'garlic',
                '11': 'pasta',
                '12': 'salt',
              },
              stepBaseNumbers: const {'20': 3},
              dirty: true,
              saving: false,
              onAddLineTweak: () {},
              onAddStepTweak: () {},
              onRemoveDraft: (_, __) {},
              onSave: () {},
              onDiscard: () {},
              onClearAll: () {},
            ),
          ),
        ),
      ));
      await tester.pumpAndSettle();
      await expectLater(
        find.byType(RecipeTweaksCard),
        matchesGoldenFile('goldens/recipe_tweaks_card.png'),
      );
    });
  });
}
