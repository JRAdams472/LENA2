import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String mealPlansQuery = r'''
  query MealPlans {
    mealPlans(page: 1, pageSize: 25) {
      items {
        id
        name
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

const String generateGroceryListMutation = r'''
  mutation GenerateGroceryList($mealPlanId: ID!) {
    generateGroceryList(mealPlanId: $mealPlanId) {
      id
    }
  }
''';

class GenerateGroceryDialog extends StatefulWidget {
  final VoidCallback? onGenerated;

  const GenerateGroceryDialog({super.key, this.onGenerated});

  @override
  State<GenerateGroceryDialog> createState() => _GenerateGroceryDialogState();
}

class _GenerateGroceryDialogState extends State<GenerateGroceryDialog> {
  String? _mealPlanId;
  bool _isSaving = false;

  Future<List<Map<String, dynamic>>> _loadMealPlans() async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(QueryOptions(document: gql(mealPlansQuery)));
    return (result.data?['mealPlans']?['items'] as List? ?? [])
        .cast<Map<String, dynamic>>();
  }

  Future<void> _generate() async {
    if (_mealPlanId == null || _mealPlanId!.isEmpty) return;
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      await client.mutate(MutationOptions(
        document: gql(generateGroceryListMutation),
        variables: {'mealPlanId': _mealPlanId},
      ));
      if (mounted) {
        Navigator.pop(context);
        widget.onGenerated?.call();
      }
    } finally {
      if (mounted) setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<Map<String, dynamic>>>(
      future: _loadMealPlans(),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const AlertDialog(
            content: SizedBox(
              height: 60,
              child: Center(child: CircularProgressIndicator()),
            ),
          );
        }

        final plans = snapshot.data ?? [];

        return AlertDialog(
          title: const Text('Generate Grocery List'),
          content: DropdownButtonFormField<String?>(
            value: _mealPlanId,
            decoration: const InputDecoration(labelText: 'Meal plan'),
            items: plans
                .map((p) => DropdownMenuItem(
                      value: p['id'] as String,
                      child: Text(p['name'] as String),
                    ))
                .toList(),
            onChanged: (v) => setState(() => _mealPlanId = v),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              onPressed: _isSaving ? null : _generate,
              child: _isSaving
                  ? const SizedBox(
                      height: 16,
                      width: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Generate'),
            ),
          ],
        );
      },
    );
  }
}
