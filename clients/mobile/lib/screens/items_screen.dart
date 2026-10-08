import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/paged_list_view.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';
import 'edit_item_screen.dart';

const int itemsPageSize = 50;

const String itemsQuery = r'''
  query Items($page: Int, $search: String) {
    items(page: $page, pageSize: 50, search: $search) {
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

class ItemsScreen extends StatefulWidget {
  const ItemsScreen({super.key});

  @override
  State<ItemsScreen> createState() => _ItemsScreenState();
}

class _ItemsScreenState extends State<ItemsScreen> {
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
      appBar: AppBar(title: const Text('Items')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: TextField(
              controller: _searchCtrl,
              decoration: const InputDecoration(
                labelText: 'Search items',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _onSearchChanged,
            ),
          ),
          Expanded(
            child: Query(
              options: QueryOptions(
                document: gql(itemsQuery),
                variables: {
                  'page': 1,
                  'search': _search.isEmpty ? null : _search,
                },
              ),
              builder: (QueryResult result,
                  {VoidCallback? refetch, FetchMore? fetchMore}) {
                if (result.isLoading) {
                  return const SkeletonList();
                }
                if (result.hasException) {
                  return Center(
                      child: Text('Error: ${result.exception.toString()}'));
                }

                final items = result.data?['items']?['items'] as List? ?? [];
                final total =
                    result.data?['items']?['pageInfo']?['totalCount'] as int? ??
                        items.length;

                return PagedListView(
                  loadedCount: items.length,
                  totalCount: total,
                  onLoadMore: () async {
                    if (fetchMore == null) return;
                    await fetchMore(FetchMoreOptions(
                      variables: {
                        'page': nextPageFor(items.length, itemsPageSize),
                      },
                      updateQuery: appendPageItems('items'),
                    ));
                  },
                  itemBuilder: (context, index) {
                    final item = items[index] as Map<String, dynamic>;
                    final brand = item['brand']?['name'] as String?;
                    final category = item['category']?['name'] as String?;
                    final subtitle = [
                      item['unit'] as String?,
                      category,
                      brand,
                    ]
                        .whereType<String>()
                        .where((s) => s.isNotEmpty)
                        .join(' · ');
                    return Card(
                      child: ListTile(
                        dense: true,
                        title: Text(item['name'] as String),
                        subtitle: subtitle.isEmpty ? null : Text(subtitle),
                        onTap: () => Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (_) =>
                                EditItemScreen(itemId: item['id'] as String),
                          ),
                        ),
                      ),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        heroTag: 'fab-items',
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => const EditItemScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
