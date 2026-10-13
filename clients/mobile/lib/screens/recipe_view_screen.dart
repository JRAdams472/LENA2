import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../allergy.dart';
import 'edit_recipe_screen.dart';

/// Read-only recipe peek — name, ingredients, steps. Opened from the
/// recipes list and dashboard so users can glance at a recipe without
/// entering the editor.
const String recipeViewQuery = r'''
  query RecipeView($id: ID!) {
    recipe(id: $id) {
      id
      name
      items(view: effective) {
        id
        section
        displayOrder
        quantity
        unit
        isOptional
        notes
        item {
          id
          name
        }
        ingredient {
          id
          name
        }
      }
      steps(view: effective) {
        id
        stepNumber
        instruction
        durationMinutes
      }
    }
  }
''';

class RecipeViewScreen extends StatelessWidget {
  const RecipeViewScreen({super.key, required this.recipeId});

  final String recipeId;

  /// "2 cup milk" — quantity + unit + ingredient/item name (same
  /// convention as EditRecipeScreen._lineLabel).
  static String lineLabel(Map<String, dynamic> line) {
    final name =
        (line['ingredient']?['name'] ?? line['item']?['name'] ?? '') as String;
    final qty = line['quantity'];
    final unit = line['unit'] as String?;
    final bits = [
      if (qty != null) '$qty',
      if (unit != null && unit.isNotEmpty) unit,
      name,
      if (line['isOptional'] == true) '(optional)',
    ].join(' ');
    return bits.trim().isEmpty ? '(line)' : bits.trim();
  }

  void _openEditor(BuildContext context) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => EditRecipeScreen(recipeId: recipeId),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(recipeViewQuery),
        variables: {'id': recipeId},
      ),
      builder: (QueryResult result,
          {VoidCallback? refetch, FetchMore? fetchMore}) {
        final recipe = result.data?['recipe'] as Map<String, dynamic>?;
        return Scaffold(
          appBar: AppBar(
            title: Text(recipe?['name'] as String? ?? 'Recipe'),
            actions: [
              IconButton(
                icon: const Icon(Icons.edit),
                tooltip: 'Edit',
                onPressed: () => _openEditor(context),
              ),
            ],
          ),
          body: _body(context, result, recipe),
        );
      },
    );
  }

  Widget _body(
      BuildContext context, QueryResult result, Map<String, dynamic>? recipe) {
    if (result.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception}'));
    }
    if (recipe == null) {
      return const Center(child: Text('Recipe not found'));
    }

    final items = recipe['items'] as List? ?? [];
    final steps = List<Map<String, dynamic>>.from(
        (recipe['steps'] as List? ?? [])
            .map((s) => Map<String, dynamic>.from(s as Map)))
      ..sort((a, b) => ((a['stepNumber'] as num?) ?? 0)
          .compareTo((b['stepNumber'] as num?) ?? 0));

    // Group consecutive items sharing a section, preserving display
    // order — most recipes are one flat group.
    final groups = <({String? name, List<Map<String, dynamic>> lines})>[];
    for (final raw in items) {
      final line = Map<String, dynamic>.from(raw as Map);
      final name = line['section'] as String?;
      if (groups.isNotEmpty && groups.last.name == name) {
        groups.last.lines.add(line);
      } else {
        groups.add((name: name, lines: [line]));
      }
    }

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: [
        Text('Ingredients', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 4),
        if (items.isEmpty)
          Text('No ingredients', style: Theme.of(context).textTheme.bodySmall),
        for (final group in groups) ...[
          if (group.name != null)
            Padding(
              padding: const EdgeInsets.only(top: 8, bottom: 2),
              child: Text(
                group.name!,
                style: Theme.of(context).textTheme.labelLarge?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant),
              ),
            ),
          for (final line in group.lines)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(lineLabel(line)),
                  if ((line['notes'] as String?)?.isNotEmpty == true)
                    Text(line['notes'] as String,
                        style: Theme.of(context).textTheme.bodySmall),
                ],
              ),
            ),
        ],
        const SizedBox(height: 16),
        const Divider(),
        const SizedBox(height: 8),
        Text('Steps', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 4),
        if (steps.isEmpty)
          Text('No steps', style: Theme.of(context).textTheme.bodySmall),
        for (final step in steps)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${step['stepNumber']}. ${step['instruction']}'),
                if (step['durationMinutes'] != null)
                  Text('${step['durationMinutes']} min',
                      style: Theme.of(context).textTheme.bodySmall),
              ],
            ),
          ),
        const SizedBox(height: 16),
        const Divider(),
        const DisclaimerCaption(),
      ],
    );
  }
}
