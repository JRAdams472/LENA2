import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'generate_grocery_dialog.dart';
import 'grocery_list_screen.dart';

const String groceryListsQuery = r'''
  query GroceryLists($page: Int, $pageSize: Int) {
    groceryLists(page: $page, pageSize: $pageSize) {
      items {
        id
        generatedAt
      }
      pageInfo {
        pageNumber
        pageSize
        totalCount
      }
    }
  }
''';

class GroceryListsScreen extends StatelessWidget {
  const GroceryListsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(groceryListsQuery),
        variables: const {'page': 1, 'pageSize': 25},
      ),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        return Scaffold(
          appBar: AppBar(title: const Text('Grocery Lists')),
          body: _body(context, result, refetch),
          floatingActionButton: FloatingActionButton(
            onPressed: () => showDialog(
              context: context,
              builder: (_) => GenerateGroceryDialog(onGenerated: refetch),
            ),
            child: const Icon(Icons.add),
          ),
        );
      },
    );
  }

  Widget _body(BuildContext context, QueryResult result, VoidCallback? refetch) {
    if (result.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }
    final lists = result.data?['groceryLists']?['items'] as List? ?? [];
    if (lists.isEmpty) {
      return const Center(child: Text('No grocery lists yet.'));
    }
    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView.builder(
        padding: const EdgeInsets.all(16.0),
        itemCount: lists.length,
        itemBuilder: (context, index) {
          final list = lists[index] as Map<String, dynamic>;
          final id = list['id'] as String;
          final generatedAt = list['generatedAt'] as String? ?? '';
          return Card(
            child: ListTile(
              title: Text('List ${index + 1}'),
              subtitle: Text('Generated: $generatedAt'),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => GroceryListScreen(listId: id),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
