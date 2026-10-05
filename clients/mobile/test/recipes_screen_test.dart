import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/recipes_screen.dart';
import 'package:lena_mobile/screens/edit_recipe_screen.dart';
import 'package:lena_mobile/screens/edit_meal_plan_screen.dart';

void main() {
  testWidgets('RecipesScreen renders search field and filter actions',
      (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: RecipesScreen()),
      ),
    );
    await tester.pump();

    expect(find.text('Recipes'), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
    expect(find.byTooltip('Favorites only'), findsOneWidget);
    expect(find.byTooltip('Filter by category'), findsOneWidget);
  });

  testWidgets('RecipesScreen search field accepts input', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: RecipesScreen()),
      ),
    );
    await tester.pump();

    await tester.enterText(find.byType(TextField), 'pasta');
    await tester.pump();
    expect(find.text('pasta'), findsOneWidget);
  });

  testWidgets('RecipesScreen filter button opens the category sheet',
      (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: RecipesScreen()),
      ),
    );
    await tester.pump();

    await tester.tap(find.byTooltip('Filter by category'));
    await tester.pump();

    expect(find.text('Filter by category'), findsWidgets);
    expect(find.text('Done'), findsOneWidget);
  });

  testWidgets('EditRecipeScreen create form renders without category section',
      (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: EditRecipeScreen()),
      ),
    );
    await tester.pump();

    expect(find.text('Create Recipe'), findsOneWidget);
    expect(find.text('Categories'), findsNothing);
  });

  testWidgets('EditRecipeScreen edit mode renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: EditRecipeScreen(recipeId: '1')),
      ),
    );
    await tester.pump();

    expect(find.text('Edit Recipe'), findsOneWidget);
  });

  testWidgets('EditMealPlanScreen renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: EditMealPlanScreen(mealPlanId: '1')),
      ),
    );
    await tester.pump();

    expect(find.byType(Scaffold), findsOneWidget);
  });
}
