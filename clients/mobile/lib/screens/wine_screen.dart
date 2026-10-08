import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/paged_list_view.dart';
import '../widgets/skeleton.dart';
import 'adjust_bottle_screen.dart';
import 'bottles_screen.dart';

const int userBottlesPageSize = 25;

const String userBottlesQuery = r'''
  query UserBottles($page: Int) {
    userBottles(page: $page, pageSize: 25) {
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
      appBar: AppBar(
        title: const Text('Wine Cellar'),
        actions: [
          IconButton(
            icon: const Icon(Icons.wine_bar),
            tooltip: 'Bottle catalog',
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: (_) => const BottlesScreen()),
            ),
          ),
        ],
      ),
      body: Query(
        options: QueryOptions(document: gql(userBottlesQuery)),
        builder: (QueryResult result,
            {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const SkeletonList();
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final items = result.data?['userBottles']?['items'] as List? ?? [];
          final total =
              result.data?['userBottles']?['pageInfo']?['totalCount'] as int? ??
                  items.length;

          return PagedListView(
            loadedCount: items.length,
            totalCount: total,
            onLoadMore: () async {
              if (fetchMore == null) return;
              await fetchMore(FetchMoreOptions(
                variables: {
                  'page': nextPageFor(items.length, userBottlesPageSize),
                },
                updateQuery: appendPageItems('userBottles'),
              ));
            },
            itemBuilder: (context, index) {
              final item = items[index] as Map<String, dynamic>;
              final bottle = item['bottle'] as Map<String, dynamic>?;
              final bottleId = bottle?['id'] as String?;
              final name = bottle?['vineyard'] as String? ?? 'Unknown';
              final year = bottle?['vintageYear']?.toString() ?? '';
              final isFavorite = item['isFavorite'] as bool? ?? false;
              return Card(
                child: ListTile(
                  dense: true,
                  title: Text('$name $year'.trim()),
                  subtitle: Text('Quantity: ${item['quantity'] ?? 0}'),
                  trailing: IconButton(
                    icon: Icon(
                        isFavorite ? Icons.favorite : Icons.favorite_border),
                    onPressed: bottleId == null
                        ? null
                        : () => _toggleFavorite(
                            context, bottleId, isFavorite, refetch),
                  ),
                  onTap: bottleId == null
                      ? null
                      : () => Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (_) => AdjustBottleScreen(
                                bottleId: bottleId,
                                bottleName: '$name $year'.trim(),
                                quantity: item['quantity'] as int?,
                              ),
                            ),
                          ),
                ),
              );
            },
          );
        },
      ),
      floatingActionButton: FloatingActionButton.extended(
        heroTag: 'adjust',
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => const AdjustBottleScreen()),
        ),
        icon: const Icon(Icons.add),
        label: const Text('Adjust'),
      ),
    );
  }
}
