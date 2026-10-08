import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/empty_state.dart';
import '../widgets/paged_list_view.dart';
import '../widgets/skeleton.dart';
import '../format.dart';
import 'generate_grocery_dialog.dart';
import 'grocery_list_screen.dart';

const int groceryListsPageSize = 25;

const String groceryListsQuery = r'''
  query GroceryLists($page: Int, $pageSize: Int) {
    groceryLists(page: $page, pageSize: $pageSize) {
      items {
        id
        generatedAt
      }
      pageInfo {
        pageNumber
        pageSize
        totalCount
      }
    }
  }
''';

class GroceryListsScreen extends StatelessWidget {
  const GroceryListsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(groceryListsQuery),
        variables: const {'page': 1, 'pageSize': 25},
      ),
      builder: (QueryResult result,
          {VoidCallback? refetch, FetchMore? fetchMore}) {
        return Scaffold(
          appBar: AppBar(title: const Text('Grocery Lists')),
          body: _body(context, result, refetch, fetchMore),
          floatingActionButton: FloatingActionButton(
            // Tabs live together in MainScreen's IndexedStack — every
            // tab FAB needs a unique heroTag or route transitions crash.
            heroTag: 'fab-grocery-lists',
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

  Widget _body(BuildContext context, QueryResult result, VoidCallback? refetch,
      FetchMore? fetchMore) {
    if (result.isLoading) {
      return const SkeletonList();
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }
    final lists = result.data?['groceryLists']?['items'] as List? ?? [];
    final total =
        result.data?['groceryLists']?['pageInfo']?['totalCount'] as int? ??
            lists.length;
    if (lists.isEmpty) {
      return const EmptyState(
        icon: Icons.shopping_cart_outlined,
        title: 'No grocery lists yet',
        description: 'Generate one from a meal plan.',
      );
    }
    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: PagedListView(
        loadedCount: lists.length,
        totalCount: total,
        onLoadMore: () async {
          if (fetchMore == null) return;
          await fetchMore(FetchMoreOptions(
            variables: {
              'page': nextPageFor(lists.length, groceryListsPageSize),
            },
            updateQuery: appendPageItems('groceryLists'),
          ));
        },
        itemBuilder: (context, index) {
          final list = lists[index] as Map<String, dynamic>;
          final id = list['id'] as String;
          final generatedAt = list['generatedAt'] as String? ?? '';
          return Card(
            child: ListTile(
              title: Text('List ${index + 1}'),
              subtitle: Text('Generated ${fmtIso(generatedAt)}'),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => GroceryListScreen(listId: id),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
