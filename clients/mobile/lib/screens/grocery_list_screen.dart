import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String groceryListQuery = r'''
  query GroceryList($id: ID!) {
    groceryList(id: $id) {
      id
      items {
        id
        item {
          name
        }
        manualItemName
        quantityNeeded
        unitOfMeasure
        source
        isChecked
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

const String addGroceryItemMutation = r'''
  mutation AddGroceryItem($input: AddGroceryItemInput!) {
    addGroceryItem(input: $input) {
      id
    }
  }
''';

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

  Future<void> _toggle(
    BuildContext context,
    String id,
    VoidCallback? refetch,
  ) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(toggleGroceryItemMutation),
      variables: {'groceryListItemId': id},
    ));
    refetch?.call();
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
        document: gql(groceryListQuery),
        variables: {'id': widget.listId},
      ),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        return Scaffold(
          appBar: AppBar(title: const Text('Grocery List')),
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
    final items = result.data?['groceryList']?['items'] as List? ?? [];

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          ...items.map((raw) {
            final item = raw as Map<String, dynamic>;
            final id = item['id'] as String;
            final name = (item['item']?['name'] as String?) ??
                (item['manualItemName'] as String?) ??
                'Unknown';
            final quantity = item['quantityNeeded'] as num? ?? 0;
            final unit = item['unitOfMeasure'] as String? ?? '';
            final isChecked = item['isChecked'] as bool? ?? false;

            return CheckboxListTile(
              title: Text('$name — $quantity $unit'),
              subtitle: Text('Source: ${item['source']}'),
              value: isChecked,
              onChanged: (_) => _toggle(context, id, refetch),
            );
          }),
          const Divider(),
          const Text('Add item', style: TextStyle(fontWeight: FontWeight.bold)),
          TextField(
            controller: _manualCtrl,
            decoration: const InputDecoration(labelText: 'Item name'),
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
            onPressed: () => _addItem(context, refetch),
            child: const Text('Add'),
          ),
        ],
      ),
    );
  }
}
