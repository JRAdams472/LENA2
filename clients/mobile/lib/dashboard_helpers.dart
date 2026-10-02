import 'package:flutter/material.dart';

import 'theme.dart';

// Friendly labels for RecommendationReason values — mirrors
// clients/web/app/page.tsx REASON_LABELS so mobile never shows raw
// backend strings like "category_affinity".
const _reasonLabels = {
  'ingredient_overlap': 'Similar to your menu',
  'rating_recency': 'Due for a revisit',
  'collaborative_filtering': 'Recommended for you',
  'category_affinity': "Matches your household's tastes",
  'household_trending': 'Trending in your household',
};

String reasonLabel(String reason) =>
    _reasonLabels[reason] ?? 'Recommended for you';

// Deterministic accent per recipe so suggestion tiles differentiate
// without real photos.
Color accentFor(int recipeId) =>
    lenaAccents[recipeId.abs() % lenaAccents.length];

// Category/dish-type keywords drive the suggestion tile icon, same
// rules as the web dashboard.
IconData suggestionIcon(List<dynamic>? categories) {
  final text = (categories ?? [])
      .map(
        (c) => '${(c as Map?)?['group']?['name'] ?? ''} ${c?['name'] ?? ''}',
      )
      .join(' ')
      .toLowerCase();
  if (RegExp(r'breakfast|brunch').hasMatch(text)) return Icons.free_breakfast;
  if (RegExp(r'café|cafe|coffee|beverage|drink').hasMatch(text)) {
    return Icons.local_cafe;
  }
  if (RegExp(r'dessert|cake|sweet|baking|pastry|cookie').hasMatch(text)) {
    return Icons.cake;
  }
  if (RegExp(r'seafood|fish').hasMatch(text)) return Icons.set_meal;
  if (RegExp(r'dinner|supper|main course').hasMatch(text)) {
    return Icons.dinner_dining;
  }
  if (RegExp(r'lunch|salad|sandwich').hasMatch(text)) return Icons.lunch_dining;
  return Icons.restaurant;
}

// "20 min · ★ 4.5" — quick stats line under a recipe title.
String suggestionMeta(Map<String, dynamic>? recipe) {
  if (recipe == null) return '';
  final parts = <String>[];
  final mins = (recipe['prepTimeMinutes'] as int? ?? 0) +
      (recipe['cookTimeMinutes'] as int? ?? 0);
  if (mins > 0) parts.add('$mins min');
  final rating = recipe['averageRating'] as num?;
  if (rating != null) parts.add('★ ${rating.toStringAsFixed(1)}');
  return parts.join(' · ');
}

// Meal slots carry a string mealType (breakfast/lunch/dinner/snack).
IconData mealIcon(String? mealType) {
  switch (mealType?.toLowerCase()) {
    case 'breakfast':
      return Icons.free_breakfast;
    case 'lunch':
      return Icons.lunch_dining;
    case 'dinner':
      return Icons.dinner_dining;
    case 'snack':
      return Icons.cookie;
    default:
      return Icons.restaurant;
  }
}

String mealLabel(String? mealType) {
  final t = mealType?.trim() ?? '';
  if (t.isEmpty) return 'Meal';
  return t[0].toUpperCase() + t.substring(1);
}

// dayOfWeek on the wire is JS-style 0=Sun..6=Sat; Dart weekday is
// 1=Mon..7=Sun — convert before matching.
int jsWeekday(DateTime d) => d.weekday % 7;

List<Map<String, dynamic>> todaysSlots(List<dynamic> plans, int jsDay) {
  if (plans.isEmpty) return [];
  final plan = plans.first;
  if (plan is! Map) return [];
  final slots = plan['slots'] as List? ?? [];
  return slots
      .whereType<Map>()
      .map((s) => s.cast<String, dynamic>())
      .where((s) => s['dayOfWeek'] == jsDay)
      .toList();
}
