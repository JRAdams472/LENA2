import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/search_picker.dart';

const String bottlePickerQuery = r'''
  query BottlePicker($search: String) {
    bottles(page: 1, pageSize: 50, search: $search) {
      items {
        id
        vineyard
        vintageYear
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

String bottleLabel(Map<String, dynamic> b) =>
    '${(b['vineyard'] as String?) ?? 'Unknown'} ${b['vintageYear']?.toString() ?? ''}'
        .trim();

class AdjustBottleScreen extends StatefulWidget {
  final String? bottleId;
  final String? bottleName;
  final int? quantity;

  const AdjustBottleScreen(
      {super.key, this.bottleId, this.bottleName, this.quantity});

  @override
  State<AdjustBottleScreen> createState() => _AdjustBottleScreenState();
}

class _AdjustBottleScreenState extends State<AdjustBottleScreen> {
  final _quantityCtrl = TextEditingController();
  Map<String, dynamic>? _bottle;
  bool _isSaving = false;

  @override
  void initState() {
    super.initState();
    if (widget.bottleId != null) {
      _bottle = {'id': widget.bottleId, 'vineyard': widget.bottleName};
    }
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
    if (_bottle == null) return;
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      await client.mutate(MutationOptions(
        document: gql(adjustUserBottleMutation),
        variables: {
          'bottleId': _bottle!['id'],
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
    return Scaffold(
      appBar: AppBar(title: const Text('Adjust Holding')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            SearchPickerField(
              label: 'Bottle',
              displayText: _bottle == null ? '' : bottleLabel(_bottle!),
              hintText: 'Select a bottle',
              sheetTitle: 'Search bottles',
              document: bottlePickerQuery,
              connectionField: 'bottles',
              searchEntityType: 'bottle',
              variablesFor: (term) => {'search': term.isEmpty ? null : term},
              itemLabel: bottleLabel,
              onChanged: (b) => setState(() => _bottle = b),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _quantityCtrl,
              decoration: const InputDecoration(labelText: 'Quantity'),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed:
                  (_isSaving || _bottle == null) ? null : () => _save(context),
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
