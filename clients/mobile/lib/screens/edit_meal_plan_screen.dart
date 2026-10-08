import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/recipe_picker.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';
import '../allergy.dart';
import '../format.dart';

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
          ingredient {
            id
            name
          }
          quantity
          unit
          isFromRecipe
        }
        allergyWarnings {
          memberKind
          entityKind
          member { id displayName firstName lastName }
          allergen { id name }
        }
        allergens {
          kind
          allergen { id name }
        }
      }
    }
  }
''';

const String recipeCategoryGroupsQuery = r'''
  query RecipeCategoryGroups {
    recipeCategoryGroups {
      id
      name
      categories {
        id
        name
      }
    }
  }
''';

const String itemsQuery = r'''
  query Items($search: String) {
    items(page: 1, pageSize: 50, search: $search) {
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
  int _weekStartDay = 0;
  int _slotDay = 0;
  final _mealTypeCtrl = TextEditingController();
  final _servingsCtrl = TextEditingController();
  final _noteCtrl = TextEditingController();

  bool _isSaving = false;
  bool _isAddingSlot = false;
  bool _loaded = false;
  List<Map<String, dynamic>> _items = [];
  List<Map<String, dynamic>> _categoryGroups = [];
  String? _categoryFilter;
  Map<String, dynamic>? _selectedRecipe;
  Map<String, dynamic>? _plan;

  Map<String, TextEditingController> _itemQtyCtrls = {};
  Map<String, TextEditingController> _itemUnitCtrls = {};
  Map<String, TextEditingController> _itemSearchCtrls = {};
  Map<String, String?> _itemSelections = {};
  final _itemSearchDebouncer = Debouncer();

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _loadData();
    }
  }

  Future<void> _loadItems() async {
    final client = GraphQLProvider.of(context).value;
    final itemsResult = await client.query(QueryOptions(
      document: gql(itemsQuery),
      variables: {'search': _itemSearch.isEmpty ? null : _itemSearch},
    ));
    if (!mounted) return;
    setState(() {
      _mergeItems(itemsResult.data?['items']?['items'] as List? ?? []);
    });
  }

  Future<void> _loadData() async {
    final client = GraphQLProvider.of(context).value;
    final groupsResult = await client
        .query(QueryOptions(document: gql(recipeCategoryGroupsQuery)));
    if (!mounted) return;
    setState(() {
      _categoryGroups =
          (groupsResult.data?['recipeCategoryGroups'] as List? ?? [])
              .cast<Map<String, dynamic>>();
    });
    await _loadItems();

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
          _weekStartDay = (plan['weekStartDayOfWeek'] as num?)?.toInt() ?? 0;
        });
      }
    }
  }

  String _itemSearch = '';

  /// Loads ranked catalog items into `_items`, keeping every currently
  /// selected slot item in the list — a search result set that omits a
  /// selected id would trip DropdownButtonFormField's value assertion.
  void _mergeItems(List<dynamic> loaded) {
    final merged = loaded.cast<Map<String, dynamic>>();
    final selectedIds = _itemSelections.values.whereType<String>().toSet();
    for (final id in selectedIds) {
      if (!merged.any((i) => i['id'] == id)) {
        final prev = _items.where((i) => i['id'] == id).toList();
        if (prev.isNotEmpty) merged.insert(0, prev.first);
      }
    }
    _items = merged;
  }

  void _onItemSearchChanged(String value) {
    _itemSearchDebouncer.run(() {
      final term = value.trim();
      if (term.isNotEmpty) {
        recordSearch(GraphQLProvider.of(context).value, 'item', term);
      }
      setState(() => _itemSearch = term);
      _loadItems();
    });
  }

  Future<void> _save(BuildContext context) async {
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final input = <String, dynamic>{
        'name': _nameCtrl.text,
        'weekStartDate': _dateCtrl.text,
        'weekStartDayOfWeek': _weekStartDay,
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
      final recipeId = _selectedRecipe?['id'] as String?;
      await client.mutate(MutationOptions(
        document: gql(addMealSlotMutation),
        variables: {
          'input': {
            'mealPlanId': widget.mealPlanId,
            'dayOfWeek': _slotDay,
            'mealType': _mealTypeCtrl.text,
            'recipeId': recipeId,
            'servings': _servingsCtrl.text.isEmpty
                ? null
                : int.tryParse(_servingsCtrl.text),
            'replacementNote': _noteCtrl.text.isEmpty ? null : _noteCtrl.text,
          }
        },
      ));
      if (recipeId != null) {
        recordSelection(client, 'recipe', recipeId);
      }
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
    recordSelection(client, 'item', itemId);
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

  @override
  void dispose() {
    _nameCtrl.dispose();
    _dateCtrl.dispose();

    _mealTypeCtrl.dispose();
    _servingsCtrl.dispose();
    _noteCtrl.dispose();
    _itemQtyCtrls.values.forEach((c) => c.dispose());
    _itemUnitCtrls.values.forEach((c) => c.dispose());
    _itemSearchCtrls.values.forEach((c) => c.dispose());
    _itemSearchDebouncer.dispose();
    super.dispose();
  }

  Widget _slotCard(Map<String, dynamic> slot) {
    final slotId = slot['id'] as String;
    _itemQtyCtrls.putIfAbsent(slotId, () => TextEditingController());
    _itemUnitCtrls.putIfAbsent(slotId, () => TextEditingController());
    _itemSearchCtrls.putIfAbsent(slotId, () => TextEditingController());
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
                    '${weekdayName(slot['dayOfWeek'] as int?)} — ${slot['mealType']}',
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.delete),
                  onPressed: () => _removeSlot(slotId),
                ),
              ],
            ),
            if (slot['recipe'] != null)
              Row(
                children: [
                  Expanded(child: Text('Recipe: ${slot['recipe']['name']}')),
                  AllergyWarningBadge(warnings: allergyWarningsOf(slot)),
                ],
              ),
            if (slot['servings'] != null) Text('Servings: ${slot['servings']}'),
            if (slot['replacementNote'] != null &&
                (slot['replacementNote'] as String).isNotEmpty)
              Text('Note: ${slot['replacementNote']}'),
            ...items.map((it) => ListTile(
                  dense: true,
                  title: Text(
                      '${it['item']?['name'] ?? it['ingredient']?['name'] ?? 'From recipe'} ${it['quantity']} ${it['unit']}'),
                  trailing: IconButton(
                    icon: const Icon(Icons.delete),
                    onPressed: () => _removeSlotItem(it['id'] as String),
                  ),
                )),
            TextField(
              controller: _itemSearchCtrls[slotId],
              decoration: const InputDecoration(
                labelText: 'Search items',
                prefixIcon: Icon(Icons.search),
                isDense: true,
              ),
              onChanged: _onItemSearchChanged,
            ),
            Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String?>(
                    isExpanded: true,
                    value: _itemSelections[slotId],
                    decoration: const InputDecoration(labelText: 'Item'),
                    items: _items
                        .map((i) => DropdownMenuItem(
                              value: i['id'] as String,
                              child: Text(i['name'] as String),
                            ))
                        .toList(),
                    onChanged: (v) =>
                        setState(() => _itemSelections[slotId] = v),
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 70,
                  child: TextField(
                    controller: _itemQtyCtrls[slotId],
                    decoration: const InputDecoration(labelText: 'Qty'),
                    keyboardType:
                        const TextInputType.numberWithOptions(decimal: true),
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
        title: Text(
            widget.mealPlanId == null ? 'Create Meal Plan' : 'Edit Meal Plan'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            TextField(
              controller: _nameCtrl,
              decoration: const InputDecoration(labelText: 'Name'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _dateCtrl,
              decoration: const InputDecoration(
                  labelText: 'Week start date (YYYY-MM-DD)'),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<int>(
              isExpanded: true,
              initialValue: _weekStartDay,
              decoration: const InputDecoration(
                floatingLabelBehavior: FloatingLabelBehavior.always,
                labelText: 'Week starts on',
              ),
              items: [
                for (var d = 0; d < 7; d++)
                  DropdownMenuItem(value: d, child: Text(weekdayName(d))),
              ],
              onChanged: (v) => setState(() => _weekStartDay = v ?? 0),
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
              Text('Add Slot', style: Theme.of(context).textTheme.titleMedium),
              DropdownButtonFormField<int>(
                isExpanded: true,
                initialValue: _slotDay,
                decoration: const InputDecoration(
                  floatingLabelBehavior: FloatingLabelBehavior.always,
                  labelText: 'Day',
                ),
                items: [
                  for (var d = 0; d < 7; d++)
                    DropdownMenuItem(value: d, child: Text(weekdayName(d))),
                ],
                onChanged: (v) => setState(() => _slotDay = v ?? 0),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _mealTypeCtrl,
                decoration: const InputDecoration(labelText: 'Meal type'),
                // Rebuild so RecipePickerField's mealType prop stays current.
                onChanged: (_) => setState(() {}),
              ),
              const SizedBox(height: 12),
              if (_categoryGroups.isNotEmpty)
                DropdownButtonFormField<String?>(
                  isExpanded: true,
                  value: _categoryFilter,
                  decoration: const InputDecoration(
                      labelText: 'Filter recipes by category'),
                  items: [
                    const DropdownMenuItem(
                        value: null, child: Text('All categories')),
                    for (final group in _categoryGroups)
                      for (final cat in (group['categories'] as List? ?? []))
                        DropdownMenuItem(
                          value: cat['id'] as String,
                          child: Text('${group['name']}: ${cat['name']}'),
                        ),
                  ],
                  onChanged: (v) => setState(() => _categoryFilter = v),
                ),
              const SizedBox(height: 12),
              // Server-side searchable picker — the category filter feeds
              // categoryIds so the narrowed set is still reachable.
              RecipePickerField(
                selected: _selectedRecipe,
                mealType: _mealTypeCtrl.text,
                categoryId: _categoryFilter,
                label: 'Recipe (optional)',
                onChanged: (r) => setState(() => _selectedRecipe = r),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _servingsCtrl,
                decoration: const InputDecoration(labelText: 'Servings'),
                keyboardType: TextInputType.number,
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _noteCtrl,
                decoration:
                    const InputDecoration(labelText: 'Replacement note'),
              ),
              const SizedBox(height: 12),
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
              Text('Slots', style: Theme.of(context).textTheme.titleMedium),
              ...slots.map((s) => _slotCard(s)),
            ],
          ],
        ),
      ),
    );
  }
}
