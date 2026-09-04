import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String itemQuery = r'''
  query Item($id: ID!) {
    item(id: $id) {
      id
      name
      unit
      upc12
      upc14
      brand {
        id
        name
      }
      category {
        id
        name
      }
    }
  }
''';

const String categoriesQuery = r'''
  query Categories {
    categories {
      id
      name
    }
  }
''';

const String brandsQuery = r'''
  query Brands {
    brands {
      id
      name
    }
  }
''';

const String createItemMutation = r'''
  mutation CreateItem($input: CreateItemInput!) {
    createItem(input: $input) {
      id
      name
    }
  }
''';

const String updateItemMutation = r'''
  mutation UpdateItem($id: ID!, $input: UpdateItemInput!) {
    updateItem(id: $id, input: $input) {
      id
      name
    }
  }
''';

class EditItemScreen extends StatefulWidget {
  final String? itemId;

  const EditItemScreen({super.key, this.itemId});

  @override
  State<EditItemScreen> createState() => _EditItemScreenState();
}

class _EditItemScreenState extends State<EditItemScreen> {
  final _nameController = TextEditingController();
  final _unitController = TextEditingController();
  final _upc12Controller = TextEditingController();
  final _upc14Controller = TextEditingController();
  String? _categoryId;
  String? _brandId;
  bool _isSaving = false;

  Future<List<QueryResult>> _fetchData(BuildContext context) {
    final client = GraphQLProvider.of(context).value;
    final futures = <Future<QueryResult<Object?>>>[];
    futures.add(client.query(QueryOptions(document: gql(categoriesQuery))));
    futures.add(client.query(QueryOptions(document: gql(brandsQuery))));
    if (widget.itemId != null) {
      futures.add(client.query(
        QueryOptions(
          document: gql(itemQuery),
          variables: {'id': widget.itemId},
        ),
      ));
    }
    return Future.wait(futures);
  }

  @override
  void dispose() {
    _nameController.dispose();
    _unitController.dispose();
    _upc12Controller.dispose();
    _upc14Controller.dispose();
    super.dispose();
  }

  Future<void> _save(BuildContext context) async {
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final input = <String, dynamic>{
        'name': _nameController.text,
        'unit': _unitController.text,
        'categoryId': _categoryId,
        'brandId': _brandId,
        'upc12': _upc12Controller.text.isEmpty ? null : _upc12Controller.text,
        'upc14': _upc14Controller.text.isEmpty ? null : _upc14Controller.text,
      };
      if (widget.itemId == null) {
        await client.mutate(MutationOptions(
          document: gql(createItemMutation),
          variables: {'input': input},
        ));
      } else {
        await client.mutate(MutationOptions(
          document: gql(updateItemMutation),
          variables: {'id': widget.itemId, 'input': input},
        ));
      }
      if (mounted) Navigator.pop(context);
    } finally {
      setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.itemId == null ? 'Create Item' : 'Edit Item'),
      ),
      body: FutureBuilder<List<QueryResult>>(
        future: _fetchData(context),
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text('Error: ${snapshot.error}'));
          }

          final results = snapshot.data ?? [];
          final categories = (results[0].data?['categories'] as List? ?? [])
              .cast<Map<String, dynamic>>();
          final brands = (results[1].data?['brands'] as List? ?? [])
              .cast<Map<String, dynamic>>();

          if (results.length > 2 && results[2].data?['item'] != null) {
            final item = results[2].data!['item'] as Map<String, dynamic>;
            _nameController.text = item['name'] as String;
            _unitController.text = item['unit'] as String;
            _upc12Controller.text = (item['upc12'] as String?) ?? '';
            _upc14Controller.text = (item['upc14'] as String?) ?? '';
            _categoryId = (item['category']?['id'] as String?);
            _brandId = (item['brand']?['id'] as String?);
          }

          return Padding(
            padding: const EdgeInsets.all(16.0),
            child: ListView(
              children: [
                TextField(
                  controller: _nameController,
                  decoration: const InputDecoration(labelText: 'Name'),
                ),
                TextField(
                  controller: _unitController,
                  decoration: const InputDecoration(labelText: 'Unit'),
                ),
                DropdownButtonFormField<String?>(
                  value: _categoryId,
                  decoration: const InputDecoration(labelText: 'Category'),
                  items: categories
                      .map((c) => DropdownMenuItem(
                            value: c['id'] as String,
                            child: Text(c['name'] as String),
                          ))
                      .toList(),
                  onChanged: (v) => setState(() => _categoryId = v),
                ),
                DropdownButtonFormField<String?>(
                  value: _brandId,
                  decoration: const InputDecoration(labelText: 'Brand (optional)'),
                  items: [
                    const DropdownMenuItem(value: null, child: Text('None')),
                    ...brands.map((b) => DropdownMenuItem(
                          value: b['id'] as String,
                          child: Text(b['name'] as String),
                        )),
                  ],
                  onChanged: (v) => setState(() => _brandId = v),
                ),
                TextField(
                  controller: _upc12Controller,
                  decoration: const InputDecoration(labelText: 'UPC-12 (optional)'),
                ),
                TextField(
                  controller: _upc14Controller,
                  decoration: const InputDecoration(labelText: 'UPC-14 (optional)'),
                ),
                const SizedBox(height: 16),
                ElevatedButton(
                  onPressed: _isSaving ? null : () => _save(context),
                  child: _isSaving
                      ? const SizedBox(
                          height: 16,
                          width: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Save'),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
