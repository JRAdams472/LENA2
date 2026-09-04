import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String recipesQuery = r'''
  query Recipes {
    recipes(page: 1, pageSize: 25) {
      items {
        id
        name
        description
        servings
        prepTimeMinutes
        cookTimeMinutes
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class RecipesScreen extends StatelessWidget {
  const RecipesScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Recipes')),
      body: Query(
        options: QueryOptions(document: gql(recipesQuery)),
        builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final items = result.data?['recipes']?['items'] as List? ?? [];

          return ListView.builder(
            padding: const EdgeInsets.all(16.0),
            itemCount: items.length,
            itemBuilder: (context, index) {
              final item = items[index] as Map<String, dynamic>;
              final description = item['description'] as String?;
              return ListTile(
                title: Text(item['name'] as String),
                subtitle: description != null && description.isNotEmpty
                    ? Text(description)
                    : null,
                trailing: Text('Serves ${item['servings'] ?? '-'}'),
              );
            },
          );
        },
      ),
    );
  }
}
