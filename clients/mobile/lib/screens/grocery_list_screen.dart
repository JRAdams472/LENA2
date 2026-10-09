import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:url_launcher/url_launcher.dart';
import '../widgets/skeleton.dart';
import '../allergy.dart';
import '../format.dart';

const String groceryRoutingQuery = r'''
  query GroceryRouting($id: ID!) {
    groceryList(id: $id) {
      id
      store { id name }
    }
    groceryStores { id name aisles { id name position } }
    shopperProviders
    groceryRouteGroups(groceryListId: $id) {
      aisle { id name position }
      items {
        suggested
        item {
          id
          item { id name }
          ingredient { id name }
          usualBrand {
            id
            name
            brand { id name }
          }
          manualItemName
          quantityNeeded
          unitOfMeasure
          source
          isChecked
          allergyWarnings {
            memberKind
            entityKind
            member { id displayName firstName lastName }
            allergen { id name }
          }
          allergens {
            kind
            allergen { id name }
          }
        }
      }
    }
  }
''';

const String toggleGroceryItemMutation = r'''
  mutation ToggleGroceryItem($groceryListItemId: ID!) {
    toggleGroceryItemChecked(groceryListItemId: $groceryListItemId) {
      id
      isChecked
    }
  }
''';

const String checkGroceryItemWithBrandMutation = r'''
  mutation CheckGroceryItemWithBrand($groceryListItemId: ID!, $itemId: ID!) {
    checkGroceryItemWithBrand(groceryListItemId: $groceryListItemId, itemId: $itemId) {
      id
      isChecked
    }
  }
''';

const String itemsQuery = r'''
  query Items($search: String) {
    items(page: 1, pageSize: 25, search: $search) {
      items {
        id
        name
        brand { id name }
      }
    }
  }
''';

const String addGroceryItemMutation = r'''
  mutation AddGroceryItem($input: AddGroceryItemInput!) {
    addGroceryItem(input: $input) {
      id
    }
  }
''';

const String setGroceryListStoreMutation = r'''
  mutation SetGroceryListStore($groceryListId: ID!, $storeId: ID) {
    setGroceryListStore(groceryListId: $groceryListId, storeId: $storeId) { id }
  }
''';

const String assignItemToAisleMutation = r'''
  mutation AssignItemToAisle($storeId: ID!, $aisleId: ID, $itemId: ID, $ingredientId: ID, $manualItemName: String) {
    assignItemToAisle(storeId: $storeId, aisleId: $aisleId, itemId: $itemId, ingredientId: $ingredientId, manualItemName: $manualItemName)
  }
''';

const String reorderGroceryListItemsMutation = r'''
  mutation ReorderGroceryListItems($groceryListId: ID!, $entries: [GroceryReorderEntryInput!]!) {
    reorderGroceryListItems(groceryListId: $groceryListId, entries: $entries)
  }
''';

const String createShoppingLinkMutation = r'''
  mutation CreateShoppingLink($groceryListId: ID!, $provider: ShopperProvider, $includeChecked: Boolean) {
    createShoppingLink(groceryListId: $groceryListId, provider: $provider, includeChecked: $includeChecked) {
      provider
      url
    }
  }
''';

/// The server owns ordering — the app renders groceryRouteGroups verbatim so
/// the phone and the web always show the same arrangement. A null aisle is the
/// trailing unassigned bucket ("Other items" on web).

/// Builds the reorderGroceryListItems payload for a post-drag display order.
/// Mirrors the web client's reorderEntries: every item goes in display order;
/// only the moved row carries aisleId — other items keep their assignments and
/// an unset aisleId means "no change", not "unassigned".
List<Map<String, String?>> groceryReorderEntries(
  List<Map<String, dynamic>> groups,
  String movedId,
  int targetGroupIdx,
  int targetItemIdx,
) {
  final flat = <Map<String, dynamic>>[];
  for (var gi = 0; gi < groups.length; gi++) {
    for (final ri in (groups[gi]['items'] as List? ?? [])) {
      flat.add({'item': (ri as Map)['item'], 'groupIdx': gi});
    }
  }

  final from =
      flat.indexWhere((f) => ((f['item'] as Map)['id'] as String) == movedId);
  if (from == -1) return [];
  final moved = flat.removeAt(from);

  // Convert the drop target (group, index-within-group) into a flat insert
  // index in the remaining list.
  var insertAt = flat.length;
  var seen = 0;
  for (var i = 0; i < flat.length; i++) {
    if (flat[i]['groupIdx'] == targetGroupIdx) {
      if (seen == targetItemIdx) {
        insertAt = i;
        break;
      }
      seen++;
      insertAt = i + 1;
    }
  }
  flat.insert(insertAt > flat.length ? flat.length : insertAt, moved);

  final targetAisle = targetGroupIdx < groups.length
      ? groups[targetGroupIdx]['aisle'] as Map?
      : null;
  final targetAisleId = targetAisle?['id'] as String?;
  return flat.map((f) {
    final id = (f['item'] as Map)['id'] as String;
    return {
      'groceryListItemId': id,
      'aisleId': id == movedId ? targetAisleId : null,
    };
  }).toList();
}

/// An unchecked ingredient-bound line with no brand item and no remembered
/// usual needs a first-time brand pick so the check-off can credit a real
/// item and record it as the household's usual brand.
bool needsBrandPick(Map<String, dynamic> item) {
  return (item['isChecked'] as bool? ?? false) == false &&
      item['ingredient'] != null &&
      item['item'] == null &&
      item['usualBrand'] == null;
}

class GroceryListScreen extends StatefulWidget {
  final String listId;

  const GroceryListScreen({super.key, required this.listId});

  @override
  State<GroceryListScreen> createState() => _GroceryListScreenState();
}

class _GroceryListScreenState extends State<GroceryListScreen> {
  final _manualCtrl = TextEditingController();
  final _qtyCtrl = TextEditingController();
  final _unitCtrl = TextEditingController();
  bool _shopping = false;

  Future<void> _toggle(
    BuildContext context,
    Map<String, dynamic> item,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    final id = item['id'] as String;
    // Ingredient-only line with no bound item and no remembered usual —
    // ask which brand was bought so it becomes the household's usual.
    if (needsBrandPick(item)) {
      final picked = await _promptBrandPick(
          context, item['ingredient']['name'] as String? ?? 'item', client);
      if (picked == null) return;
      await client.mutate(MutationOptions(
        document: gql(checkGroceryItemWithBrandMutation),
        variables: {'groceryListItemId': id, 'itemId': picked['id']},
      ));
      refetch?.call();
      return;
    }
    await client.mutate(MutationOptions(
      document: gql(toggleGroceryItemMutation),
      variables: {'groceryListItemId': id},
    ));
    refetch?.call();
  }

  /// First-time brand picker: searches catalog items and returns the pick
  /// (credited to the line + recorded as the usual brand) or null on cancel.
  Future<Map<String, dynamic>?> _promptBrandPick(
    BuildContext context,
    String ingredientName,
    GraphQLClient client,
  ) {
    return showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => _BrandPickDialog(
        ingredientName: ingredientName,
        client: client,
      ),
    );
  }

  Future<void> _addItem(
    BuildContext context,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    final manual = _manualCtrl.text.trim();
    final qty = double.tryParse(_qtyCtrl.text) ?? 0;
    if (manual.isEmpty || qty <= 0) return;

    await client.mutate(MutationOptions(
      document: gql(addGroceryItemMutation),
      variables: {
        'input': {
          'groceryListId': widget.listId,
          'manualItemName': manual,
          'quantity': qty,
          'unit': _unitCtrl.text,
        },
      },
    ));
    _manualCtrl.clear();
    _qtyCtrl.clear();
    _unitCtrl.clear();
    refetch?.call();
  }

  /// Pushes the list to the configured shopper provider and hands the
  /// returned link to the user. Mobile sends unchecked lines only — the
  /// web's includeChecked escape hatch stays out of the compact UI.
  Future<void> _shop(BuildContext context, bool hasChecked) async {
    if (_shopping) return;
    setState(() => _shopping = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final result = await client.mutate(MutationOptions(
        document: gql(createShoppingLinkMutation),
        variables: {'groceryListId': widget.listId, 'provider': 'INSTACART'},
      ));
      if (!context.mounted) return;
      final link = result.data?['createShoppingLink'] as Map<String, dynamic>?;
      if (result.hasException || link == null) {
        // Server messages are already sanitized for clients.
        final msg = result.exception?.graphqlErrors.isNotEmpty == true
            ? result.exception!.graphqlErrors.first.message
            : 'Could not create the Instacart link';
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(msg)));
        return;
      }
      setState(() => _shopping = false);
      await showDialog<void>(
        context: context,
        builder: (_) => ShopLinkDialog(
          url: link['url'] as String,
          excludedChecked: hasChecked,
        ),
      );
    } finally {
      if (mounted) setState(() => _shopping = false);
    }
  }

  Future<void> _setStore(
    BuildContext context,
    String? storeId,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(setGroceryListStoreMutation),
      variables: {'groceryListId': widget.listId, 'storeId': storeId},
    ));
    refetch?.call();
  }

  /// Reorders an item to (group, index-within-group), moving it to that aisle
  /// when the drop crosses a boundary. Dropping into the unassigned bucket
  /// clears the aisle explicitly — a null aisleId in the reorder payload means
  /// "no change", so unassignment goes through assignItemToAisle first.
  Future<void> _submitOrder(
    BuildContext context,
    List<Map<String, dynamic>> groups,
    String? storeId,
    String movedId,
    int targetGroupIdx,
    int targetItemIdx,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    final entries =
        groceryReorderEntries(groups, movedId, targetGroupIdx, targetItemIdx);
    if (entries.isEmpty) return;

    final targetAisle = targetGroupIdx < groups.length
        ? groups[targetGroupIdx]['aisle'] as Map?
        : null;
    Map<String, dynamic>? movedItem;
    var wasAssigned = false;
    for (final g in groups) {
      for (final ri in (g['items'] as List? ?? [])) {
        if (((ri as Map)['item'] as Map)['id'] == movedId) {
          movedItem = (ri['item'] as Map).cast<String, dynamic>();
          wasAssigned = g['aisle'] != null &&
              (ri['suggested'] as bool? ?? false) == false;
        }
      }
    }

    if (targetAisle == null &&
        wasAssigned &&
        storeId != null &&
        movedItem != null) {
      await client.mutate(MutationOptions(
        document: gql(assignItemToAisleMutation),
        variables: {
          'storeId': storeId,
          'aisleId': null,
          'itemId': (movedItem['item'] as Map?)?['id'],
          'ingredientId': (movedItem['ingredient'] as Map?)?['id'],
          'manualItemName': movedItem['manualItemName'],
        },
      ));
    }

    await client.mutate(MutationOptions(
      document: gql(reorderGroceryListItemsMutation),
      variables: {'groceryListId': widget.listId, 'entries': entries},
    ));
    refetch?.call();
  }

  @override
  void dispose() {
    _manualCtrl.dispose();
    _qtyCtrl.dispose();
    _unitCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(groceryRoutingQuery),
        variables: {'id': widget.listId},
      ),
      builder: (QueryResult result,
          {VoidCallback? refetch, FetchMore? fetchMore}) {
        final list = result.data?['groceryList'] as Map<String, dynamic>?;
        final storeId = (list?['store'] as Map?)?['id'] as String?;
        final canShop = (result.data?['shopperProviders'] as List? ?? const [])
            .contains('INSTACART');
        final hasChecked = (result.data?['groceryRouteGroups'] as List? ??
                const [])
            .expand((g) => (g as Map)['items'] as List? ?? const [])
            .any((ri) => ((ri as Map)['item'] as Map?)?['isChecked'] == true);
        return Scaffold(
          appBar: AppBar(
            title: const Text('Grocery List'),
            actions: [
              if (canShop)
                IconButton(
                  tooltip: 'Shop with Instacart',
                  onPressed:
                      _shopping ? null : () => _shop(context, hasChecked),
                  icon: _shopping
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.shopping_cart_checkout),
                ),
              _storePicker(context, result, storeId, refetch),
            ],
          ),
          body: _body(context, result, storeId, refetch),
        );
      },
    );
  }

  Widget _storePicker(
    BuildContext context,
    QueryResult result,
    String? storeId,
    VoidCallback? refetch,
  ) {
    final stores = result.data?['groceryStores'] as List? ?? const [];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8.0),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String?>(
          value: storeId,
          hint: const Text('Store'),
          onChanged: (v) => _setStore(context, v, refetch),
          items: [
            const DropdownMenuItem<String?>(
              value: null,
              child: Text('No store'),
            ),
            ...stores.map(
              (s) => DropdownMenuItem<String?>(
                value: (s as Map)['id'] as String,
                child: Text(s['name'] as String? ?? ''),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _body(
    BuildContext context,
    QueryResult result,
    String? storeId,
    VoidCallback? refetch,
  ) {
    if (result.isLoading) {
      return const SkeletonList();
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }

    final groups = (result.data?['groceryRouteGroups'] as List? ?? [])
        .cast<Map<String, dynamic>>()
        .where((g) => (g['items'] as List? ?? []).isNotEmpty)
        .toList();
    final storeAisles = _aislesForStore(result, storeId);

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          for (var gi = 0; gi < groups.length; gi++)
            _groupSection(context, groups, gi, storeId, storeAisles, refetch),
          const Divider(),
          const Text('Add item', style: TextStyle(fontWeight: FontWeight.bold)),
          TextField(
            controller: _manualCtrl,
            decoration: const InputDecoration(labelText: 'Item name'),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _qtyCtrl,
            decoration: const InputDecoration(labelText: 'Quantity'),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _unitCtrl,
            decoration: const InputDecoration(labelText: 'Unit'),
          ),
          ElevatedButton(
            onPressed: () => _addItem(context, refetch),
            child: const Text('Add'),
          ),
        ],
      ),
    );
  }

  /// The store's aisle list feeds the "move to aisle" menu.
  List<Map<String, dynamic>> _aislesForStore(
      QueryResult result, String? storeId) {
    if (storeId == null) return const [];
    final stores = result.data?['groceryStores'] as List? ?? [];
    for (final s in stores) {
      if ((s as Map)['id'] == storeId) {
        return ((s['aisles'] as List? ?? []).cast<Map<String, dynamic>>())
          ..sort(
              (a, b) => (a['position'] as num).compareTo(b['position'] as num));
      }
    }
    return const [];
  }

  Widget _groupSection(
    BuildContext context,
    List<Map<String, dynamic>> groups,
    int gi,
    String? storeId,
    List<Map<String, dynamic>> storeAisles,
    VoidCallback? refetch,
  ) {
    final group = groups[gi];
    final aisle = group['aisle'] as Map?;
    final items = (group['items'] as List? ?? []).cast<Map<String, dynamic>>();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: 8.0, bottom: 4.0),
          child: Text(
            (aisle?['name'] as String?) ?? 'Other items',
            style: Theme.of(context).textTheme.titleMedium,
          ),
        ),
        ReorderableListView(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          buildDefaultDragHandles: true,
          onReorderItem: (oldIndex, newIndex) {
            // onReorderItem reports newIndex already adjusted for removal.
            if (newIndex == oldIndex) return;
            final movedId = (items[oldIndex]['item'] as Map)['id'] as String;
            _submitOrder(
                context, groups, storeId, movedId, gi, newIndex, refetch);
          },
          children: [
            for (final ri in items)
              _itemTile(context, ri, storeId, storeAisles, groups, refetch),
          ],
        ),
      ],
    );
  }

  Widget _itemTile(
    BuildContext context,
    Map<String, dynamic> routeItem,
    String? storeId,
    List<Map<String, dynamic>> storeAisles,
    List<Map<String, dynamic>> groups,
    VoidCallback? refetch,
  ) {
    final item = (routeItem['item'] as Map).cast<String, dynamic>();
    final id = item['id'] as String;
    final suggested = routeItem['suggested'] as bool? ?? false;
    final name = (item['item']?['name'] as String?) ??
        (item['ingredient']?['name'] as String?) ??
        (item['manualItemName'] as String?) ??
        'Unknown';
    final quantity = item['quantityNeeded'] as num? ?? 0;
    final unit = item['unitOfMeasure'] as String? ?? '';
    final isChecked = item['isChecked'] as bool? ?? false;
    final usual = item['usualBrand'] as Map<String, dynamic>?;
    final usualBrandName = usual == null
        ? null
        : brandedName(
            (usual['brand'] as Map?)?['name'] as String?,
            usual['name'] as String?,
          );

    return CheckboxListTile(
      key: ValueKey('grocery-item-$id'),
      title: Row(
        children: [
          Expanded(child: Text('$name — $quantity $unit')),
          AllergyWarningBadge(warnings: allergyWarningsOf(item)),
          if (suggested)
            Padding(
              padding: const EdgeInsets.only(left: 4.0),
              child: Text(
                'suggested aisle',
                style: Theme.of(context)
                    .textTheme
                    .labelSmall
                    ?.copyWith(fontStyle: FontStyle.italic, letterSpacing: 0),
              ),
            ),
        ],
      ),
      subtitle: Text(
        humanSource(item['source'] as String?) +
            (usualBrandName != null && usualBrandName.isNotEmpty
                ? ' · usually buys $usualBrandName'
                : ''),
      ),
      value: isChecked,
      onChanged: (_) => _toggle(context, item, refetch),
      secondary: storeAisles.isEmpty
          ? null
          // PopupMenuItem pops the route with its value — a null value reads
          // as dismissal and never reaches onSelected, so unassigned uses a
          // sentinel instead.
          : PopupMenuButton<String>(
              icon: const Icon(Icons.more_vert),
              tooltip: 'Move to aisle',
              onSelected: (aisleId) {
                final target = groups.indexWhere((g) =>
                    (g['aisle'] as Map?)?['id'] ==
                    (aisleId.isEmpty ? null : aisleId));
                final idx = target == -1 ? groups.length - 1 : target;
                final count = target == -1
                    ? 0
                    : (groups[idx]['items'] as List? ?? []).length;
                _submitOrder(context, groups, storeId, id, idx < 0 ? 0 : idx,
                    count, refetch);
              },
              itemBuilder: (_) => [
                ...storeAisles.map(
                  (a) => PopupMenuItem<String>(
                    value: a['id'] as String,
                    child: Text('Move to ${a['name']}'),
                  ),
                ),
                const PopupMenuItem<String>(
                  value: '',
                  child: Text('Move to unassigned'),
                ),
              ],
            ),
    );
  }
}

/// Searchable item picker shown when an ingredient-only grocery line is
/// checked off for the first time. Pops with the picked catalog item so the
/// caller can credit the check-off and record it as the household's usual.
class _BrandPickDialog extends StatefulWidget {
  final String ingredientName;
  final GraphQLClient client;

  const _BrandPickDialog({required this.ingredientName, required this.client});

  @override
  State<_BrandPickDialog> createState() => _BrandPickDialogState();
}

class _BrandPickDialogState extends State<_BrandPickDialog> {
  final _searchCtrl = TextEditingController();
  List<Map<String, dynamic>> _results = [];
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _searchCtrl.text = widget.ingredientName;
    _search(widget.ingredientName);
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  Future<void> _search(String term) async {
    setState(() => _loading = true);
    final result = await widget.client.query(QueryOptions(
      document: gql(itemsQuery),
      variables: {'search': term.isEmpty ? null : term},
      fetchPolicy: FetchPolicy.networkOnly,
    ));
    if (!mounted) return;
    setState(() {
      _loading = false;
      _results = (result.data?['items']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text('Which ${widget.ingredientName} did you buy?'),
      content: SizedBox(
        width: double.maxFinite,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              "Pick the brand you grabbed — we'll remember it as your usual "
              'for ${widget.ingredientName}.',
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _searchCtrl,
              decoration: const InputDecoration(
                labelText: 'Brand or item',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _search,
            ),
            const SizedBox(height: 8),
            if (_loading)
              const Padding(
                padding: EdgeInsets.all(16.0),
                child: CircularProgressIndicator(),
              )
            else
              Flexible(
                child: ListView(
                  shrinkWrap: true,
                  children: [
                    for (final it in _results)
                      ListTile(
                        title: Text(it['name'] as String? ?? ''),
                        subtitle: (it['brand'] as Map?)?['name'] != null
                            ? Text((it['brand'] as Map)['name'] as String)
                            : null,
                        onTap: () => Navigator.of(context).pop(it),
                      ),
                  ],
                ),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
      ],
    );
  }
}

/// Hands the freshly generated provider link to the user — checkout happens
/// on the provider's site, so the dialog's only job is to move the URL out
/// of LENA (open externally or copy) and state the link's scope.
class ShopLinkDialog extends StatelessWidget {
  final String url;
  final bool excludedChecked;

  const ShopLinkDialog({
    super.key,
    required this.url,
    this.excludedChecked = false,
  });

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Shop with Instacart'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            excludedChecked
                ? 'Unchecked items from this list are ready on Instacart. '
                    'Checked items were left off.'
                : 'Items from this list are ready on Instacart.',
          ),
          const SizedBox(height: 8),
          Text(
            'Open the link to pick a store and finish checking out there.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 12),
          SelectableText(
            url,
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ),
      actions: [
        TextButton.icon(
          icon: const Icon(Icons.copy),
          label: const Text('Copy link'),
          onPressed: () async {
            await Clipboard.setData(ClipboardData(text: url));
            if (context.mounted) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Link copied')),
              );
            }
          },
        ),
        FilledButton.icon(
          icon: const Icon(Icons.open_in_new),
          label: const Text('Open Instacart'),
          onPressed: () => launchUrl(
            Uri.parse(url),
            mode: LaunchMode.externalApplication,
          ),
        ),
      ],
    );
  }
}
