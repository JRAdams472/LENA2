import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'generate_grocery_dialog.dart';

const String groceryListQuery = r'''
  query GroceryList {
    groceryLists(page: 1, pageSize: 1) {
      items {
        id
        generatedAt
        items {
          id
          item { name }
          manualItemName
          quantityNeeded
          unitOfMeasure
          source
          isChecked
        }
      }
    }
  }
''';

const String itemsQuery = r'''
  query Items {
    items(page: 1, pageSize: 100) {
      items {
        id
        name
      }
    }
  }
'';

const String toggleGroceryItemMutation = r'''
  mutation ToggleGroceryItem($id: ID!) {
    toggleGroceryItemChecked(groceryListItemId: $id) {
      id
      isChecked
    }
  }
'';

const String deleteGroceryItemMutation = r'''
  mutation DeleteGroceryItem($groceryListItemId: ID!) {
    deleteGroceryItem(groceryListItemId: $groceryListItemId)
  }
'';

const String addGroceryItemMutation = r'''
  mutation AddGroceryItem($input: AddGroceryItemInput!) {
    addGroceryItem(input: $input) {
      id
    }
  }
'';

class GroceryScreen extends StatefulWidget {
  const GroceryScreen({super.key});

  @override
  State<GroceryScreen> createState() => _GroceryScreenState();
}

class _GroceryScreenState extends State<GroceryScreen> {
  final _manualCtrl = TextEditingController();
  final _qtyCtrl = TextEditingController();
  final _unitCtrl = TextEditingController();
  String? _selectedItemId;
  List<Map<String, dynamic>> _items = [];

  Future<void> _toggle(
    BuildContext context,
    String id,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(toggleGroceryItemMutation),
      variables: {'id': id},
    ));
    refetch?.call();
  }

  Future<void> _delete(
    BuildContext context,
    String id,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(deleteGroceryItemMutation),
      variables: {'groceryListItemId': id},
    ));
    refetch?.call();
  }

  Future<void> _addItem(
    BuildContext context,
    String listId,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    final manual = _manualCtrl.text;
    final input = <String, dynamic>{
      'groceryListId': listId,
      'itemId': _selectedItemId,
      'manualItemName': manual.isEmpty ? null : manual,
      'quantity': double.tryParse(_qtyCtrl.text) ?? 0,
      'unit': _unitCtrl.text,
    };
    await client.mutate(MutationOptions(
      document: gql(addGroceryItemMutation),
      variables: {'input': input},
    ));
    _manualCtrl.clear();
    _qtyCtrl.clear();
    _unitCtrl.clear();
    setState(() => _selectedItemId = null);
    refetch?.call();
  }

  Future<void> _loadItems(BuildContext context) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(QueryOptions(document: gql(itemsQuery)));
    setState(() {
      _items = (result.data?['items']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _loadItems(context);
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
      options: QueryOptions(document: gql(groceryListQuery)),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        Widget body;
        if (result.isLoading) {
          body = const Center(child: CircularProgressIndicator());
        } else if (result.hasException) {
          body = Center(child: Text('Error: ${result.exception.toString()}'));
        } else {
          final lists = result.data?['groceryLists']?['items'] as List? ?? [];
          if (lists.isEmpty) {
            body = const Center(child: Text('No grocery list found.'));
          } else {
            final list = lists.first;
            final listId = list['id'] as String;
            final items = list['items'] as List? ?? [];
            body = ListView(
              padding: const EdgeInsets.all(16.0),
              children: [
                ...items.asMap().entries.map((e) {
                  final item = e.value as Map<String, dynamic>;
                  final id = item['id'] as String;
                  final name = (item['item']?['name'] as String?) ??
                      (item['manualItemName'] as String?) ??
                      'Unknown';
                  final quantity = item['quantityNeeded'] ?? 0;
                  final unit = item['unitOfMeasure'] as String? ?? '';
                  final isChecked = item['isChecked'] as bool? ?? false;

                  return CheckboxListTile(
                    title: Text('$name — $quantity $unit'),
                    subtitle: Text('Source: ${item['source']}'),
                    value: isChecked,
                    onChanged: (value) => _toggle(context, id, refetch),
                    secondary: IconButton(
                      icon: const Icon(Icons.delete),
                      onPressed: () => _delete(context, id, refetch),
                    ),
                  );
                }).toList(),
                const Divider(),
                const Text('Add item', style: TextStyle(fontWeight: FontWeight.bold)),
                DropdownButtonFormField<String?>(
                  value: _selectedItemId,
                  decoration: const InputDecoration(labelText: 'Catalog item'),
                  items: [
                    const DropdownMenuItem(value: null, child: Text('None')),
                    ..._items.map((i) => DropdownMenuItem(
                          value: i['id'] as String,
                          child: Text(i['name'] as String),
                        ))
                  ],
                  onChanged: (v) => setState(() => _selectedItemId = v),
                ),
                TextField(
                  controller: _manualCtrl,
                  decoration: const InputDecoration(labelText: 'or manual name'),
                ),
                TextField(
                  controller: _qtyCtrl,
                  decoration: const InputDecoration(labelText: 'Quantity'),
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                ),
                TextField(
                  controller: _unitCtrl,
                  decoration: const InputDecoration(labelText: 'Unit'),
                ),
                ElevatedButton(
                  onPressed: () => _addItem(context, listId, refetch),
                  child: const Text('Add'),
                ),
              ],
            );
          }
        }

        return Scaffold(
          appBar: AppBar(title: const Text('Grocery List')),
          body: body,
          floatingActionButton: FloatingActionButton(
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
}
