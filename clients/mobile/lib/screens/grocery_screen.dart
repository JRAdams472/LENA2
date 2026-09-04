import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'generate_grocery_dialog.dart';

const String groceryListQuery = r'''
  query GroceryList {
    groceryLists(page: 1, pageSize: 1) {
      items {
        id
        generatedAt
        items {
          id
          item { name }
          manualItemName
          quantityNeeded
          unitOfMeasure
          source
          isChecked
        }
      }
    }
  }
''';

const String toggleGroceryItemMutation = r'''
  mutation ToggleGroceryItem($id: ID!) {
    toggleGroceryItemChecked(groceryListItemId: $id) {
      id
      isChecked
    }
  }
''';

class GroceryScreen extends StatelessWidget {
  const GroceryScreen({super.key});

  Future<void> _toggle(
    BuildContext context,
    String id,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(toggleGroceryItemMutation),
      variables: {'id': id},
    ));
    refetch?.call();
  }

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(document: gql(groceryListQuery)),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        Widget body;
        if (result.isLoading) {
          body = const Center(child: CircularProgressIndicator());
        } else if (result.hasException) {
          body = Center(child: Text('Error: ${result.exception.toString()}'));
        } else {
          final lists = result.data?['groceryLists']?['items'] as List? ?? [];
          if (lists.isEmpty) {
            body = const Center(child: Text('No grocery list found.'));
          } else {
            final list = lists.first;
            final items = list['items'] as List? ?? [];
            body = ListView.builder(
              padding: const EdgeInsets.all(16.0),
              itemCount: items.length,
              itemBuilder: (context, index) {
                final item = items[index] as Map<String, dynamic>;
                final id = item['id'] as String;
                final name = (item['item']?['name'] as String?) ??
                    (item['manualItemName'] as String?) ??
                    'Unknown';
                final quantity = item['quantityNeeded'] ?? 0;
                final unit = item['unitOfMeasure'] as String? ?? '';
                final isChecked = item['isChecked'] as bool? ?? false;

                return CheckboxListTile(
                  title: Text('$name — $quantity $unit'),
                  subtitle: Text('Source: ${item['source']}'),
                  value: isChecked,
                  onChanged: (value) { _toggle(context, id, refetch); },
                );
              },
            );
          }
        }

        return Scaffold(
          appBar: AppBar(title: const Text('Grocery List')),
          body: body,
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
}
