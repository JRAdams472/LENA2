import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import '../scan/upc_utils.dart';

const String itemByUpcQuery = r'''
  query ItemByUpc($code: String!) {
    itemByUpc(code: $code) {
      id
      name
      brand
      upc12
      upc14
      unit
      category {
        id
        name
      }
      nutrients {
        amount
        nutrient {
          id
          name
          unit
        }
      }
    }
  }
''';

const String incrementUserItemMutation = r'''
  mutation IncrementUserItem($itemId: ID!, $delta: Float!) {
    incrementUserItem(itemId: $itemId, delta: $delta) {
      id
      currentQty
    }
  }
''';

const String submitItemMutation = r'''
  mutation SubmitItem($input: CreateItemInput!) {
    submitItem(input: $input) {
      id
      name
    }
  }
''';

const String setItemNutrientsMutation = r'''
  mutation SetItemNutrients($itemId: ID!, $nutrients: [FoodNutrientInput!]!) {
    setItemNutrients(itemId: $itemId, nutrients: $nutrients) {
      amount
      nutrient { id name unit }
    }
  }
''';

class ScanScreen extends StatefulWidget {
  const ScanScreen({super.key});

  @override
  State<ScanScreen> createState() => _ScanScreenState();
}

class _ScanScreenState extends State<ScanScreen> {
  final _scanner = MobileScannerController(autoStart: true);
  final _qtyCtrl = TextEditingController(text: '1');
  final _nameCtrl = TextEditingController();
  final _unitCtrl = TextEditingController();
  final _categoryCtrl = TextEditingController();
  final List<Map<String, TextEditingController>> _nutrientCtrls = [];

  bool _isLoading = false;
  String? _upc;
  Map<String, dynamic>? _foundItem;
  bool _showSubmit = false;
  String? _message;
  String? _error;

  void _addNutrient() {
    setState(() {
      _nutrientCtrls.add({
        'id': TextEditingController(),
        'amount': TextEditingController(),
      });
    });
  }

  void _removeNutrient(int index) {
    final row = _nutrientCtrls.removeAt(index);
    row['id']?.dispose();
    row['amount']?.dispose();
    setState(() {});
  }

  Future<void> _onDetect(BarcodeCapture capture) async {
    final raw = capture.barcodes.firstOrNull?.rawValue;
    if (raw == null || raw.isEmpty) return;

    final normalized = normalizeUpc(raw);
    if (normalized == null) {
      setState(() {
        _error = 'Unsupported barcode: $raw';
      });
      return;
    }

    await _scanner.stop();
    _lookup(normalized);
  }

  Future<void> _lookup(String code) async {
    setState(() {
      _isLoading = true;
      _error = null;
      _message = null;
    });

    final client = GraphQLProvider.of(context).value;
    final result = await client.query(
      QueryOptions(
        document: gql(itemByUpcQuery),
        variables: {'code': code},
        fetchPolicy: FetchPolicy.networkOnly,
      ),
    );

    if (!mounted) return;

    setState(() {
      _isLoading = false;
      _upc = code;
      if (result.hasException) {
        _error = result.exception.toString();
      } else if (result.data?['itemByUpc'] == null) {
        _showSubmit = true;
        _foundItem = null;
      } else {
        _foundItem = result.data?['itemByUpc'] as Map<String, dynamic>;
        _showSubmit = false;
      }
    });
  }

  Future<void> _adjustInventory(bool add) async {
    if (_foundItem == null) return;
    final qty = double.tryParse(_qtyCtrl.text) ?? 0;
    if (qty <= 0) return;

    final delta = add ? qty : -qty;
    final client = GraphQLProvider.of(context).value;
    final id = _foundItem!['id'] as String;

    setState(() => _isLoading = true);
    final result = await client.mutate(
      MutationOptions(
        document: gql(incrementUserItemMutation),
        variables: {'itemId': id, 'delta': delta},
      ),
    );

    if (!mounted) return;
    setState(() => _isLoading = false);

    if (result.hasException) {
      setState(() => _error = result.exception.toString());
    } else {
      final action = add ? 'added' : 'removed';
      setState(() {
        _message = '${_foundItem!['name']} $action to pantry.';
        _error = null;
      });
      _resetAfterDelay();
    }
  }

  Future<void> _submitItem() async {
    final name = _nameCtrl.text.trim();
    final unit = _unitCtrl.text.trim();
    final category = _categoryCtrl.text.trim();

    if (name.isEmpty || unit.isEmpty || category.isEmpty || _upc == null) {
      setState(() => _error = 'Please fill in name, unit, and category.');
      return;
    }

    final input = <String, dynamic>{
      'name': name,
      'unit': unit,
      'categoryId': category,
      'brandId': null,
    };

    if (_upc!.length == 12) {
      input['upc12'] = _upc;
      input['upc14'] = null;
    } else if (_upc!.length == 14) {
      input['upc12'] = null;
      input['upc14'] = _upc;
    }

    final client = GraphQLProvider.of(context).value;
    setState(() => _isLoading = true);
    final result = await client.mutate(
      MutationOptions(
        document: gql(submitItemMutation),
        variables: {'input': input},
      ),
    );

    if (!mounted) return;
    setState(() => _isLoading = false);

    if (result.hasException) {
      setState(() => _error = result.exception.toString());
      return;
    }

    final itemId = (result.data?['submitItem']?['id'] as String?) ?? '';
    final nutrients = _nutrientCtrls
        .where((row) => row['id']!.text.trim().isNotEmpty && row['amount']!.text.trim().isNotEmpty)
        .map((row) => {
              'nutrientId': row['id']!.text.trim(),
              'amount': double.tryParse(row['amount']!.text.trim()) ?? 0,
            })
        .toList();

    if (nutrients.isNotEmpty && itemId.isNotEmpty) {
      final nutrientResult = await client.mutate(
        MutationOptions(
          document: gql(setItemNutrientsMutation),
          variables: {
            'itemId': itemId,
            'nutrients': nutrients,
          },
        ),
      );

      if (!mounted) return;

      if (nutrientResult.hasException) {
        setState(() => _error = nutrientResult.exception.toString());
        return;
      }
    }

    setState(() {
      _message = '$name submitted for approval.';
      _error = null;
      _showSubmit = false;
    });
    _resetAfterDelay();
  }

  void _clearNutrients() {
    for (final row in _nutrientCtrls) {
      row['id']?.dispose();
      row['amount']?.dispose();
    }
    _nutrientCtrls.clear();
  }

  void _reset() {
    _qtyCtrl.text = '1';
    _nameCtrl.clear();
    _unitCtrl.clear();
    _categoryCtrl.clear();
    _clearNutrients();
    setState(() {
      _foundItem = null;
      _showSubmit = false;
      _upc = null;
      _message = null;
      _error = null;
    });
    _scanner.start();
  }

  void _resetAfterDelay() {
    Future.delayed(const Duration(seconds: 2), _reset);
  }

  @override
  void dispose() {
    _scanner.dispose();
    _qtyCtrl.dispose();
    _nameCtrl.dispose();
    _unitCtrl.dispose();
    _categoryCtrl.dispose();
    _clearNutrients();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Scan Item')),
      body: _body(),
    );
  }

  Widget _body() {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_foundItem != null) {
      return _foundView();
    }
    if (_showSubmit) {
      return _submitView();
    }
    return _scannerView();
  }

  Widget _scannerView() {
    return Stack(
      children: [
        MobileScanner(
          controller: _scanner,
          onDetect: _onDetect,
        ),
        Align(
          alignment: Alignment.bottomCenter,
          child: Padding(
            padding: const EdgeInsets.all(24.0),
            child: Container(
              padding: const EdgeInsets.all(16.0),
              decoration: BoxDecoration(
                color: Colors.black54,
                borderRadius: BorderRadius.circular(8.0),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text(
                    'Center a barcode in the camera view',
                    style: TextStyle(color: Colors.white),
                  ),
                  if (_error != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8.0),
                      child: Text(
                        _error!,
                        style: const TextStyle(color: Colors.red),
                      ),
                    ),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _foundView() {
    final item = _foundItem!;
    final name = item['name'] as String? ?? 'Unknown';
    final brand = item['brand'] as String?;
    final unit = item['unit'] as String? ?? '';
    final upc = item['upc12'] as String? ?? item['upc14'] as String? ?? _upc ?? '';

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(name, style: Theme.of(context).textTheme.headlineSmall),
          if (brand != null && brand.isNotEmpty)
            Text('Brand: $brand'),
          Text('Unit: $unit'),
          Text('UPC: $upc'),
          const SizedBox(height: 24),
          TextField(
            controller: _qtyCtrl,
            decoration: const InputDecoration(labelText: 'Quantity'),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: FilledButton(
                  onPressed: () => _adjustInventory(true),
                  child: const Text('Add to Pantry'),
                ),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: FilledButton.tonal(
                  onPressed: () => _adjustInventory(false),
                  child: const Text('Remove from Pantry'),
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          Center(
            child: TextButton(
              onPressed: _reset,
              child: const Text('Scan another'),
            ),
          ),
          if (_message != null)
            Padding(
              padding: const EdgeInsets.only(top: 16.0),
              child: Center(
                child: Text(
                  _message!,
                  style: TextStyle(color: Theme.of(context).colorScheme.primary),
                ),
              ),
            ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(top: 16.0),
              child: Center(
                child: Text(
                  _error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _submitView() {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Item not found',
            style: Theme.of(context).textTheme.headlineSmall,
          ),
          Text('UPC: $_upc'),
          const SizedBox(height: 16),
          const Text(
            'Submit this item for admin approval. Add optional nutrients below.',
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _nameCtrl,
            decoration: const InputDecoration(labelText: 'Item name'),
          ),
          TextField(
            controller: _unitCtrl,
            decoration: const InputDecoration(labelText: 'Unit (e.g., oz, lb)'),
          ),
          TextField(
            controller: _categoryCtrl,
            decoration: const InputDecoration(
              labelText: 'Category ID',
              helperText: 'Use an existing category ID from the catalog.',
            ),
            keyboardType: TextInputType.number,
          ),
          const SizedBox(height: 24),
          Text('Nutrients (optional)', style: Theme.of(context).textTheme.titleMedium),
          ..._nutrientCtrls.asMap().entries.map((entry) {
            final index = entry.key;
            final row = entry.value;
            return Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: row['id'],
                    decoration: const InputDecoration(
                      labelText: 'Nutrient ID',
                    ),
                    keyboardType: TextInputType.number,
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: TextField(
                    controller: row['amount'],
                    decoration: const InputDecoration(
                      labelText: 'Amount',
                    ),
                    keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.delete),
                  onPressed: () => _removeNutrient(index),
                ),
              ],
            );
          }),
          TextButton.icon(
            onPressed: _addNutrient,
            icon: const Icon(Icons.add),
            label: const Text('Add nutrient'),
          ),
          const SizedBox(height: 24),
          Row(
            children: [
              Expanded(
                child: FilledButton(
                  onPressed: _submitItem,
                  child: const Text('Submit for approval'),
                ),
              ),
              const SizedBox(width: 16),
              TextButton(
                onPressed: _reset,
                child: const Text('Cancel'),
              ),
            ],
          ),
          if (_message != null)
            Padding(
              padding: const EdgeInsets.only(top: 16.0),
              child: Center(
                child: Text(
                  _message!,
                  style: TextStyle(color: Theme.of(context).colorScheme.primary),
                ),
              ),
            ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(top: 16.0),
              child: Center(
                child: Text(
                  _error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
