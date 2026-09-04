import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String itemsQuery = r'''
  query Items {
    items(page: 1, pageSize: 100) {
      items {
        id
        name
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

const String recipeQuery = r'''
  query Recipe($id: ID!) {
    recipe(id: $id) {
      id
      name
      description
      servings
      prepTimeMinutes
      cookTimeMinutes
      isFavorite
      items {
        item {
          id
          name
        }
        quantity
        unit
      }
      steps {
        stepNumber
        instruction
      }
    }
  }
''';

const String createRecipeMutation = r'''
  mutation CreateRecipe($input: CreateRecipeInput!) {
    createRecipe(input: $input) {
      id
    }
  }
''';

const String updateRecipeMutation = r'''
  mutation UpdateRecipe($id: ID!, $input: CreateRecipeInput!) {
    updateRecipe(id: $id, input: $input) {
      id
    }
  }
''';

const String setRecipeFavoriteMutation = r'''
  mutation SetRecipeFavorite($recipeId: ID!, $isFavorite: Boolean!) {
    setRecipeFavorite(recipeId: $recipeId, isFavorite: $isFavorite)
  }
''';

class EditRecipeScreen extends StatefulWidget {
  final String? recipeId;

  const EditRecipeScreen({super.key, this.recipeId});

  @override
  State<EditRecipeScreen> createState() => _EditRecipeScreenState();
}

class _EditRecipeScreenState extends State<EditRecipeScreen> {
  final _nameCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final _servingsCtrl = TextEditingController();
  final _prepCtrl = TextEditingController();
  final _cookCtrl = TextEditingController();
  final _qtyCtrl = TextEditingController();
  final _unitCtrl = TextEditingController();
  final _stepCtrl = TextEditingController();

  String? _itemId;
  bool _loaded = false;
  bool _isSaving = false;
  bool _isFavorite = false;
  bool _isTogglingFavorite = false;
  List<Map<String, dynamic>> _items = [];

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _loadData();
    }
  }

  Future<void> _loadData() async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(QueryOptions(document: gql(itemsQuery)));
    setState(() {
      _items = (result.data?['items']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });

    if (widget.recipeId != null) {
      final recipeResult = await client.query(
        QueryOptions(
          document: gql(recipeQuery),
          variables: {'id': widget.recipeId},
        ),
      );
      final recipe = recipeResult.data?['recipe'] as Map<String, dynamic>?;
      if (recipe != null) {
        final item = (recipe['items'] as List?)?[0] as Map<String, dynamic>?;
        final step = (recipe['steps'] as List?)?[0] as Map<String, dynamic>?;
        setState(() {
          _nameCtrl.text = (recipe['name'] as String?) ?? '';
          _descCtrl.text = (recipe['description'] as String?) ?? '';
          _servingsCtrl.text = recipe['servings']?.toString() ?? '';
          _prepCtrl.text = recipe['prepTimeMinutes']?.toString() ?? '';
          _cookCtrl.text = recipe['cookTimeMinutes']?.toString() ?? '';
          _itemId = item?['item']?['id'] as String?;
          _qtyCtrl.text = item?['quantity']?.toString() ?? '';
          _unitCtrl.text = (item?['unit'] as String?) ?? '';
          _stepCtrl.text = (step?['instruction'] as String?) ?? '';
          _isFavorite = (recipe['isFavorite'] as bool?) ?? false;
        });
      }
    }
  }

  Future<void> _toggleFavorite() async {
    if (widget.recipeId == null) return;
    setState(() => _isTogglingFavorite = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final next = !_isFavorite;
      await client.mutate(MutationOptions(
        document: gql(setRecipeFavoriteMutation),
        variables: {'recipeId': widget.recipeId, 'isFavorite': next},
      ));
      if (mounted) setState(() => _isFavorite = next);
    } finally {
      if (mounted) setState(() => _isTogglingFavorite = false);
    }
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _descCtrl.dispose();
    _servingsCtrl.dispose();
    _prepCtrl.dispose();
    _cookCtrl.dispose();
    _qtyCtrl.dispose();
    _unitCtrl.dispose();
    _stepCtrl.dispose();
    super.dispose();
  }

  Future<void> _save(BuildContext context) async {
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final input = <String, dynamic>{
        'name': _nameCtrl.text,
        'description': _descCtrl.text.isEmpty ? null : _descCtrl.text,
        'servings': _servingsCtrl.text.isEmpty ? null : int.tryParse(_servingsCtrl.text),
        'prepTimeMinutes': _prepCtrl.text.isEmpty ? null : int.tryParse(_prepCtrl.text),
        'cookTimeMinutes': _cookCtrl.text.isEmpty ? null : int.tryParse(_cookCtrl.text),
        'items': [
          {
            'itemId': _itemId,
            'quantity': double.tryParse(_qtyCtrl.text) ?? 0,
            'unit': _unitCtrl.text,
            'notes': null,
            'isOptional': false,
          },
        ],
        'steps': [
          {
            'stepNumber': 1,
            'instruction': _stepCtrl.text,
          },
        ],
      };
      if (widget.recipeId == null) {
        await client.mutate(MutationOptions(
          document: gql(createRecipeMutation),
          variables: {'input': input},
        ));
      } else {
        await client.mutate(MutationOptions(
          document: gql(updateRecipeMutation),
          variables: {'id': widget.recipeId, 'input': input},
        ));
      }
      if (mounted) Navigator.pop(context);
    } finally {
      setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_items.isEmpty) {
      return Scaffold(
        appBar: AppBar(
          title: Text(widget.recipeId == null ? 'Create Recipe' : 'Edit Recipe'),
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.recipeId == null ? 'Create Recipe' : 'Edit Recipe'),
        actions: widget.recipeId == null
            ? null
            : [
                IconButton(
                  icon: Icon(_isFavorite ? Icons.star : Icons.star_border),
                  onPressed: _isTogglingFavorite ? null : _toggleFavorite,
                ),
              ],
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            TextField(
              controller: _nameCtrl,
              decoration: const InputDecoration(labelText: 'Name'),
            ),
            TextField(
              controller: _descCtrl,
              decoration: const InputDecoration(labelText: 'Description'),
            ),
            TextField(
              controller: _servingsCtrl,
              decoration: const InputDecoration(labelText: 'Servings'),
              keyboardType: TextInputType.number,
            ),
            TextField(
              controller: _prepCtrl,
              decoration: const InputDecoration(labelText: 'Prep minutes'),
              keyboardType: TextInputType.number,
            ),
            TextField(
              controller: _cookCtrl,
              decoration: const InputDecoration(labelText: 'Cook minutes'),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 16),
            const Text('Ingredient', style: TextStyle(fontWeight: FontWeight.bold)),
            DropdownButtonFormField<String?>(
              value: _itemId,
              decoration: const InputDecoration(labelText: 'Item'),
              items: _items
                  .map((i) => DropdownMenuItem(
                        value: i['id'] as String,
                        child: Text(i['name'] as String),
                      ))
                  .toList(),
              onChanged: (v) => setState(() => _itemId = v),
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
            const SizedBox(height: 16),
            const Text('Step 1', style: TextStyle(fontWeight: FontWeight.bold)),
            TextField(
              controller: _stepCtrl,
              decoration: const InputDecoration(labelText: 'Instruction'),
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
