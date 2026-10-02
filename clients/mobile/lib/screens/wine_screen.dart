import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'adjust_bottle_screen.dart';
import 'bottles_screen.dart';

const String userBottlesQuery = r'''
  query UserBottles {
    userBottles(page: 1, pageSize: 25) {
      items {
        id
        bottle {
          id
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

const String setBottleFavoriteMutation = r'''
  mutation SetBottleFavorite($bottleId: ID!, $isFavorite: Boolean!) {
    setBottleFavorite(bottleId: $bottleId, isFavorite: $isFavorite) {
      id
    }
  }
''';

class WineScreen extends StatelessWidget {
  const WineScreen({super.key});

  void _toggleFavorite(
    BuildContext context,
    String bottleId,
    bool isFavorite,
    VoidCallback? refetch,
  ) {
    final client = GraphQLProvider.of(context).value;
    client
        .mutate(MutationOptions(
          document: gql(setBottleFavoriteMutation),
          variables: {'bottleId': bottleId, 'isFavorite': !isFavorite},
        ))
        .then((_) => refetch?.call());
  }

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
              final bottleId = bottle?['id'] as String?;
              final name = bottle?['vineyard'] as String? ?? 'Unknown';
              final year = bottle?['vintageYear']?.toString() ?? '';
              final isFavorite = item['isFavorite'] as bool? ?? false;
              return ListTile(
                title: Text('$name $year'),
                subtitle: Text('Quantity: ${item['quantity'] ?? 0}'),
                trailing: IconButton(
                  icon: Icon(isFavorite ? Icons.favorite : Icons.favorite_border),
                  onPressed: bottleId == null
                      ? null
                      : () => _toggleFavorite(context, bottleId, isFavorite, refetch),
                ),
                onTap: bottleId == null
                    ? null
                    : () => Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (_) => AdjustBottleScreen(
                              bottleId: bottleId,
                              quantity: item['quantity'] as int?,
                            ),
                          ),
                        ),
              );
            },
          );
        },
      ),
      floatingActionButton: Column(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          FloatingActionButton.small(
            heroTag: 'adjust',
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: (_) => const AdjustBottleScreen()),
            ),
            child: const Icon(Icons.add),
          ),
          const SizedBox(height: 8),
          FloatingActionButton(
            heroTag: 'bottles',
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: (_) => const BottlesScreen()),
            ),
            child: const Icon(Icons.wine_bar),
          ),
        ],
      ),
    );
  }
}
