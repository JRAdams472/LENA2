import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../recipe_delta.dart';

// Local copies of the catalog queries — this codebase keeps each screen's
// query strings next to the widget that uses them.
const String _itemsQuery = r'''
  query Items($search: String) {
    items(page: 1, pageSize: 50, search: $search) {
      items { id name }
    }
  }
''';

const String _ingredientsQuery = r'''
  query Ingredients($search: String) {
    ingredients(page: 1, pageSize: 50, search: $search) {
      items { id name }
    }
  }
''';

const String _unitsQuery = r'''
  query Units {
    units { id name }
  }
''';

/// The "Household tweaks" card — drafts the member edits locally, then
/// commits through setRecipeDelta. Mirrors the web tweaks panel.
class RecipeTweaksCard extends StatelessWidget {
  const RecipeTweaksCard({
    super.key,
    required this.itemDrafts,
    required this.stepDrafts,
    required this.itemBaseLabels,
    required this.stepBaseNumbers,
    required this.dirty,
    required this.saving,
    required this.onAddLineTweak,
    required this.onAddStepTweak,
    required this.onRemoveDraft,
    required this.onSave,
    required this.onDiscard,
    required this.onClearAll,
  });

  final List<DeltaItemDraft> itemDrafts;
  final List<DeltaStepDraft> stepDrafts;
  // Anchor → display label lookups (canonical rows), for "Swap X for Y".
  final Map<String, String> itemBaseLabels;
  final Map<String, int> stepBaseNumbers;
  final bool dirty;
  final bool saving;
  final VoidCallback onAddLineTweak;
  final VoidCallback onAddStepTweak;
  // Removes a draft by identity — kind is 'item' or 'step'.
  final void Function(bool isStep, Object draft) onRemoveDraft;
  final VoidCallback onSave;
  final VoidCallback onDiscard;
  final VoidCallback onClearAll;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final empty = itemDrafts.isEmpty && stepDrafts.isEmpty;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.tune, color: theme.colorScheme.primary, size: 20),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Household tweaks',
                    style: theme.textTheme.titleMedium,
                  ),
                ),
                if (!empty)
                  TextButton(
                    onPressed: onClearAll,
                    child: const Text('Clear all'),
                  ),
              ],
            ),
            Text(
              'Tweaks apply for everyone in your household — the original '
              'recipe stays unchanged. Use Tweak on a line or step, or add '
              'your own below.',
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 8),
            if (empty)
              Text(
                'No tweaks yet — the household sees the recipe as written.',
                style: theme.textTheme.bodySmall
                    ?.copyWith(fontStyle: FontStyle.italic),
              )
            else ...[
              for (final d in itemDrafts)
                _DraftRow(
                  icon: Icons.restaurant,
                  text: describeItemChange(
                      d, itemBaseLabels[d.recipeItemId]),
                  orphaned: d.orphaned,
                  onRemove: () => onRemoveDraft(false, d),
                ),
              for (final d in stepDrafts)
                _DraftRow(
                  icon: Icons.format_list_numbered,
                  text: describeStepChange(
                      d, stepBaseNumbers[d.stepId]),
                  orphaned: d.orphaned,
                  onRemove: () => onRemoveDraft(true, d),
                ),
            ],
            const Divider(height: 20),
            Wrap(
              spacing: 8,
              runSpacing: 4,
              children: [
                OutlinedButton.icon(
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('Add ingredient'),
                  onPressed: onAddLineTweak,
                ),
                OutlinedButton.icon(
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('Add step'),
                  onPressed: onAddStepTweak,
                ),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                if (dirty)
                  Expanded(
                    child: Text(
                      'Unsaved changes',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.tertiary,
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  )
                else
                  const Spacer(),
                TextButton(
                  onPressed: dirty ? onDiscard : null,
                  child: const Text('Discard'),
                ),
                const SizedBox(width: 4),
                FilledButton(
                  onPressed: dirty && !saving ? onSave : null,
                  child: saving
                      ? const SizedBox(
                          height: 16,
                          width: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Save tweaks'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _DraftRow extends StatelessWidget {
  const _DraftRow({
    required this.icon,
    required this.text,
    required this.orphaned,
    required this.onRemove,
  });

  final IconData icon;
  final String text;
  final bool orphaned;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 16, color: theme.colorScheme.primary),
          const SizedBox(width: 6),
          Expanded(
            child: Wrap(
              spacing: 6,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(text),
                if (orphaned)
                  Chip(
                    label: const Text('No longer applies'),
                    visualDensity: VisualDensity.compact,
                    labelStyle: theme.textTheme.labelSmall,
                    backgroundColor:
                        theme.colorScheme.error.withValues(alpha: 0.12),
                  ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close, size: 18),
            tooltip: 'Remove tweak',
            visualDensity: VisualDensity.compact,
            onPressed: onRemove,
          ),
        ],
      ),
    );
  }
}

/// Kind selector + fields for an item-line tweak. Returns the finished
/// draft, or null when the sheet is dismissed without applying.
Future<DeltaItemDraft?> showItemTweakSheet({
  required BuildContext context,
  // Non-null when tweaking a canonical line; null = add-a-line sheet.
  String? anchorId,
  String? anchorLabel,
}) {
  return showModalBottomSheet<DeltaItemDraft>(
    context: context,
    isScrollControlled: true,
    builder: (sheetContext) => _ItemTweakSheet(
      anchorId: anchorId,
      anchorLabel: anchorLabel,
    ),
  );
}

class _ItemTweakSheet extends StatefulWidget {
  const _ItemTweakSheet({this.anchorId, this.anchorLabel});

  final String? anchorId;
  final String? anchorLabel;

  @override
  State<_ItemTweakSheet> createState() => _ItemTweakSheetState();
}

class _ItemTweakSheetState extends State<_ItemTweakSheet> {
  // substitute | adjust | remove for anchored lines; add is implied when
  // the sheet opens without an anchor.
  late String _kind = widget.anchorId == null ? 'add' : 'substitute';
  final _qtyCtrl = TextEditingController();
  final _notesCtrl = TextEditingController();
  final _searchCtrl = TextEditingController();

  List<Map<String, dynamic>> _ingredients = [];
  List<Map<String, dynamic>> _items = [];
  List<Map<String, dynamic>> _units = [];
  String? _ingredientId;
  String? _itemId;
  String? _unitId;
  bool _isOptional = false;

  @override
  void initState() {
    super.initState();
    // Populate pickers once the sheet has a context.
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final client = GraphQLProvider.of(context).value;
      final results = await Future.wait([
        client.query(QueryOptions(
            document: gql(_ingredientsQuery), variables: const {})),
        client.query(
            QueryOptions(document: gql(_itemsQuery), variables: const {})),
        client.query(QueryOptions(document: gql(_unitsQuery))),
      ]);
      if (!mounted) return;
      setState(() {
        _ingredients =
            (results[0].data?['ingredients']?['items'] as List? ?? [])
                .cast<Map<String, dynamic>>();
        _items = (results[1].data?['items']?['items'] as List? ?? [])
            .cast<Map<String, dynamic>>();
        _units = (results[2].data?['units'] as List? ?? [])
            .cast<Map<String, dynamic>>();
      });
    });
  }

  @override
  void dispose() {
    _qtyCtrl.dispose();
    _notesCtrl.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  bool get _canApply {
    switch (_kind) {
      case 'remove':
        return true;
      case 'add':
      case 'substitute':
        return _ingredientId != null || _itemId != null;
      default:
        return true; // adjust — any field may change
    }
  }

  DeltaItemDraft _draft() => DeltaItemDraft(
        recipeItemId: widget.anchorId,
        kind: _kind,
        itemId: _itemId,
        ingredientId: _ingredientId,
        itemName: _items
            .where((i) => i['id'] == _itemId)
            .map((i) => i['name'] as String?)
            .firstOrNull,
        ingredientName: _ingredients
            .where((i) => i['id'] == _ingredientId)
            .map((i) => i['name'] as String?)
            .firstOrNull,
        quantity: double.tryParse(_qtyCtrl.text),
        unitId: _unitId,
        unit: _units
            .where((u) => u['id'] == _unitId)
            .map((u) => u['name'] as String?)
            .firstOrNull,
        notes: _notesCtrl.text.isEmpty ? null : _notesCtrl.text,
        isOptional: _isOptional,
      );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final title = widget.anchorId == null
        ? 'Add an ingredient line'
        : 'Tweak ${widget.anchorLabel ?? 'this line'}';
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(title, style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            if (widget.anchorId != null)
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'substitute', label: Text('Swap')),
                  ButtonSegment(value: 'adjust', label: Text('Adjust')),
                  ButtonSegment(value: 'remove', label: Text('Remove')),
                ],
                selected: {_kind},
                onSelectionChanged: (s) => setState(() => _kind = s.first),
              ),
            if (_kind == 'substitute' || _kind == 'add') ...[
              const SizedBox(height: 12),
              TextField(
                controller: _searchCtrl,
                decoration: const InputDecoration(
                  labelText: 'Search ingredients',
                  prefixIcon: Icon(Icons.search),
                  isDense: true,
                ),
                onChanged: (v) async {
                  final client = GraphQLProvider.of(context).value;
                  final result = await client.query(QueryOptions(
                    document: gql(_ingredientsQuery),
                    variables: {'search': v.isEmpty ? null : v},
                  ));
                  if (!mounted) return;
                  setState(() {
                    _ingredients = (result.data?['ingredients']?['items']
                                as List? ??
                            [])
                        .cast<Map<String, dynamic>>();
                  });
                },
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String?>(
                isExpanded: true,
                initialValue: _ingredientId,
                decoration: const InputDecoration(labelText: 'Ingredient'),
                items: _ingredients
                    .map((i) => DropdownMenuItem(
                          value: i['id'] as String,
                          child: Text(i['name'] as String,
                              overflow: TextOverflow.ellipsis),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _ingredientId = v),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String?>(
                isExpanded: true,
                initialValue: _itemId,
                decoration: const InputDecoration(
                    labelText: 'Brand item (optional)'),
                items: _items
                    .map((i) => DropdownMenuItem(
                          value: i['id'] as String,
                          child: Text(i['name'] as String,
                              overflow: TextOverflow.ellipsis),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _itemId = v),
              ),
            ],
            if (_kind != 'remove') ...[
              const SizedBox(height: 8),
              TextField(
                controller: _qtyCtrl,
                decoration: const InputDecoration(labelText: 'Quantity'),
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String?>(
                isExpanded: true,
                initialValue: _unitId,
                decoration: const InputDecoration(labelText: 'Unit'),
                items: _units
                    .map((u) => DropdownMenuItem(
                          value: u['id'] as String,
                          child: Text(u['name'] as String,
                              overflow: TextOverflow.ellipsis),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _unitId = v),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _notesCtrl,
                decoration: const InputDecoration(labelText: 'Notes'),
              ),
              CheckboxListTile(
                dense: true,
                contentPadding: EdgeInsets.zero,
                title: const Text('Optional'),
                value: _isOptional,
                onChanged: (v) => setState(() => _isOptional = v ?? false),
              ),
            ],
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: () => Navigator.pop(context),
                  child: const Text('Cancel'),
                ),
                FilledButton(
                  onPressed:
                      _canApply ? () => Navigator.pop(context, _draft()) : null,
                  child: const Text('Apply tweak'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// Kind selector + fields for a step tweak. Returns the finished draft,
/// or null when dismissed.
Future<DeltaStepDraft?> showStepTweakSheet({
  required BuildContext context,
  // Non-null when tweaking a canonical step; null = add-a-step sheet.
  String? anchorStepId,
  int? anchorNumber,
  int nextStepNumber = 1,
}) {
  return showModalBottomSheet<DeltaStepDraft>(
    context: context,
    isScrollControlled: true,
    builder: (sheetContext) => _StepTweakSheet(
      anchorStepId: anchorStepId,
      anchorNumber: anchorNumber,
      nextStepNumber: nextStepNumber,
    ),
  );
}

class _StepTweakSheet extends StatefulWidget {
  const _StepTweakSheet({
    this.anchorStepId,
    this.anchorNumber,
    this.nextStepNumber = 1,
  });

  final String? anchorStepId;
  final int? anchorNumber;
  final int nextStepNumber;

  @override
  State<_StepTweakSheet> createState() => _StepTweakSheetState();
}

class _StepTweakSheetState extends State<_StepTweakSheet> {
  late String _kind = widget.anchorStepId == null ? 'add' : 'replace';
  late final _numberCtrl =
      TextEditingController(text: '${widget.nextStepNumber}');
  final _instructionCtrl = TextEditingController();
  final _durationCtrl = TextEditingController();

  @override
  void dispose() {
    _numberCtrl.dispose();
    _instructionCtrl.dispose();
    _durationCtrl.dispose();
    super.dispose();
  }

  bool get _canApply {
    if (_kind == 'remove') return true;
    return _instructionCtrl.text.trim().isNotEmpty;
  }

  DeltaStepDraft _draft() => DeltaStepDraft(
        stepId: widget.anchorStepId,
        kind: _kind,
        stepNumber:
            _kind == 'add' ? int.tryParse(_numberCtrl.text) : null,
        instruction: _kind == 'remove'
            ? null
            : _instructionCtrl.text.trim(),
        durationMinutes: int.tryParse(_durationCtrl.text),
      );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final title = widget.anchorStepId == null
        ? 'Add a step'
        : 'Tweak step ${widget.anchorNumber ?? ''}';
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(title, style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            if (widget.anchorStepId != null)
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'replace', label: Text('Replace')),
                  ButtonSegment(value: 'remove', label: Text('Remove')),
                ],
                selected: {_kind},
                onSelectionChanged: (s) => setState(() => _kind = s.first),
              ),
            if (_kind == 'add')
              TextField(
                controller: _numberCtrl,
                decoration: const InputDecoration(labelText: 'At step'),
                keyboardType: TextInputType.number,
              ),
            if (_kind != 'remove') ...[
              TextField(
                controller: _instructionCtrl,
                decoration:
                    const InputDecoration(labelText: 'Directions'),
                onChanged: (_) => setState(() {}),
                maxLines: 3,
                minLines: 1,
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _durationCtrl,
                decoration:
                    const InputDecoration(labelText: 'Duration (min)'),
                keyboardType: TextInputType.number,
              ),
            ],
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: () => Navigator.pop(context),
                  child: const Text('Cancel'),
                ),
                FilledButton(
                  onPressed:
                      _canApply ? () => Navigator.pop(context, _draft()) : null,
                  child: const Text('Apply tweak'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
