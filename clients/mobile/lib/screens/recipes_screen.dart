import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'edit_recipe_screen.dart';

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
        isFavorite
      }
      pageInfo {
        totalCount
      }
    }
  }
'';

const String setRecipeFavorite = r'''
  mutation SetRecipeFavorite($recipeId: ID!, $isFavorite: Boolean!) {
    setRecipeFavorite(recipeId: $recipeId, isFavorite: $isFavorite)
  }
'';

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
              final isFavorite = item['isFavorite'] as bool? ?? false;
              return ListTile(
                title: Text(item['name'] as String),
                subtitle: description != null && description.isNotEmpty
                    ? Text(description)
                    : null,
                trailing: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('Serves ${item['servings'] ?? '-'}'),
                    Mutation(
                      options: MutationOptions(
                        document: gql(setRecipeFavorite),
                        onCompleted: (_) => refetch?.call(),
                      ),
                      builder: (RunMutation runMutation, QueryResult? result) {
                        return IconButton(
                          icon: Icon(isFavorite ? Icons.star : Icons.star_border),
                          onPressed: () => runMutation({
                            'recipeId': item['id'],
                            'isFavorite': !isFavorite,
                          }),
                        );
                      },
                    ),
                  ],
                ),
                onTap: () => Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => EditRecipeScreen(recipeId: item['id'] as String),
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
          MaterialPageRoute(builder: (_) => const EditRecipeScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
