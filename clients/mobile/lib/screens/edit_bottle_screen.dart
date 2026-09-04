import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String typesQuery = r'''
  query Types {
    types {
      id
      name
    }
  }
''';

const String countriesQuery = r'''
  query Countries {
    countries {
      id
      name
    }
  }
''';

const String regionsQuery = r'''
  query Regions($countryId: ID!) {
    regions(countryId: $countryId) {
      id
      name
    }
  }
''';

const String bottleQuery = r'''
  query Bottle($id: ID!) {
    bottle(id: $id) {
      id
      typeId
      countryId
      regionId
      vineyard
      vintageYear
      bottleSize
      abv
      acidity
      tanninLevel
      body
      sweetness
      oakIntegration
    }
  }
''';

const String createBottleMutation = r'''
  mutation CreateBottle($input: CreateBottleInput!) {
    createBottle(input: $input) {
      id
    }
  }
''';

const String updateBottleMutation = r'''
  mutation UpdateBottle($id: ID!, $input: UpdateBottleInput!) {
    updateBottle(id: $id, input: $input) {
      id
    }
  }
''';

class EditBottleScreen extends StatefulWidget {
  final String? bottleId;

  const EditBottleScreen({super.key, this.bottleId});

  @override
  State<EditBottleScreen> createState() => _EditBottleScreenState();
}

class _EditBottleScreenState extends State<EditBottleScreen> {
  final _vintageCtrl = TextEditingController();
  final _bottleSizeCtrl = TextEditingController(text: '750ml');
  final _vineyardCtrl = TextEditingController();
  final _abvCtrl = TextEditingController();
  final _acidityCtrl = TextEditingController();
  final _tanninCtrl = TextEditingController();
  final _bodyCtrl = TextEditingController();
  final _sweetnessCtrl = TextEditingController();

  String? _typeId;
  String? _countryId;
  String? _regionId;
  bool _oakIntegration = false;
  bool _isSaving = false;
  bool _loaded = false;
  List<Map<String, dynamic>> _types = [];
  List<Map<String, dynamic>> _countries = [];
  List<Map<String, dynamic>> _regions = [];

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
    final typesResult = await client.query(QueryOptions(document: gql(typesQuery)));
    final countriesResult = await client.query(QueryOptions(document: gql(countriesQuery)));

    setState(() {
      _types = (typesResult.data?['types'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      _countries = (countriesResult.data?['countries'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });

    if (widget.bottleId != null) {
      final bottleResult = await client.query(
        QueryOptions(
          document: gql(bottleQuery),
          variables: {'id': widget.bottleId},
        ),
      );
      final bottle = bottleResult.data?['bottle'] as Map<String, dynamic>?;
      if (bottle != null) {
        setState(() {
          _typeId = bottle['typeId'] as String?;
          _countryId = bottle['countryId'] as String?;
          _regionId = bottle['regionId'] as String?;
          _vintageCtrl.text = bottle['vintageYear']?.toString() ?? '';
          _bottleSizeCtrl.text = (bottle['bottleSize'] as String?) ?? '750ml';
          _vineyardCtrl.text = (bottle['vineyard'] as String?) ?? '';
          _abvCtrl.text = bottle['abv']?.toString() ?? '';
          _acidityCtrl.text = bottle['acidity']?.toString() ?? '';
          _tanninCtrl.text = bottle['tanninLevel']?.toString() ?? '';
          _bodyCtrl.text = bottle['body']?.toString() ?? '';
          _sweetnessCtrl.text = bottle['sweetness']?.toString() ?? '';
          _oakIntegration = (bottle['oakIntegration'] as bool?) ?? false;
        });
        await _loadRegions();
      }
    }
  }

  Future<void> _loadRegions() async {
    if (_countryId == null || _countryId!.isEmpty) {
      setState(() => _regions = []);
      return;
    }
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(
      QueryOptions(
        document: gql(regionsQuery),
        variables: {'countryId': _countryId},
      ),
    );
    setState(() {
      _regions = (result.data?['regions'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });
  }

  @override
  void dispose() {
    _vintageCtrl.dispose();
    _bottleSizeCtrl.dispose();
    _vineyardCtrl.dispose();
    _abvCtrl.dispose();
    _acidityCtrl.dispose();
    _tanninCtrl.dispose();
    _bodyCtrl.dispose();
    _sweetnessCtrl.dispose();
    super.dispose();
  }

  Future<void> _save(BuildContext context) async {
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final input = <String, dynamic>{
        'typeId': _typeId,
        'countryId': _countryId,
        'regionId': _regionId,
        'vintageYear': int.tryParse(_vintageCtrl.text),
        'bottleSize': _bottleSizeCtrl.text,
        'vineyard': _vineyardCtrl.text.isEmpty ? null : _vineyardCtrl.text,
        'abv': _abvCtrl.text.isEmpty ? null : double.tryParse(_abvCtrl.text),
        'acidity': _acidityCtrl.text.isEmpty ? null : int.tryParse(_acidityCtrl.text),
        'tanninLevel': _tanninCtrl.text.isEmpty ? null : int.tryParse(_tanninCtrl.text),
        'body': _bodyCtrl.text.isEmpty ? null : int.tryParse(_bodyCtrl.text),
        'sweetness': _sweetnessCtrl.text.isEmpty ? null : int.tryParse(_sweetnessCtrl.text),
        'oakIntegration': _oakIntegration,
      };
      if (widget.bottleId == null) {
        await client.mutate(MutationOptions(
          document: gql(createBottleMutation),
          variables: {'input': input},
        ));
      } else {
        await client.mutate(MutationOptions(
          document: gql(updateBottleMutation),
          variables: {'id': widget.bottleId, 'input': input},
        ));
      }
      if (mounted) Navigator.pop(context);
    } finally {
      setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_types.isEmpty || _countries.isEmpty) {
      return Scaffold(
        appBar: AppBar(
          title: Text(widget.bottleId == null ? 'Create Bottle' : 'Edit Bottle'),
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.bottleId == null ? 'Create Bottle' : 'Edit Bottle'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            DropdownButtonFormField<String?>(
              value: _typeId,
              decoration: const InputDecoration(labelText: 'Type'),
              items: _types
                  .map((t) => DropdownMenuItem(
                        value: t['id'] as String,
                        child: Text(t['name'] as String),
                      ))
                  .toList(),
              onChanged: (v) => setState(() => _typeId = v),
            ),
            DropdownButtonFormField<String?>(
              value: _countryId,
              decoration: const InputDecoration(labelText: 'Country'),
              items: _countries
                  .map((c) => DropdownMenuItem(
                        value: c['id'] as String,
                        child: Text(c['name'] as String),
                      ))
                  .toList(),
              onChanged: (v) async {
                setState(() {
                  _countryId = v;
                  _regionId = null;
                });
                await _loadRegions();
              },
            ),
            DropdownButtonFormField<String?>(
              value: _regionId,
              decoration: const InputDecoration(labelText: 'Region'),
              items: _regions
                  .map((r) => DropdownMenuItem(
                        value: r['id'] as String,
                        child: Text(r['name'] as String),
                      ))
                  .toList(),
              onChanged: (v) => setState(() => _regionId = v),
            ),
            TextField(
              controller: _vintageCtrl,
              decoration: const InputDecoration(labelText: 'Vintage year'),
              keyboardType: TextInputType.number,
            ),
            TextField(
              controller: _bottleSizeCtrl,
              decoration: const InputDecoration(labelText: 'Bottle size'),
            ),
            TextField(
              controller: _vineyardCtrl,
              decoration: const InputDecoration(labelText: 'Vineyard'),
            ),
            TextField(
              controller: _abvCtrl,
              decoration: const InputDecoration(labelText: 'ABV'),
              keyboardType: const TextInputType.numberWithOptions(decimal: true),
            ),
            TextField(
              controller: _acidityCtrl,
              decoration: const InputDecoration(labelText: 'Acidity'),
              keyboardType: TextInputType.number,
            ),
            TextField(
              controller: _tanninCtrl,
              decoration: const InputDecoration(labelText: 'Tannin level'),
              keyboardType: TextInputType.number,
            ),
            TextField(
              controller: _bodyCtrl,
              decoration: const InputDecoration(labelText: 'Body'),
              keyboardType: TextInputType.number,
            ),
            TextField(
              controller: _sweetnessCtrl,
              decoration: const InputDecoration(labelText: 'Sweetness'),
              keyboardType: TextInputType.number,
            ),
            CheckboxListTile(
              title: const Text('Oak integration'),
              value: _oakIntegration,
              onChanged: (v) => setState(() => _oakIntegration = v ?? false),
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
