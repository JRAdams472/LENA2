import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../format.dart';
import '../widgets/skeleton.dart';
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
        builder: (QueryResult result,
            {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const SkeletonList();
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
              // Auto-named plans embed the raw ISO date — swap it for the
              // localized date and drop the subtitle so the row carries
              // the date once.
              final name = item['name'] as String;
              final rawDate = item['weekStartDate'] as String? ?? '';
              final pretty = fmtIso(rawDate);
              final hasDateInName = pretty.isNotEmpty && name.contains(rawDate);
              final title =
                  hasDateInName ? name.replaceAll(rawDate, pretty) : name;
              return Card(
                child: ListTile(
                  dense: true,
                  title: Text(title),
                  subtitle: hasDateInName || pretty.isEmpty
                      ? null
                      : Text('Week starting $pretty'),
                  trailing: isActive ? const Chip(label: Text('Active')) : null,
                  onTap: () => Navigator.push(
                    context,
                    MaterialPageRoute(
                      builder: (_) =>
                          EditMealPlanScreen(mealPlanId: item['id'] as String),
                    ),
                  ),
                ),
              );
            },
          );
        },
      ),
      floatingActionButton: FloatingActionButton(
        heroTag: 'fab-meal-plans',
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => const EditMealPlanScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
