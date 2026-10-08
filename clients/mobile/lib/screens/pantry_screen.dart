import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/empty_state.dart';
import '../widgets/paged_list_view.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';

const int pantryPageSize = 25;

const String pantryQuery = r'''
  query Pantry($page: Int, $pageSize: Int, $search: String) {
    userItems(page: $page, pageSize: $pageSize, search: $search) {
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

class PantryScreen extends StatefulWidget {
  const PantryScreen({super.key});

  @override
  State<PantryScreen> createState() => _PantryScreenState();
}

class _PantryScreenState extends State<PantryScreen> {
  final _searchCtrl = TextEditingController();
  final _debouncer = Debouncer();
  String _search = '';

  @override
  void dispose() {
    _debouncer.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  void _onSearchChanged(String value) {
    _debouncer.run(() {
      final term = value.trim();
      setState(() => _search = term);
      if (term.isNotEmpty) {
        recordSearch(GraphQLProvider.of(context).value, 'item', term);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Pantry')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: TextField(
              controller: _searchCtrl,
              decoration: const InputDecoration(
                labelText: 'Search pantry',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _onSearchChanged,
            ),
          ),
          Expanded(
            child: Query(
              options: QueryOptions(
                document: gql(pantryQuery),
                variables: {
                  'page': 1,
                  'pageSize': 25,
                  'search': _search.isEmpty ? null : _search,
                },
              ),
              builder: (QueryResult result,
                  {VoidCallback? refetch, FetchMore? fetchMore}) {
                return _body(context, result, refetch, fetchMore);
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _body(BuildContext context, QueryResult result, VoidCallback? refetch,
      FetchMore? fetchMore) {
    if (result.isLoading) {
      return const SkeletonList();
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }
    final items = result.data?['userItems']?['items'] as List? ?? [];
    final total =
        result.data?['userItems']?['pageInfo']?['totalCount'] as int? ??
            items.length;
    if (items.isEmpty) {
      return const EmptyState(
        icon: Icons.kitchen_outlined,
        title: 'Your pantry is empty',
        description: 'Scan an item or add one to get started.',
      );
    }
    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: PagedListView(
        loadedCount: items.length,
        totalCount: total,
        onLoadMore: () async {
          if (fetchMore == null) return;
          await fetchMore(FetchMoreOptions(
            variables: {'page': nextPageFor(items.length, pantryPageSize)},
            updateQuery: appendPageItems('userItems'),
          ));
        },
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
