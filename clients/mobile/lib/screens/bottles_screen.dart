import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'edit_bottle_screen.dart';

const String bottlesQuery = r'''
  query Bottles {
    bottles(page: 1, pageSize: 50) {
      items {
        id
        vineyard
        vintageYear
        bottleSize
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class BottlesScreen extends StatelessWidget {
  const BottlesScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Bottles')),
      body: Query(
        options: QueryOptions(document: gql(bottlesQuery)),
        builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final items = result.data?['bottles']?['items'] as List? ?? [];

          return ListView.builder(
            padding: const EdgeInsets.all(16.0),
            itemCount: items.length,
            itemBuilder: (context, index) {
              final bottle = items[index] as Map<String, dynamic>;
              final name = (bottle['vineyard'] as String?) ?? 'Unknown';
              final year = bottle['vintageYear']?.toString() ?? '';
              return ListTile(
                title: Text('$name $year'),
                subtitle: Text(bottle['bottleSize'] as String? ?? ''),
                onTap: () => Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => EditBottleScreen(bottleId: bottle['id'] as String),
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
          MaterialPageRoute(builder: (_) => const EditBottleScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
