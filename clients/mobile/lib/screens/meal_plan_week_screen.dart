import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../allergy.dart';
import '../analytics/analytics.dart';
import '../format.dart';
import '../widgets/recipe_picker.dart';
import '../widgets/skeleton.dart';
import 'edit_meal_plan_screen.dart';

const String suggestMealsQuery = r'''
  query SuggestMeals($mealPlanId: ID!, $maxSuggestions: Int) {
    suggestMeals(mealPlanId: $mealPlanId, maxSuggestions: $maxSuggestions) {
      recipe { id name }
      dayOfWeek
      mealType
      reason
      usesExpiringItems
    }
  }
''';

const String nutritionQuery = r'''
  query MealPlanNutrition($mealPlanId: ID!) {
    nutrition(mealPlanId: $mealPlanId) {
      entries { name unit amount }
      warnings
    }
  }
''';

/// Canonical meal types shared with the web client (`MEAL_TYPES`). Slots
/// with any other value (legacy free-text) render under "Other".
const List<String> kMealTypes = ['Breakfast', 'Lunch', 'Dinner'];
const String kOtherMealType = 'Other';

String _mealRow(String? mealType) {
  final t = mealType?.trim().toLowerCase();
  for (final k in kMealTypes) {
    if (k.toLowerCase() == t) return k;
  }
  return kOtherMealType;
}

/// Week-oriented meal-plan view — the meal-plan parity piece of LEN-14.
/// Wide layouts get a 7-column day grid; narrow layouts get vertically
/// scrolling day sections. Add/edit/remove reuse the same mutations the
/// edit screen uses; "edit" is remove+add (the schema has no update —
/// the web client composes the same pair).
class MealPlanWeekScreen extends StatefulWidget {
  final String mealPlanId;

  const MealPlanWeekScreen({super.key, required this.mealPlanId});

  @override
  State<MealPlanWeekScreen> createState() => _MealPlanWeekScreenState();
}

class _MealPlanWeekScreenState extends State<MealPlanWeekScreen> {
  Map<String, dynamic>? _plan;
  bool _loading = true;
  bool _suggesting = false;
  Object? _error;
  bool _loaded = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _load();
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final client = GraphQLProvider.of(context).value;
      final result = await client.query(QueryOptions(
        document: gql(mealPlanQuery),
        variables: {'id': widget.mealPlanId},
        fetchPolicy: FetchPolicy.networkOnly,
      ));
      if (!mounted) return;
      setState(() {
        _plan = result.data?['mealPlan'] as Map<String, dynamic>?;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  List<Map<String, dynamic>> get _slots =>
      (_plan?['slots'] as List? ?? []).cast<Map<String, dynamic>>();

  /// Days in display order — the plan's week-start day first.
  List<int> get _orderedDays {
    final start = (_plan?['weekStartDayOfWeek'] as num?)?.toInt() ?? 0;
    return [for (var i = 0; i < 7; i++) (start + i) % 7];
  }

  List<Map<String, dynamic>> _slotsFor(int day, String row) => _slots
      .where((s) =>
          (s['dayOfWeek'] as num?)?.toInt() == day &&
          _mealRow(s['mealType'] as String?) == row)
      .toList();

  List<String> get _rows {
    final rows = List<String>.of(kMealTypes);
    if (_slots
        .any((s) => _mealRow(s['mealType'] as String?) == kOtherMealType)) {
      rows.add(kOtherMealType);
    }
    return rows;
  }

  Future<void> _addSlot(Map<String, dynamic> input) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(addMealSlotMutation),
      variables: {
        'input': {'mealPlanId': widget.mealPlanId, ...input}
      },
    ));
    final recipeId = input['recipeId'] as String?;
    if (recipeId != null) recordSelection(client, 'recipe', recipeId);
    await _load();
  }

  Future<void> _removeSlot(String slotId) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(removeMealSlotMutation),
      variables: {'slotId': slotId},
    ));
    await _load();
  }

  /// No updateMealSlot mutation exists — the web client's updateSlot is a
  /// remove+add pair, and this does the same.
  Future<void> _editSlot(
      Map<String, dynamic> slot, Map<String, dynamic> input) async {
    await _removeSlot(slot['id'] as String);
    await _addSlot(input);
  }

  void _openSlotDialog({int? day, String? row, Map<String, dynamic>? slot}) {
    showDialog(
      context: context,
      builder: (ctx) => _SlotDialog(
        day:
            slot != null ? (slot['dayOfWeek'] as num?)?.toInt() ?? 0 : day ?? 0,
        mealType: slot != null
            ? slot['mealType'] as String?
            : (row == kOtherMealType ? null : row),
        recipe: slot?['recipe'] as Map<String, dynamic>?,
        servings: (slot?['servings'] as num?)?.toInt(),
        note: slot?['replacementNote'] as String?,
        title: slot == null ? 'Add meal' : 'Edit meal',
        onSave: (input) async {
          if (slot == null) {
            await _addSlot(input);
          } else {
            await _editSlot(slot, input);
          }
        },
      ),
    );
  }

  void _openSlotDetail(Map<String, dynamic> slot) {
    final items = (slot['items'] as List? ?? []).cast<Map<String, dynamic>>();
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(
            '${weekdayName((slot['dayOfWeek'] as num?)?.toInt())} — ${slot['mealType']}'),
        content: SizedBox(
          width: 360,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (slot['recipe'] != null)
                Text('Recipe: ${slot['recipe']['name']}'),
              if (slot['servings'] != null)
                Text('Servings: ${slot['servings']}'),
              if (slot['replacementNote'] != null &&
                  (slot['replacementNote'] as String).isNotEmpty)
                Text('Note: ${slot['replacementNote']}'),
              ...items.map((it) => Text(
                  '• ${it['item']?['name'] ?? it['ingredient']?['name'] ?? 'From recipe'} '
                  '${it['quantity']} ${it['unit']}')),
              if (items.isEmpty && slot['recipe'] == null)
                const Text('Empty slot'),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _openSlotDialog(slot: slot);
            },
            child: const Text('Edit'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _removeSlot(slot['id'] as String);
            },
            child: const Text('Remove'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Close'),
          ),
        ],
      ),
    );
  }

  Future<void> _suggest() async {
    setState(() => _suggesting = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final result = await client.query(QueryOptions(
        document: gql(suggestMealsQuery),
        variables: {'mealPlanId': widget.mealPlanId, 'maxSuggestions': 6},
        fetchPolicy: FetchPolicy.networkOnly,
      ));
      if (!mounted) return;
      final suggestions = (result.data?['suggestMeals'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      if (result.hasException) throw result.exception!;
      setState(() => _suggesting = false);
      await showModalBottomSheet(
        context: context,
        isScrollControlled: true,
        builder: (ctx) => _SuggestionSheet(
          suggestions: suggestions,
          onAdd: (s) async {
            await _addSlot({
              'dayOfWeek': (s['dayOfWeek'] as num).toInt(),
              'mealType': s['mealType'],
              'recipeId': (s['recipe'] as Map?)?['id'],
              'servings': null,
              'replacementNote': null,
            });
            if (ctx.mounted) Navigator.pop(ctx);
          },
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Suggestions unavailable: $e')));
      }
    } finally {
      if (mounted) setState(() => _suggesting = false);
    }
  }

  Future<void> _showNutrition() async {
    final client = GraphQLProvider.of(context).value;
    try {
      final result = await client.query(QueryOptions(
        document: gql(nutritionQuery),
        variables: {'mealPlanId': widget.mealPlanId},
        fetchPolicy: FetchPolicy.networkOnly,
      ));
      if (!mounted) return;
      if (result.hasException) throw result.exception!;
      final nutrition = result.data?['nutrition'] as Map<String, dynamic>? ??
          const {'entries': [], 'warnings': []};
      final entries =
          (nutrition['entries'] as List? ?? []).cast<Map<String, dynamic>>();
      final warnings = (nutrition['warnings'] as List? ?? []).cast<String>();
      await showModalBottomSheet(
        context: context,
        builder: (ctx) => SafeArea(
          child: ListView(
            shrinkWrap: true,
            padding: const EdgeInsets.all(16),
            children: [
              Text('Nutrition', style: Theme.of(ctx).textTheme.titleLarge),
              const SizedBox(height: 8),
              if (entries.isEmpty) const Text('No nutrient data.'),
              for (final e in entries)
                ListTile(
                  dense: true,
                  title: Text(e['name'] as String? ?? ''),
                  trailing: Text(
                      '${(e['amount'] as num?)?.toStringAsFixed(1) ?? '0'} ${e['unit'] ?? ''}'),
                ),
              if (warnings.isNotEmpty) ...[
                const Divider(),
                for (final w in warnings)
                  ListTile(
                    dense: true,
                    leading: const Icon(Icons.warning_amber),
                    title: Text(w),
                  ),
              ],
            ],
          ),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Nutrition unavailable: $e')));
      }
    }
  }

  Widget _slotChip(Map<String, dynamic> slot) {
    final recipe = slot['recipe'] as Map<String, dynamic>?;
    final itemCount = (slot['items'] as List? ?? []).length;
    final label = recipe?['name'] as String? ??
        (itemCount == 0
            ? 'Slot'
            : '$itemCount item${itemCount == 1 ? '' : 's'}');
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: InkWell(
        onTap: () => _openSlotDetail(slot),
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            children: [
              Expanded(
                child: Text(label,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.bodySmall),
              ),
              AllergyWarningBadge(warnings: allergyWarningsOf(slot)),
              if (slot['servings'] != null)
                Text(' ×${slot['servings']}',
                    style: Theme.of(context).textTheme.bodySmall),
            ],
          ),
        ),
      ),
    );
  }

  Widget _cell(int day, String row) {
    final cellSlots = _slotsFor(day, row);
    return Container(
      constraints: const BoxConstraints(minHeight: 44),
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        border: Border.all(color: Theme.of(context).dividerColor, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ...cellSlots.map(_slotChip),
          Align(
            alignment: Alignment.centerLeft,
            child: IconButton(
              visualDensity: VisualDensity.compact,
              iconSize: 16,
              icon: const Icon(Icons.add),
              tooltip: 'Add $row on ${weekdayName(day)}',
              onPressed: () => _openSlotDialog(day: day, row: row),
            ),
          ),
        ],
      ),
    );
  }

  /// 7-column day grid for wide panes. Column width adapts to the pane —
  /// inside a two-pane layout this screen may be narrower than the window
  /// even at expanded breakpoints (e.g. ~920dp next to AdaptiveDetail's
  /// 360dp list). Columns shrink toward a 110dp floor so all seven days
  /// stay visible; below that the grid scrolls horizontally instead of
  /// squashing cells below readability.
  Widget _grid() {
    final rows = _rows;
    return LayoutBuilder(
      builder: (context, constraints) {
        final colWidth = ((constraints.maxWidth - 72) / 7).clamp(110.0, 140.0);
        return SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const SizedBox(width: 72),
                    for (final d in _orderedDays)
                      SizedBox(
                        width: colWidth,
                        child: Padding(
                          padding: const EdgeInsets.all(4),
                          child: Text(weekdayName(d),
                              style: Theme.of(context).textTheme.labelLarge),
                        ),
                      ),
                  ],
                ),
                for (final row in rows)
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(
                        width: 72,
                        child: Padding(
                          padding: const EdgeInsets.all(4),
                          child: Text(row,
                              style: Theme.of(context).textTheme.labelMedium),
                        ),
                      ),
                      for (final d in _orderedDays)
                        SizedBox(width: colWidth, child: _cell(d, row)),
                    ],
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  /// Vertically scrolling day sections for narrow panes.
  Widget _daySections() {
    final rows = _rows;
    return ListView(
      padding: const EdgeInsets.all(12),
      children: [
        for (final d in _orderedDays)
          Card(
            margin: const EdgeInsets.only(bottom: 12),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(weekdayName(d),
                      style: Theme.of(context).textTheme.titleSmall),
                  for (final row in rows) ...[
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(row,
                          style: Theme.of(context).textTheme.labelMedium),
                    ),
                    ..._slotsFor(d, row).map(_slotChip),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: TextButton.icon(
                        icon: const Icon(Icons.add, size: 16),
                        label: Text('Add $row'),
                        onPressed: () => _openSlotDialog(day: d, row: row),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final name = _plan?['name'] as String? ?? 'Meal plan';
    return Scaffold(
      appBar: AppBar(
        title: Text(name),
        actions: [
          IconButton(
            icon: _suggesting
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.auto_awesome),
            tooltip: 'Suggest meals',
            onPressed: _suggesting ? null : _suggest,
          ),
          IconButton(
            icon: const Icon(Icons.monitor_heart_outlined),
            tooltip: 'Nutrition',
            onPressed: _showNutrition,
          ),
          IconButton(
            icon: const Icon(Icons.edit_outlined),
            tooltip: 'Edit plan',
            onPressed: () async {
              await Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) =>
                      EditMealPlanScreen(mealPlanId: widget.mealPlanId),
                ),
              );
              _load();
            },
          ),
        ],
      ),
      body: _loading
          ? const SkeletonList()
          : _error != null
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text('Could not load plan: $_error'),
                      TextButton(onPressed: _load, child: const Text('Retry')),
                    ],
                  ),
                )
              // The pane width decides the layout — inside AdaptiveDetail's
              // right pane this screen can be narrow even on tablets.
              : LayoutBuilder(
                  builder: (ctx, c) =>
                      c.maxWidth >= 700 ? _grid() : _daySections(),
                ),
    );
  }
}

class _SlotDialog extends StatefulWidget {
  const _SlotDialog({
    required this.day,
    required this.mealType,
    required this.recipe,
    required this.servings,
    required this.note,
    required this.title,
    required this.onSave,
  });

  final int day;
  final String? mealType;
  final Map<String, dynamic>? recipe;
  final int? servings;
  final String? note;
  final String title;
  final Future<void> Function(Map<String, dynamic> input) onSave;

  @override
  State<_SlotDialog> createState() => _SlotDialogState();
}

class _SlotDialogState extends State<_SlotDialog> {
  late int _day = widget.day;
  late String _mealType =
      kMealTypes.contains(widget.mealType) ? widget.mealType! : 'Dinner';
  late Map<String, dynamic>? _recipe = widget.recipe;
  late final _servingsCtrl =
      TextEditingController(text: widget.servings?.toString() ?? '');
  late final _noteCtrl = TextEditingController(text: widget.note ?? '');
  bool _saving = false;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: SizedBox(
        width: 380,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DropdownButtonFormField<int>(
              initialValue: _day,
              decoration: const InputDecoration(labelText: 'Day'),
              items: [
                for (var d = 0; d < 7; d++)
                  DropdownMenuItem(value: d, child: Text(weekdayName(d))),
              ],
              onChanged: (v) => setState(() => _day = v ?? 0),
            ),
            const SizedBox(height: 8),
            DropdownButtonFormField<String>(
              initialValue: _mealType,
              decoration: const InputDecoration(labelText: 'Meal type'),
              items: [
                for (final t in kMealTypes)
                  DropdownMenuItem(value: t, child: Text(t)),
              ],
              onChanged: (v) => setState(() => _mealType = v ?? 'Dinner'),
            ),
            const SizedBox(height: 8),
            RecipePickerField(
              selected: _recipe,
              mealType: _mealType,
              label: 'Recipe (optional)',
              onChanged: (r) => setState(() => _recipe = r),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _servingsCtrl,
              decoration: const InputDecoration(labelText: 'Servings'),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _noteCtrl,
              decoration: const InputDecoration(labelText: 'Replacement note'),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.pop(context),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: _saving
              ? null
              : () async {
                  setState(() => _saving = true);
                  try {
                    await widget.onSave({
                      'dayOfWeek': _day,
                      'mealType': _mealType,
                      'recipeId': _recipe?['id'],
                      'servings': int.tryParse(_servingsCtrl.text.trim()),
                      'replacementNote': _noteCtrl.text.trim().isEmpty
                          ? null
                          : _noteCtrl.text.trim(),
                    });
                    if (context.mounted) Navigator.pop(context);
                  } finally {
                    if (mounted) setState(() => _saving = false);
                  }
                },
          child: const Text('Save'),
        ),
      ],
    );
  }
}

class _SuggestionSheet extends StatelessWidget {
  const _SuggestionSheet({required this.suggestions, required this.onAdd});

  final List<Map<String, dynamic>> suggestions;
  final Future<void> Function(Map<String, dynamic> suggestion) onAdd;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: ListView(
        shrinkWrap: true,
        padding: const EdgeInsets.all(16),
        children: [
          Text('Suggested meals',
              style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (suggestions.isEmpty) const Text('No suggestions.'),
          for (final s in suggestions)
            Card(
              child: ListTile(
                title: Text('${(s['recipe'] as Map?)?['name'] ?? 'Meal'} — '
                    '${weekdayName((s['dayOfWeek'] as num?)?.toInt())} ${s['mealType']}'),
                subtitle: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if ((s['reason'] as String? ?? '').isNotEmpty)
                      Text(s['reason'] as String),
                    if ((s['usesExpiringItems'] as List? ?? []).isNotEmpty)
                      Text(
                        'Uses expiring: ${(s['usesExpiringItems'] as List).join(', ')}',
                        style: TextStyle(color: Theme.of(context).hintColor),
                      ),
                  ],
                ),
                trailing: FilledButton(
                  onPressed: () => onAdd(s),
                  child: const Text('Add'),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
