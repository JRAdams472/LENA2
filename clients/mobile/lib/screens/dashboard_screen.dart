import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String dashboardQuery = r'''
  query Dashboard($limit: Int) {
    mealPlans(page: 1, pageSize: 1) {
      items {
        id
        weekStartDate
        slots {
          id
          dayOfWeek
          mealType
          servings
          recipe {
            id
            name
          }
        }
      }
    }
    recommendedRecipes(limit: $limit) {
      recipe {
        id
        name
      }
      reason
      score
    }
    me {
      email
      displayName
    }
  }
''';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(dashboardQuery),
        variables: const {'limit': 10},
      ),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        return Scaffold(
          appBar: AppBar(title: const Text('Dashboard')),
          body: _body(context, result, refetch),
        );
      },
    );
  }

  Widget _body(BuildContext context, QueryResult result, VoidCallback? refetch) {
    if (result.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (result.hasException) {
      return SingleChildScrollView(
        padding: const EdgeInsets.all(16.0),
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 600),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(
                  'Error: ${result.exception.toString()}',
                  textAlign: TextAlign.center,
                  softWrap: true,
                ),
                const SizedBox(height: 16),
                TextButton(
                  onPressed: refetch,
                  child: const Text('Retry'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    final me = result.data?['me'] as Map<String, dynamic>?;
    final name = (me?['displayName'] as String?) ?? (me?['email'] as String?) ?? 'there';
    final plans = result.data?['mealPlans']?['items'] as List? ?? [];
    final recommendations = result.data?['recommendedRecipes'] as List? ?? [];

    final slots = _todaysSlots(plans);

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          Text('Hello, $name', style: Theme.of(context).textTheme.headlineSmall),
          const SizedBox(height: 24),
          Text('Today\'s Meal Plan', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (slots.isEmpty)
            const Card(
              child: ListTile(
                title: Text('No slots for today'),
                subtitle: Text('Create a meal plan to see it here.'),
              ),
            )
          else
            ...slots.map((slot) {
              final recipe = slot['recipe'] as Map<String, dynamic>?;
              return Card(
                child: ListTile(
                  title: Text(recipe?['name'] as String? ?? 'Unknown recipe'),
                  subtitle: Text('Meal: ${slot['mealType']} — ${slot['servings']} servings'),
                ),
              );
            }),
          const SizedBox(height: 24),
          Text('Suggested for You', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (recommendations.isEmpty)
            const Card(
              child: ListTile(
                title: Text('No suggestions yet'),
                subtitle: Text('Rate and plan recipes to get recommendations.'),
              ),
            )
          else
            ...recommendations.take(5).map((rec) {
              final recipe = rec['recipe'] as Map<String, dynamic>?;
              final reason = rec['reason'] as String? ?? '';
              return Card(
                child: ListTile(
                  title: Text(recipe?['name'] as String? ?? 'Unknown'),
                  subtitle: Text('Reason: $reason'),
                ),
              );
            }),
        ],
      ),
    );
  }

  List<Map<String, dynamic>> _todaysSlots(List<dynamic> plans) {
    if (plans.isEmpty) return [];
    final plan = plans.first as Map<String, dynamic>;
    final slots = plan['slots'] as List? ?? [];
    final today = DateTime.now().weekday;
    return slots
        .cast<Map<String, dynamic>>()
        .where((s) => (s['dayOfWeek'] as int?) == today)
        .toList();
  }
}
