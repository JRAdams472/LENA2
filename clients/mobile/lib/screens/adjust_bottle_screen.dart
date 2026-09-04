import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String bottlesQuery = r'''
  query Bottles {
    bottles(page: 1, pageSize: 100) {
      items {
        id
        vineyard
        vintageYear
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

const String adjustUserBottleMutation = r'''
  mutation AdjustUserBottle($bottleId: ID!, $quantity: Int!) {
    adjustUserBottle(bottleId: $bottleId, quantity: $quantity) {
      id
    }
  }
''';

class AdjustBottleScreen extends StatefulWidget {
  final String? bottleId;
  final int? quantity;

  const AdjustBottleScreen({super.key, this.bottleId, this.quantity});

  @override
  State<AdjustBottleScreen> createState() => _AdjustBottleScreenState();
}

class _AdjustBottleScreenState extends State<AdjustBottleScreen> {
  final _quantityCtrl = TextEditingController();
  String? _bottleId;
  bool _loaded = false;
  bool _isSaving = false;
  List<Map<String, dynamic>> _bottles = [];

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
    final result = await client.query(QueryOptions(document: gql(bottlesQuery)));
    setState(() {
      _bottles = (result.data?['bottles']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      _bottleId = widget.bottleId ?? (_bottles.isNotEmpty ? _bottles.first['id'] as String : null);
    });
    if (widget.quantity != null) {
      _quantityCtrl.text = widget.quantity.toString();
    }
  }

  @override
  void dispose() {
    _quantityCtrl.dispose();
    super.dispose();
  }

  Future<void> _save(BuildContext context) async {
    if (_bottleId == null) return;
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      await client.mutate(MutationOptions(
        document: gql(adjustUserBottleMutation),
        variables: {
          'bottleId': _bottleId,
          'quantity': int.tryParse(_quantityCtrl.text) ?? 0,
        },
      ));
      if (mounted) Navigator.pop(context);
    } finally {
      setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_bottles.isEmpty) {
      return Scaffold(
        appBar: AppBar(title: const Text('Adjust Holding')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Adjust Holding')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            DropdownButtonFormField<String?>(
              value: _bottleId,
              decoration: const InputDecoration(labelText: 'Bottle'),
              items: _bottles
                  .map((b) => DropdownMenuItem(
                        value: b['id'] as String,
                        child: Text(
                          '${(b['vineyard'] as String?) ?? 'Unknown'} ${b['vintageYear']?.toString() ?? ''}',
                        ),
                      ))
                  .toList(),
              onChanged: (v) => setState(() => _bottleId = v),
            ),
            TextField(
              controller: _quantityCtrl,
              decoration: const InputDecoration(labelText: 'Quantity'),
              keyboardType: TextInputType.number,
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
