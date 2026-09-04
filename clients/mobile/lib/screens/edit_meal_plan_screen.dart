import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String mealPlanQuery = r'''
  query MealPlan($id: ID!) {
    mealPlan(id: $id) {
      id
      name
      weekStartDate
      isActive
      slots {
        id
        dayOfWeek
        mealType
        servings
        replacementNote
        recipe {
          id
          name
        }
        items {
          id
          item {
            id
            name
          }
          quantity
          unit
          isFromRecipe
        }
      }
    }
  }
''';

const String recipesQuery = r'''
  query Recipes {
    recipes(page: 1, pageSize: 100) {
      items {
        id
        name
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
''';

const String createMealPlanMutation = r'''
  mutation CreateMealPlan($input: CreateMealPlanInput!) {
    createMealPlan(input: $input) {
      id
    }
  }
''';

const String updateMealPlanMutation = r'''
  mutation UpdateMealPlan($id: ID!, $input: CreateMealPlanInput!) {
    updateMealPlan(id: $id, input: $input) {
      id
    }
  }
''';

const String addMealSlotMutation = r'''
  mutation AddMealSlot($input: AddMealSlotInput!) {
    addMealSlot(input: $input) {
      id
    }
  }
''';

const String removeMealSlotMutation = r'''
  mutation RemoveMealSlot($slotId: ID!) {
    removeMealSlot(slotId: $slotId)
  }
''';

const String addMealSlotItemMutation = r'''
  mutation AddMealSlotItem($input: AddMealSlotItemInput!) {
    addMealSlotItem(input: $input) {
      id
    }
  }
''';

const String removeMealSlotItemMutation = r'''
  mutation RemoveMealSlotItem($slotItemId: ID!) {
    removeMealSlotItem(slotItemId: $slotItemId)
  }
''';

class EditMealPlanScreen extends StatefulWidget {
  final String? mealPlanId;

  const EditMealPlanScreen({super.key, this.mealPlanId});

  @override
  State<EditMealPlanScreen> createState() => _EditMealPlanScreenState();
}

class _EditMealPlanScreenState extends State<EditMealPlanScreen> {
  final _nameCtrl = TextEditingController();
  final _dateCtrl = TextEditingController();
  final _dayCtrl = TextEditingController(text: '0');
  final _dayCtrlNew = TextEditingController(text: '0');
  final _mealTypeCtrl = TextEditingController();
  final _servingsCtrl = TextEditingController();
  final _noteCtrl = TextEditingController();

  bool _isSaving = false;
  bool _isAddingSlot = false;
  bool _loaded = false;
  List<Map<String, dynamic>> _recipes = [];
  List<Map<String, dynamic>> _items = [];
  Map<String, dynamic>? _plan;

  Map<String, TextEditingController> _itemQtyCtrls = {};
  Map<String, TextEditingController> _itemUnitCtrls = {};
  Map<String, String?> _itemSelections = {};

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
    final recipesResult = await client.query(QueryOptions(document: gql(recipesQuery)));
    final itemsResult = await client.query(QueryOptions(document: gql(itemsQuery)));
    setState(() {
      _recipes = (recipesResult.data?['recipes']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      _items = (itemsResult.data?['items']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
    });

    if (widget.mealPlanId != null) {
      final planResult = await client.query(
        QueryOptions(
          document: gql(mealPlanQuery),
          variables: {'id': widget.mealPlanId},
        ),
      );
      final plan = planResult.data?['mealPlan'] as Map<String, dynamic>?;
      if (plan != null) {
        setState(() {
          _plan = plan;
          _nameCtrl.text = (plan['name'] as String?) ?? '';
          _dateCtrl.text = (plan['weekStartDate'] as String?) ?? '';
          _dayCtrl.text = (plan['weekStartDayOfWeek']?.toString()) ?? '0';
        });
      }
    }
  }

  Future<void> _save(BuildContext context) async {
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final input = <String, dynamic>{
        'name': _nameCtrl.text,
        'weekStartDate': _dateCtrl.text,
        'weekStartDayOfWeek': int.tryParse(_dayCtrl.text) ?? 0,
      };
      if (widget.mealPlanId == null) {
        await client.mutate(MutationOptions(
          document: gql(createMealPlanMutation),
          variables: {'input': input},
        ));
      } else {
        await client.mutate(MutationOptions(
          document: gql(updateMealPlanMutation),
          variables: {'id': widget.mealPlanId, 'input': input},
        ));
      }
      if (mounted) Navigator.pop(context);
    } finally {
      setState(() => _isSaving = false);
    }
  }

  Future<void> _addSlot() async {
    if (widget.mealPlanId == null) return;
    setState(() => _isAddingSlot = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final recipeId = _recipeSelection?.isNotEmpty == true ? _recipeSelection : null;
      await client.mutate(MutationOptions(
        document: gql(addMealSlotMutation),
        variables: {
          'input': {
            'mealPlanId': widget.mealPlanId,
            'dayOfWeek': int.tryParse(_dayCtrlNew.text) ?? 0,
            'mealType': _mealTypeCtrl.text,
            'recipeId': recipeId,
            'servings': _servingsCtrl.text.isEmpty ? null : int.tryParse(_servingsCtrl.text),
            'replacementNote': _noteCtrl.text.isEmpty ? null : _noteCtrl.text,
          }
        },
      ));
      _mealTypeCtrl.clear();
      _servingsCtrl.clear();
      _noteCtrl.clear();
      await _loadData();
    } finally {
      setState(() => _isAddingSlot = false);
    }
  }

  Future<void> _removeSlot(String slotId) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(removeMealSlotMutation),
      variables: {'slotId': slotId},
    ));
    await _loadData();
  }

  Future<void> _addSlotItem(String slotId) async {
    final itemId = _itemSelections[slotId];
    final qty = _itemQtyCtrls[slotId]?.text ?? '';
    if (itemId == null || itemId.isEmpty || qty.isEmpty) return;
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(addMealSlotItemMutation),
      variables: {
        'input': {
          'slotId': slotId,
          'itemId': itemId,
          'quantity': double.tryParse(qty) ?? 0,
          'unit': _itemUnitCtrls[slotId]?.text ?? '',
          'isFromRecipe': false,
        }
      },
    ));
    _itemQtyCtrls[slotId]?.clear();
    _itemUnitCtrls[slotId]?.clear();
    setState(() => _itemSelections[slotId] = null);
    await _loadData();
  }

  Future<void> _removeSlotItem(String slotItemId) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(removeMealSlotItemMutation),
      variables: {'slotItemId': slotItemId},
    ));
    await _loadData();
  }

  String? _recipeSelection;

  @override
  void dispose() {
    _nameCtrl.dispose();
    _dateCtrl.dispose();
    _dayCtrl.dispose();
    _dayCtrlNew.dispose();
    _mealTypeCtrl.dispose();
    _servingsCtrl.dispose();
    _noteCtrl.dispose();
    _itemQtyCtrls.values.forEach((c) => c.dispose());
    _itemUnitCtrls.values.forEach((c) => c.dispose());
    super.dispose();
  }

  Widget _slotCard(Map<String, dynamic> slot) {
    final slotId = slot['id'] as String;
    _itemQtyCtrls.putIfAbsent(slotId, () => TextEditingController());
    _itemUnitCtrls.putIfAbsent(slotId, () => TextEditingController());
    final items = (slot['items'] as List? ?? []).cast<Map<String, dynamic>>();
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 8),
      child: Padding(
        padding: const EdgeInsets.all(12.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Day ${slot['dayOfWeek']} - ${slot['mealType']}',
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.delete),
                  onPressed: () => _removeSlot(slotId),
                ),
              ],
            ),
            if (slot['recipe'] != null) Text('Recipe: ${slot['recipe']['name']}'),
            if (slot['servings'] != null) Text('Servings: ${slot['servings']}'),
            if (slot['replacementNote'] != null && (slot['replacementNote'] as String).isNotEmpty)
              Text('Note: ${slot['replacementNote']}'),
            ...items.map((it) => ListTile(
                  dense: true,
                  title: Text('${it['item']?['name'] ?? 'From recipe'} ${it['quantity']} ${it['unit']}'),
                  trailing: IconButton(
                    icon: const Icon(Icons.delete),
                    onPressed: () => _removeSlotItem(it['id'] as String),
                  ),
                )),
            Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String?>(
                    value: _itemSelections[slotId],
                    decoration: const InputDecoration(labelText: 'Item'),
                    items: _items.map((i) => DropdownMenuItem(
                          value: i['id'] as String,
                          child: Text(i['name'] as String),
                        ))
                        .toList(),
                    onChanged: (v) => setState(() => _itemSelections[slotId] = v),
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 70,
                  child: TextField(
                    controller: _itemQtyCtrls[slotId],
                    decoration: const InputDecoration(labelText: 'Qty'),
                    keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 80,
                  child: TextField(
                    controller: _itemUnitCtrls[slotId],
                    decoration: const InputDecoration(labelText: 'Unit'),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.add),
                  onPressed: () => _addSlotItem(slotId),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final slots = (_plan?['slots'] as List? ?? []).cast<Map<String, dynamic>>();
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.mealPlanId == null ? 'Create Meal Plan' : 'Edit Meal Plan'),
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
              controller: _dateCtrl,
              decoration: const InputDecoration(labelText: 'Week start date (YYYY-MM-DD)'),
            ),
            TextField(
              controller: _dayCtrl,
              decoration: const InputDecoration(labelText: 'Week start day (0-6)'),
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
            if (widget.mealPlanId != null) ...[
              const Divider(height: 32),
              const Text('Add Slot', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 18)),
              TextField(
                controller: _dayCtrlNew,
                decoration: const InputDecoration(labelText: 'Day (0-6)'),
                keyboardType: TextInputType.number,
              ),
              TextField(
                controller: _mealTypeCtrl,
                decoration: const InputDecoration(labelText: 'Meal type'),
              ),
              DropdownButtonFormField<String?>(
                value: _recipeSelection,
                decoration: const InputDecoration(labelText: 'Recipe (optional)'),
                items: [
                  const DropdownMenuItem(value: null, child: Text('None')),
                  ..._recipes.map((r) => DropdownMenuItem(
                        value: r['id'] as String,
                        child: Text(r['name'] as String),
                      ))
                ],
                onChanged: (v) => setState(() => _recipeSelection = v),
              ),
              TextField(
                controller: _servingsCtrl,
                decoration: const InputDecoration(labelText: 'Servings'),
                keyboardType: TextInputType.number,
              ),
              TextField(
                controller: _noteCtrl,
                decoration: const InputDecoration(labelText: 'Replacement note'),
              ),
              ElevatedButton(
                onPressed: _isAddingSlot ? null : _addSlot,
                child: _isAddingSlot
                    ? const SizedBox(
                        height: 16,
                        width: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Add Slot'),
              ),
              const Divider(height: 32),
              const Text('Slots', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 18)),
              ...slots.map((s) => _slotCard(s)),
            ],
          ],
        ),
      ),
    );
  }
}
