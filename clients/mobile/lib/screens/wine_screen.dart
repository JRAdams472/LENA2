import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String userBottlesQuery = r'''
  query UserBottles {
    userBottles(page: 1, pageSize: 25) {
      items {
        id
        bottle {
          vineyard
          vintageYear
        }
        quantity
        isFavorite
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class WineScreen extends StatelessWidget {
  const WineScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Wine Cellar')),
      body: Query(
        options: QueryOptions(document: gql(userBottlesQuery)),
        builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final items = result.data?['userBottles']?['items'] as List? ?? [];

          return ListView.builder(
            padding: const EdgeInsets.all(16.0),
            itemCount: items.length,
            itemBuilder: (context, index) {
              final item = items[index] as Map<String, dynamic>;
              final bottle = item['bottle'] as Map<String, dynamic>?;
              final name = bottle?['vineyard'] as String? ?? 'Unknown';
              final year = bottle?['vintageYear']?.toString() ?? '';
              final isFavorite = item['isFavorite'] as bool? ?? false;
              return ListTile(
                title: Text('$name $year'),
                subtitle: Text('Quantity: ${item['quantity'] ?? 0}'),
                trailing: Icon(isFavorite ? Icons.favorite : Icons.favorite_border),
              );
            },
          );
        },
      ),
    );
  }
}
