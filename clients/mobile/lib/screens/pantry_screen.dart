import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String pantryQuery = r'''
  query Pantry($page: Int, $pageSize: Int) {
    userItems(page: $page, pageSize: $pageSize) {
      items {
        id
        item {
          name
        }
        currentQty
        minQty
        notes
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class PantryScreen extends StatelessWidget {
  const PantryScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(pantryQuery),
        variables: const {'page': 1, 'pageSize': 25},
      ),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        return Scaffold(
          appBar: AppBar(title: const Text('Pantry')),
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
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }
    final items = result.data?['userItems']?['items'] as List? ?? [];
    if (items.isEmpty) {
      return const Center(child: Text('Your pantry is empty.'));
    }
    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView.builder(
        padding: const EdgeInsets.all(16.0),
        itemCount: items.length,
        itemBuilder: (context, index) {
          final item = items[index] as Map<String, dynamic>;
          final name = item['item']?['name'] as String? ?? 'Unknown';
          final qty = item['currentQty'] as num? ?? 0;
          final min = item['minQty'] as num?;
          final notes = item['notes'] as String?;
          return Card(
            child: ListTile(
              title: Text(name),
              subtitle: Text(
                'Qty: $qty ${min != null ? '(min: $min)' : ''}${notes != null && notes.isNotEmpty ? '\n$notes' : ''}',
              ),
            ),
          );
        },
      ),
    );
  }
}
