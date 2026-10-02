import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/screens/dashboard_content.dart';
import 'package:lena_mobile/theme.dart';

Map<String, dynamic> _fixture() => {
      'me': {'id': '1', 'email': 'sam@example.com', 'displayName': 'Sam'},
      'mealPlans': <String, dynamic>{
        'items': [
          {
            'id': '7',
            'weekStartDate': '2026-09-28',
            'slots': [
              {
                'id': '1',
                'dayOfWeek': DateTime.now().weekday % 7,
                'mealType': 'dinner',
                'servings': 4,
                'recipe': {
                  'id': '11',
                  'name': 'Herb Roast Chicken',
                  'prepTimeMinutes': 20,
                  'cookTimeMinutes': 75,
                },
              },
            ],
          },
        ],
      },
      'recommendedRecipes': [
        {
          'recipe': {
            'id': '21',
            'name': 'Garlic Butter Pasta',
            'description': 'Quick weeknight pasta.',
            'prepTimeMinutes': 5,
            'cookTimeMinutes': 15,
            'averageRating': 4.5,
            'categories': [
              {
                'name': 'Dinner',
                'group': {'name': 'Course'}
              },
            ],
          },
          'reason': 'category_affinity',
          'score': 0.9,
        },
        {
          'recipe': {
            'id': '22',
            'name': 'Hearty Vegetable Soup',
            'prepTimeMinutes': 15,
            'cookTimeMinutes': 40,
            'averageRating': null,
            'categories': [],
          },
          'reason': 'household_trending',
          'score': 0.7,
        },
      ],
      'householdInvites': [],
      'unreadNotificationCount': 0,
    };

Widget _wrap(Widget child) =>
    MaterialApp(theme: lenaTheme(), home: Scaffold(body: child));

void main() {
  testWidgets('dashboard renders greeting, meals, and friendly suggestions', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(DashboardContent(data: _fixture())));
    await tester.pump();

    expect(find.text('Hello, Sam'), findsOneWidget);
    expect(find.text('Today\'s meals'), findsOneWidget);
    expect(find.text('Herb Roast Chicken'), findsOneWidget);
    expect(find.text('Delicious ideas for tonight'), findsOneWidget);
    expect(find.text('Garlic Butter Pasta'), findsOneWidget);
    expect(find.text('Hearty Vegetable Soup'), findsOneWidget);

    // Friendly reason labels — never the raw backend string.
    expect(find.text("Matches your household's tastes"), findsWidgets);
    expect(find.text('Trending in your household'), findsOneWidget);
    expect(find.textContaining('Reason:'), findsNothing);
    expect(find.textContaining('category_affinity'), findsNothing);

    // Real metadata: total cook time + rating.
    expect(find.textContaining('95 min'), findsOneWidget);
    expect(find.textContaining('20 min'), findsOneWidget);
  });

  testWidgets('empty meals show an actionable Plan it link', (tester) async {
    final data = _fixture();
    (data['mealPlans'] as Map<String, dynamic>)['items'] = <dynamic>[];
    await tester.pumpWidget(_wrap(DashboardContent(data: data)));
    await tester.pump();

    expect(find.text('Nothing planned for today yet'), findsOneWidget);
    expect(find.text('Plan it →'), findsOneWidget);
  });

  testWidgets('empty suggestions show friendly copy', (tester) async {
    final data = _fixture();
    data['recommendedRecipes'] = [];
    await tester.pumpWidget(_wrap(DashboardContent(data: data)));
    await tester.pump();

    expect(find.textContaining('rate a few recipes'), findsOneWidget);
  });
}
