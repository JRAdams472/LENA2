import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/dashboard_helpers.dart';
import 'package:lena_mobile/theme.dart';

Map<String, dynamic> _category(String group, String name) => {
      'name': name,
      'group': {'name': group},
    };

void main() {
  group('reasonLabel', () {
    test('maps backend reasons to friendly microcopy', () {
      expect(reasonLabel('ingredient_overlap'), 'Similar to your menu');
      expect(reasonLabel('rating_recency'), 'Due for a revisit');
      expect(reasonLabel('collaborative_filtering'), 'Recommended for you');
      expect(
        reasonLabel('category_affinity'),
        "Matches your household's tastes",
      );
      expect(reasonLabel('household_trending'), 'Trending in your household');
    });

    test('unknown reasons fall back to a friendly default', () {
      expect(reasonLabel('something_new'), 'Recommended for you');
      expect(reasonLabel(''), 'Recommended for you');
    });
  });

  group('accentFor', () {
    test('is deterministic and always in the accent palette', () {
      for (final id in [0, 1, 5, -3, 9999]) {
        expect(lenaAccents, contains(accentFor(id)));
        expect(accentFor(id), accentFor(id));
      }
    });
  });

  group('suggestionIcon', () {
    test('maps category keywords to meal icons', () {
      expect(
        suggestionIcon([_category('Course', 'Breakfast')]),
        Icons.free_breakfast,
      );
      expect(suggestionIcon([_category('Dish Type', 'Cake')]), Icons.cake);
      expect(
        suggestionIcon([_category('Main Ingredient', 'Fish')]),
        Icons.set_meal,
      );
      expect(
          suggestionIcon([_category('Course', 'Dinner')]), Icons.dinner_dining);
      expect(
        suggestionIcon([_category('Course', 'Lunch')]),
        Icons.lunch_dining,
      );
      expect(
          suggestionIcon([_category('Cuisine', 'Italian')]), Icons.restaurant);
      expect(suggestionIcon(null), Icons.restaurant);
      expect(suggestionIcon([]), Icons.restaurant);
    });
  });

  group('suggestionMeta', () {
    test('combines total time and rating', () {
      expect(
        suggestionMeta({
          'prepTimeMinutes': 10,
          'cookTimeMinutes': 15,
          'averageRating': 4.25,
        }),
        '25 min · ★ 4.3',
      );
    });

    test('omits missing parts and handles nulls', () {
      expect(suggestionMeta({'prepTimeMinutes': 20}), '20 min');
      expect(suggestionMeta({'averageRating': 5.0}), '★ 5.0');
      expect(suggestionMeta({}), '');
      expect(suggestionMeta(null), '');
    });
  });

  group('meal icons and labels', () {
    test('maps mealType strings', () {
      expect(mealIcon('breakfast'), Icons.free_breakfast);
      expect(mealIcon('lunch'), Icons.lunch_dining);
      expect(mealIcon('dinner'), Icons.dinner_dining);
      expect(mealIcon('snack'), Icons.cookie);
      expect(mealIcon('other'), Icons.restaurant);
      expect(mealIcon(null), Icons.restaurant);
    });

    test('capitalizes labels', () {
      expect(mealLabel('dinner'), 'Dinner');
      expect(mealLabel(''), 'Meal');
      expect(mealLabel(null), 'Meal');
    });
  });

  group('todaysSlots', () {
    final plans = [
      {
        'slots': [
          {'dayOfWeek': 0, 'mealType': 'dinner'},
          {'dayOfWeek': 4, 'mealType': 'lunch'},
          {'dayOfWeek': 4, 'mealType': 'dinner'},
        ],
      },
    ];

    test('matches the JS 0=Sun convention, not Dart weekday', () {
      // Sunday: wire dayOfWeek is 0, Dart weekday is 7.
      expect(jsWeekday(DateTime(2026, 10, 4)), 0); // a Sunday
      expect(jsWeekday(DateTime(2026, 10, 1)), 4); // a Thursday
      expect(todaysSlots(plans, 0).length, 1);
      expect(todaysSlots(plans, 0).first['mealType'], 'dinner');
    });

    test('returns all matching slots for a day', () {
      expect(todaysSlots(plans, 4).length, 2);
    });

    test('handles empty plans and missing slots', () {
      expect(todaysSlots([], 0), isEmpty);
      expect(todaysSlots([{}], 0), isEmpty);
    });
  });
}
