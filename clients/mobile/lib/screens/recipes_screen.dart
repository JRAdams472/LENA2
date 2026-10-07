import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/empty_state.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';
import '../theme.dart';
import 'edit_recipe_screen.dart';

const String recipesQuery = r'''
  query Recipes($search: String, $categoryIds: [ID!], $isFavorite: Boolean) {
    recipes(page: 1, pageSize: 25, search: $search, categoryIds: $categoryIds, isFavorite: $isFavorite) {
      items {
        id
        name
        description
        servings
        prepTimeMinutes
        cookTimeMinutes
        isFavorite
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

const String recipeCategoryGroupsQuery = r'''
  query RecipeCategoryGroups {
    recipeCategoryGroups {
      id
      name
      exclusive
      displayOrder
      categories {
        id
        name
      }
    }
  }
''';

const String setRecipeFavorite = r'''
  mutation SetRecipeFavorite($recipeId: ID!, $isFavorite: Boolean!) {
    setRecipeFavorite(recipeId: $recipeId, isFavorite: $isFavorite)
  }
''';

class RecipesScreen extends StatefulWidget {
  const RecipesScreen({super.key});

  @override
  State<RecipesScreen> createState() => _RecipesScreenState();
}

class _RecipesScreenState extends State<RecipesScreen> {
  final _searchCtrl = TextEditingController();
  final _debouncer = Debouncer();
  String _search = '';
  bool _favoritesOnly = false;
  final Set<String> _selectedCategoryIds = {};

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
        recordSearch(GraphQLProvider.of(context).value, 'recipe', term);
      }
    });
  }

  Map<String, dynamic> get _variables => {
        'search': _search.isEmpty ? null : _search,
        'categoryIds':
            _selectedCategoryIds.isEmpty ? null : _selectedCategoryIds.toList(),
        'isFavorite': _favoritesOnly ? true : null,
      };

  Future<void> _openFilters() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) {
        return StatefulBuilder(
          builder: (context, setSheetState) {
            return Padding(
              padding: EdgeInsets.only(
                left: 16,
                right: 16,
                top: 16,
                bottom: MediaQuery.of(context).viewInsets.bottom + 16,
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          'Filter by category',
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                      ),
                      TextButton(
                        onPressed: () {
                          setSheetState(() => _selectedCategoryIds.clear());
                          setState(() {});
                        },
                        child: const Text('Clear'),
                      ),
                    ],
                  ),
                  Query(
                    options:
                        QueryOptions(document: gql(recipeCategoryGroupsQuery)),
                    builder: (result, {refetch, fetchMore}) {
                      if (result.isLoading) {
                        return const Padding(
                          padding: EdgeInsets.all(32),
                          child: Center(child: CircularProgressIndicator()),
                        );
                      }
                      final groups =
                          result.data?['recipeCategoryGroups'] as List? ?? [];
                      return SingleChildScrollView(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            for (final group in groups) ...[
                              const SizedBox(height: 8),
                              Text(
                                '${group['name']}'
                                '${group['exclusive'] == true ? ' (pick one)' : ''}',
                                style: const TextStyle(
                                    fontWeight: FontWeight.bold),
                              ),
                              Wrap(
                                spacing: 8,
                                children: [
                                  for (final cat
                                      in (group['categories'] as List? ?? []))
                                    if (group['exclusive'] == true)
                                      ChoiceChip(
                                        label: Text(cat['name'] as String),
                                        selected: _selectedCategoryIds
                                            .contains(cat['id']),
                                        onSelected: (sel) {
                                          setSheetState(() {
                                            final ids = (group['categories']
                                                        as List? ??
                                                    [])
                                                .map((c) => c['id'] as String);
                                            _selectedCategoryIds.removeAll(ids);
                                            if (sel) {
                                              _selectedCategoryIds
                                                  .add(cat['id'] as String);
                                            }
                                          });
                                          setState(() {});
                                        },
                                      )
                                    else
                                      FilterChip(
                                        label: Text(cat['name'] as String),
                                        selected: _selectedCategoryIds
                                            .contains(cat['id']),
                                        onSelected: (sel) {
                                          setSheetState(() {
                                            if (sel) {
                                              _selectedCategoryIds
                                                  .add(cat['id'] as String);
                                            } else {
                                              _selectedCategoryIds
                                                  .remove(cat['id']);
                                            }
                                          });
                                          setState(() {});
                                        },
                                      ),
                                ],
                              ),
                            ],
                          ],
                        ),
                      );
                    },
                  ),
                  const SizedBox(height: 16),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: () => Navigator.pop(sheetContext),
                      child: const Text('Done'),
                    ),
                  ),
                ],
              ),
            );
          },
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Recipes'),
        actions: [
          IconButton(
            icon: Icon(
              _favoritesOnly ? Icons.star : Icons.star_border,
              color: _favoritesOnly
                  ? (Theme.of(context).brightness == Brightness.dark
                      ? lenaDarkWheat
                      : lenaWheat)
                  : null,
            ),
            tooltip: 'Favorites only',
            onPressed: () => setState(() => _favoritesOnly = !_favoritesOnly),
          ),
          IconButton(
            icon: Badge(
              isLabelVisible: _selectedCategoryIds.isNotEmpty,
              label: Text('${_selectedCategoryIds.length}'),
              child: const Icon(Icons.filter_list),
            ),
            tooltip: 'Filter by category',
            onPressed: _openFilters,
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: TextField(
              controller: _searchCtrl,
              decoration: InputDecoration(
                hintText: 'Search recipes',
                prefixIcon: const Icon(Icons.search),
                suffixIcon: _searchCtrl.text.isEmpty
                    ? null
                    : IconButton(
                        icon: const Icon(Icons.clear),
                        onPressed: () {
                          _searchCtrl.clear();
                          _onSearchChanged('');
                        },
                      ),
                border: const OutlineInputBorder(),
                isDense: true,
              ),
              onChanged: _onSearchChanged,
            ),
          ),
          Expanded(
            child: Query(
              options: QueryOptions(
                document: gql(recipesQuery),
                variables: _variables,
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

                final items = result.data?['recipes']?['items'] as List? ?? [];

                if (items.isEmpty) {
                  return const EmptyState(
                    icon: Icons.restaurant_menu,
                    title: 'No recipes found',
                    description: 'Try a different search or filter.',
                  );
                }

                return ListView.builder(
                  padding: const EdgeInsets.all(16.0),
                  itemCount: items.length,
                  itemBuilder: (context, index) {
                    final item = items[index] as Map<String, dynamic>;
                    final description = item['description'] as String?;
                    final isFavorite = item['isFavorite'] as bool? ?? false;
                    return Card(
                      child: ListTile(
                        dense: true,
                        title: Text(item['name'] as String),
                        subtitle: description != null && description.isNotEmpty
                            ? Text(description,
                                maxLines: 2, overflow: TextOverflow.ellipsis)
                            : null,
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text('Serves ${item['servings'] ?? '-'}'),
                            Mutation(
                              options: MutationOptions(
                                document: gql(setRecipeFavorite),
                                onCompleted: (_) => refetch?.call(),
                              ),
                              builder: (RunMutation runMutation,
                                  QueryResult? result) {
                                return IconButton(
                                  icon: Icon(isFavorite
                                      ? Icons.star
                                      : Icons.star_border),
                                  onPressed: () => runMutation({
                                    'recipeId': item['id'],
                                    'isFavorite': !isFavorite,
                                  }),
                                );
                              },
                            ),
                          ],
                        ),
                        onTap: () => Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (_) => EditRecipeScreen(
                                recipeId: item['id'] as String),
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
        heroTag: 'fab-recipes',
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => const EditRecipeScreen()),
        ),
        child: const Icon(Icons.add),
      ),
    );
  }
}
