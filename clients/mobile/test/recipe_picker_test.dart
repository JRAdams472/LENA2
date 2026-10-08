import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/widgets/recipe_picker.dart';
import 'package:lena_mobile/widgets/search_picker.dart';

Widget _wrap(Widget child) => GraphQLProvider(
      client: ValueNotifier(graphQLClient),
      child: MaterialApp(home: Scaffold(body: child)),
    );

void main() {
  testWidgets('shows the selected recipe name and the none label otherwise',
      (tester) async {
    await tester.pumpWidget(_wrap(RecipePickerField(
      selected: const {'id': '7', 'name': 'Pancakes'},
      onChanged: (_) {},
    )));
    expect(find.text('Pancakes'), findsOneWidget);

    await tester.pumpWidget(_wrap(RecipePickerField(
      selected: null,
      onChanged: (_) {},
      noneLabel: 'Free-form dish',
    )));
    expect(find.text('Free-form dish'), findsOneWidget);
  });

  testWidgets('tapping the field opens the search sheet', (tester) async {
    await tester.pumpWidget(_wrap(RecipePickerField(
      selected: null,
      onChanged: (_) {},
    )));
    await tester.tap(find.byType(RecipePickerField));
    await tester.pump();

    expect(find.byType(SearchPickerSheet), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Search recipes'), findsOneWidget);
    // Let the initial (unreachable-server) query settle so the ListView —
    // including the "None" row — is built.
    await tester.pump(const Duration(seconds: 2));
    await tester.pump();
    expect(
        find.descendant(
          of: find.byType(SearchPickerSheet),
          matching: find.widgetWithText(ListTile, 'None'),
        ),
        findsOneWidget);
  });

  testWidgets('None row reports null and pops', (tester) async {
    var picked = false;
    Object? value = 'untouched';
    await tester.pumpWidget(_wrap(RecipePickerField(
      selected: null,
      onChanged: (v) {
        picked = true;
        value = v;
      },
    )));
    await tester.tap(find.byType(RecipePickerField));
    await tester.pump();
    // Let the initial (unreachable-server) query settle into an error/empty
    // list before interacting — the None row renders regardless.
    await tester.pump(const Duration(seconds: 2));
    await tester.pump();

    await tester.tap(find.descendant(
      of: find.byType(SearchPickerSheet),
      matching: find.widgetWithText(ListTile, 'None'),
    ));
    await tester.pump();
    expect(picked, isTrue);
    expect(value, isNull);
  });
}
