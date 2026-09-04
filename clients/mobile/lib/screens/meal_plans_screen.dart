import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'edit_meal_plan_screen.dart';

const String mealPlansQuery = r'''
  query MealPlans {
    mealPlans(page: 1, pageSize: 25) {
      items {
        id
        name
        weekStartDate
        isActive
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class MealPlansScreen extends StatelessWidget {
  const MealPlansScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Meal Plans')),
      body: Query(
        options: QueryOptions(document: gql(mealPlansQuery)),
        builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final items = result.data?['mealPlans']?['items'] as List? ?? [];

          return ListView.builder(
            padding: const EdgeInsets.all(16.0),
            itemCount: items.length,
            itemBuilder: (context, index) {
              final item = items[index] as Map<String, dynamic>;
              final isActive = item['isActive'] as bool? ?? false;
              return ListTile(
                title: Text(item['name'] as String),
                subtitle: Text('Week starting: ${item['weekStartDate']}'),
                trailing: Chip(label: Text(isActive ? 'Active' : 'Inactive')),
                onTap: () => Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => EditMealPlanScreen(mealPlanId: item['id'] as String),
                  ),
                ),
              );
            },
          );
        },
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => const EditMealPlanScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
