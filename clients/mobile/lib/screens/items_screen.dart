import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'edit_item_screen.dart';

const String itemsQuery = r'''
  query Items {
    items(page: 1, pageSize: 50) {
      items {
        id
        name
        unit
        brand {
          id
          name
        }
        category {
          id
          name
        }
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class ItemsScreen extends StatelessWidget {
  const ItemsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Items')),
      body: Query(
        options: QueryOptions(document: gql(itemsQuery)),
        builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final items = result.data?['items']?['items'] as List? ?? [];

          return ListView.builder(
            padding: const EdgeInsets.all(16.0),
            itemCount: items.length,
            itemBuilder: (context, index) {
              final item = items[index] as Map<String, dynamic>;
              final brand = item['brand']?['name'] as String?;
              final category = item['category']?['name'] as String?;
              return ListTile(
                title: Text(item['name'] as String),
                subtitle: Text(
                  '${item['unit']} ${category != null ? '— $category' : ''} ${brand != null ? '/ $brand' : ''}',
                ),
                onTap: () => Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => EditItemScreen(itemId: item['id'] as String),
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
          MaterialPageRoute(builder: (_) => const EditItemScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
