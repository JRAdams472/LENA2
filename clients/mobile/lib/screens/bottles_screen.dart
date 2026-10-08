import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/paged_list_view.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';
import 'edit_bottle_screen.dart';

const int bottlesPageSize = 50;

const String bottlesQuery = r'''
  query Bottles($page: Int, $search: String) {
    bottles(page: $page, pageSize: 50, search: $search) {
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

class BottlesScreen extends StatefulWidget {
  const BottlesScreen({super.key});

  @override
  State<BottlesScreen> createState() => _BottlesScreenState();
}

class _BottlesScreenState extends State<BottlesScreen> {
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
        recordSearch(GraphQLProvider.of(context).value, 'bottle', term);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Bottles')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: TextField(
              controller: _searchCtrl,
              decoration: const InputDecoration(
                labelText: 'Search bottles',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _onSearchChanged,
            ),
          ),
          Expanded(
            child: Query(
              options: QueryOptions(
                document: gql(bottlesQuery),
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

                final items = result.data?['bottles']?['items'] as List? ?? [];
                final total = result.data?['bottles']?['pageInfo']
                        ?['totalCount'] as int? ??
                    items.length;

                return PagedListView(
                  loadedCount: items.length,
                  totalCount: total,
                  onLoadMore: () async {
                    if (fetchMore == null) return;
                    await fetchMore(FetchMoreOptions(
                      variables: {
                        'page': nextPageFor(items.length, bottlesPageSize),
                      },
                      updateQuery: appendPageItems('bottles'),
                    ));
                  },
                  itemBuilder: (context, index) {
                    final bottle = items[index] as Map<String, dynamic>;
                    final name = (bottle['vineyard'] as String?) ?? 'Unknown';
                    final year = bottle['vintageYear']?.toString() ?? '';
                    return Card(
                      child: ListTile(
                        dense: true,
                        title: Text('$name $year'.trim()),
                        subtitle: Text(bottle['bottleSize'] as String? ?? ''),
                        onTap: () => Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (_) => EditBottleScreen(
                                bottleId: bottle['id'] as String),
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
        heroTag: 'fab-bottles',
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => const EditBottleScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
