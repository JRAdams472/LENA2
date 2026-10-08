import 'package:flutter/material.dart';
import 'search_picker.dart';

const String recipePickerQuery = r'''
  query RecipePicker($search: String, $mealType: String, $categoryIds: [ID!]) {
    recipes(page: 1, pageSize: 50, search: $search, mealType: $mealType, categoryIds: $categoryIds) {
      items {
        id
        name
      }
    }
  }
''';

/// Form-field-style picker backed by a debounced, server-side recipe
/// search (see [SearchPickerField]) — a dropdown of the first N recipes
/// cannot reach recipes beyond the first page. [selected] is the chosen
/// recipe's `{id, name}` map (or null); [onChanged] receives the same.
class RecipePickerField extends StatelessWidget {
  const RecipePickerField({
    super.key,
    required this.selected,
    required this.onChanged,
    this.mealType,
    this.categoryId,
    this.label = 'Recipe',
    this.noneLabel = 'None',
  });

  final Map<String, dynamic>? selected;
  final ValueChanged<Map<String, dynamic>?> onChanged;
  final String? mealType;
  final String? categoryId;
  final String label;
  final String noneLabel;

  @override
  Widget build(BuildContext context) {
    final mt = mealType?.trim();
    return SearchPickerField(
      label: label,
      displayText: selected?['name'] as String? ?? '',
      noneLabel: noneLabel,
      sheetTitle: 'Search recipes',
      document: recipePickerQuery,
      connectionField: 'recipes',
      searchEntityType: 'recipe',
      variablesFor: (term) => {
        'search': term.isEmpty ? null : term,
        'mealType': (mt?.isNotEmpty ?? false) ? mt : null,
        'categoryIds': categoryId == null ? null : [categoryId],
      },
      itemLabel: (r) => r['name'] as String? ?? '',
      onChanged: onChanged,
    );
  }
}
