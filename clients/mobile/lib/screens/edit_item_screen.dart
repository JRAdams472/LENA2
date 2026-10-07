import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';

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

const String searchBrandsQuery = r'''
  query SearchBrands($term: String!, $limit: Int) {
    searchBrands(term: $term, limit: $limit) {
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
  final _brandSearchCtrl = TextEditingController();
  final _debouncer = Debouncer();
  String? _categoryId;
  String? _brandId;
  bool _isSaving = false;
  bool _loaded = false;
  List<Map<String, dynamic>> _categories = [];
  List<Map<String, dynamic>> _brands = [];

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _loadData();
    }
  }

  Future<void> _loadBrands(String term) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(QueryOptions(
      document: gql(searchBrandsQuery),
      variables: {'term': term, 'limit': 20},
    ));
    if (!mounted) return;
    setState(() {
      final loaded = (result.data?['searchBrands'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      if (_brandId != null && !loaded.any((b) => b['id'] == _brandId)) {
        final prev = _brands.where((b) => b['id'] == _brandId).toList();
        if (prev.isNotEmpty) loaded.insert(0, prev.first);
      }
      _brands = loaded;
    });
  }

  void _onBrandSearchChanged(String value) {
    _debouncer.run(() {
      final term = value.trim();
      if (term.isNotEmpty) {
        recordSearch(GraphQLProvider.of(context).value, 'brand', term);
      }
      _loadBrands(term);
    });
  }

  Future<void> _loadData() async {
    final client = GraphQLProvider.of(context).value;
    final categoriesResult =
        await client.query(QueryOptions(document: gql(categoriesQuery)));
    if (!mounted) return;
    setState(() {
      _categories = (categoriesResult.data?['categories'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });
    await _loadBrands('');

    if (widget.itemId != null) {
      recordView(client, 'item', widget.itemId!);
      final itemResult = await client.query(
        QueryOptions(
          document: gql(itemQuery),
          variables: {'id': widget.itemId},
        ),
      );
      final item = itemResult.data?['item'] as Map<String, dynamic>?;
      if (item != null && mounted) {
        setState(() {
          _nameController.text = item['name'] as String;
          _unitController.text = item['unit'] as String;
          _upc12Controller.text = (item['upc12'] as String?) ?? '';
          _upc14Controller.text = (item['upc14'] as String?) ?? '';
          _categoryId = (item['category']?['id'] as String?);
          _brandId = (item['brand']?['id'] as String?);
          final brand = item['brand'] as Map<String, dynamic>?;
          if (brand != null && !_brands.any((b) => b['id'] == _brandId)) {
            _brands = [brand, ..._brands];
          }
        });
      }
    }
  }

  @override
  void dispose() {
    _debouncer.dispose();
    _nameController.dispose();
    _unitController.dispose();
    _upc12Controller.dispose();
    _upc14Controller.dispose();
    _brandSearchCtrl.dispose();
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
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            TextField(
              controller: _nameController,
              decoration: const InputDecoration(labelText: 'Name'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _unitController,
              decoration: const InputDecoration(labelText: 'Unit'),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String?>(
              isExpanded: true,
              value: _categoryId,
              decoration: const InputDecoration(labelText: 'Category'),
              items: _categories
                  .map((c) => DropdownMenuItem(
                        value: c['id'] as String,
                        child: Text(c['name'] as String),
                      ))
                  .toList(),
              onChanged: (v) => setState(() => _categoryId = v),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _brandSearchCtrl,
              decoration: const InputDecoration(
                labelText: 'Search brands',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _onBrandSearchChanged,
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String?>(
              isExpanded: true,
              value: _brandId,
              decoration: const InputDecoration(labelText: 'Brand (optional)'),
              items: [
                const DropdownMenuItem(value: null, child: Text('None')),
                ..._brands.map((b) => DropdownMenuItem(
                      value: b['id'] as String,
                      child: Text(b['name'] as String),
                    )),
              ],
              onChanged: (v) => setState(() {
                _brandId = v;
                if (v != null) {
                  recordSelection(
                      GraphQLProvider.of(context).value, 'brand', v);
                }
              }),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _upc12Controller,
              decoration: const InputDecoration(labelText: 'UPC-12 (optional)'),
            ),
            const SizedBox(height: 12),
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
      ),
    );
  }
}
