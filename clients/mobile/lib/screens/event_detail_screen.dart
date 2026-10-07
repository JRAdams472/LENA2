import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';
import '../allergy.dart';
import 'edit_event_screen.dart';
import 'event_timeline_screen.dart';

const String foodEventQuery = r'''
  query FoodEvent($id: ID!) {
    foodEvent(id: $id) {
      id
      name
      eventDate
      slotGranularityMinutes
      isActive
      recipes {
        id
        mealType
        targetTime
        servings
        baseServings
        scalingFactor
        notes
        recipe {
          id
          name
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

const String recipesQuery = r'''
  query Recipes {
    recipes(page: 1, pageSize: 200) {
      items {
        id
        name
      }
    }
  }
''';

const String deleteFoodEventMutation = r'''
  mutation DeleteFoodEvent($id: ID!) {
    deleteFoodEvent(id: $id)
  }
''';

const String addEventRecipeMutation = r'''
  mutation AddEventRecipe($input: AddEventRecipeInput!) {
    addEventRecipe(input: $input) {
      id
    }
  }
''';

const String updateEventRecipeMutation = r'''
  mutation UpdateEventRecipe($id: ID!, $input: UpdateEventRecipeInput!) {
    updateEventRecipe(id: $id, input: $input) {
      id
    }
  }
''';

const String removeEventRecipeMutation = r'''
  mutation RemoveEventRecipe($id: ID!) {
    removeEventRecipe(id: $id)
  }
''';

const mealTypes = ['breakfast', 'lunch', 'dinner', 'snack', 'other'];

// Serve times land on the event date; the resolver compares the date using
// the timestamp's own offset, so emit UTC like the web client.
String toTargetTime(String eventDate, String hhmm) => '${eventDate}T$hhmm:00Z';

String hhmmOf(String? iso) {
  if (iso == null) return '';
  final d = DateTime.tryParse(iso)?.toUtc();
  if (d == null) return '';
  return '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';
}

List<String> serveTimeOptions(int granularity) {
  final out = <String>[];
  for (var mins = 0; mins < 24 * 60; mins += granularity) {
    out.add(
      '${(mins ~/ 60).toString().padLeft(2, '0')}:${(mins % 60).toString().padLeft(2, '0')}',
    );
  }
  return out;
}

class EventDetailScreen extends StatefulWidget {
  final String foodEventId;

  const EventDetailScreen({super.key, required this.foodEventId});

  @override
  State<EventDetailScreen> createState() => _EventDetailScreenState();
}

class _EventDetailScreenState extends State<EventDetailScreen> {
  Map<String, dynamic>? _event;
  List<Map<String, dynamic>> _recipes = [];
  bool _loading = true;
  String? _error;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_event == null) _load();
  }

  Future<void> _load() async {
    final client = GraphQLProvider.of(context).value;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final results = await Future.wait([
        client.query(QueryOptions(
          document: gql(foodEventQuery),
          variables: {'id': widget.foodEventId},
          fetchPolicy: FetchPolicy.networkOnly,
        )),
        client.query(QueryOptions(document: gql(recipesQuery))),
      ]);
      final event = results[0].data?['foodEvent'] as Map<String, dynamic>?;
      if (event == null) {
        setState(() {
          _error = 'Event not found.';
          _loading = false;
        });
        return;
      }
      setState(() {
        _event = event;
        _recipes = (results[1].data?['recipes']?['items'] as List? ?? [])
            .cast<Map<String, dynamic>>();
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _deleteEvent() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete event?'),
        content: const Text(
          'This removes the event and all its dishes. Recipes are not affected.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(deleteFoodEventMutation),
      variables: {'id': widget.foodEventId},
    ));
    if (mounted) Navigator.pop(context);
  }

  Future<void> _removeSlot(String slotId) async {
    final client = GraphQLProvider.of(context).value;
    await client.mutate(MutationOptions(
      document: gql(removeEventRecipeMutation),
      variables: {'id': slotId},
    ));
    await _load();
  }

  Future<void> _openSlotEditor({Map<String, dynamic>? slot}) async {
    final saved = await showDialog<bool>(
      context: context,
      builder: (_) => _SlotDialog(
        event: _event!,
        slot: slot,
        recipes: _recipes,
      ),
    );
    if (saved == true) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final event = _event;
    return Scaffold(
      appBar: AppBar(
        title: Text(event?['name'] as String? ?? 'Event'),
        actions: [
          IconButton(
            icon: const Icon(Icons.edit),
            tooltip: 'Edit event',
            onPressed: event == null
                ? null
                : () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => EditEventScreen(event: event),
                      ),
                    ).then((saved) {
                      if (saved == true) _load();
                    }),
          ),
          IconButton(
            icon: const Icon(Icons.delete),
            tooltip: 'Delete event',
            onPressed: event == null ? null : _deleteEvent,
          ),
        ],
      ),
      body: _body(),
      floatingActionButton: event == null
          ? null
          : FloatingActionButton.extended(
              heroTag: 'fab-event-detail',
              onPressed: () => _openSlotEditor(),
              icon: const Icon(Icons.add),
              label: const Text('Add dish'),
            ),
    );
  }

  Widget _body() {
    if (_loading) {
      return const SkeletonList();
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Error: $_error', textAlign: TextAlign.center),
            const SizedBox(height: 8),
            TextButton(onPressed: _load, child: const Text('Retry')),
          ],
        ),
      );
    }
    final event = _event!;
    final slots = (event['recipes'] as List? ?? []).cast<Map<String, dynamic>>()
      ..sort((a, b) =>
          (a['targetTime'] as String).compareTo(b['targetTime'] as String));

    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          Card(
            child: ListTile(
              leading: const Icon(Icons.calendar_month),
              title: Text(event['eventDate'] as String),
              subtitle: Text(
                '${event['slotGranularityMinutes']}-min schedule',
              ),
            ),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(
                builder: (_) => EventTimelineScreen(
                  foodEventId: widget.foodEventId,
                  eventName: event['name'] as String? ?? 'Event',
                ),
              ),
            ),
            icon: const Icon(Icons.schedule),
            label: const Text('View cooking timeline'),
          ),
          const SizedBox(height: 16),
          Text('Dishes', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (slots.isEmpty)
            const Card(
              child: ListTile(
                title: Text('No dishes yet'),
                subtitle:
                    Text('Add a recipe or a free-form dish with a serve time.'),
              ),
            )
          else
            ...slots.map(_slotCard),
          // Room so the FAB doesn't cover the last card.
          const SizedBox(height: 80),
        ],
      ),
    );
  }

  Widget _slotCard(Map<String, dynamic> slot) {
    final recipe = slot['recipe'] as Map<String, dynamic>?;
    final name = recipe?['name'] as String? ?? 'Free-form dish';
    final servings = slot['servings'] as int?;
    final baseServings = slot['baseServings'] as int?;
    final factor = (slot['scalingFactor'] as num?)?.toDouble() ?? 1.0;
    final notes = slot['notes'] as String?;
    final scaleLabel = (servings != null && baseServings != null && factor != 1)
        ? ' · ×${_trimNum(factor)} of $baseServings'
        : '';

    return Card(
      margin: const EdgeInsets.symmetric(vertical: 4),
      child: ListTile(
        title: Row(
          children: [
            Expanded(child: Text(name)),
            AllergyWarningBadge(warnings: allergyWarningsOf(slot)),
          ],
        ),
        subtitle: Text(
          'serve ${hhmmOf(slot['targetTime'] as String?)} · '
          '${slot['mealType']}'
          '${servings != null ? ' · $servings servings$scaleLabel' : ''}'
          '${notes != null && notes.isNotEmpty ? '\n$notes' : ''}',
        ),
        isThreeLine: notes != null && notes.isNotEmpty,
        onTap: () => _openSlotEditor(slot: slot),
        trailing: IconButton(
          icon: const Icon(Icons.delete_outline),
          onPressed: () => _removeSlot(slot['id'] as String),
        ),
      ),
    );
  }

  static String _trimNum(double v) =>
      v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(1);
}

class _SlotDialog extends StatefulWidget {
  final Map<String, dynamic> event;
  final Map<String, dynamic>? slot;
  final List<Map<String, dynamic>> recipes;

  const _SlotDialog({
    required this.event,
    required this.slot,
    required this.recipes,
  });

  @override
  State<_SlotDialog> createState() => _SlotDialogState();
}

class _SlotDialogState extends State<_SlotDialog> {
  String? _recipeId;
  String _mealType = 'dinner';
  late String _time;
  final _servingsCtrl = TextEditingController();
  final _notesCtrl = TextEditingController();
  bool _saving = false;
  String? _error;

  List<String> get _times =>
      serveTimeOptions(widget.event['slotGranularityMinutes'] as int? ?? 30);

  @override
  void initState() {
    super.initState();
    final slot = widget.slot;
    if (slot != null) {
      _recipeId = (slot['recipe'] as Map?)?['id'] as String?;
      _mealType = (slot['mealType'] as String?) ?? 'dinner';
      _time = hhmmOf(slot['targetTime'] as String?);
      _servingsCtrl.text = (slot['servings'] as int?)?.toString() ?? '';
      _notesCtrl.text = (slot['notes'] as String?) ?? '';
    } else {
      _time = _times.contains('18:00') ? '18:00' : _times.first;
    }
    if (!_times.contains(_time)) _time = _times.first;
  }

  @override
  void dispose() {
    _servingsCtrl.dispose();
    _notesCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final client = GraphQLProvider.of(context).value;
      final targetTime =
          toTargetTime(widget.event['eventDate'] as String, _time);
      final servings = int.tryParse(_servingsCtrl.text.trim());
      final notes = _notesCtrl.text.trim();
      if (widget.slot == null) {
        final result = await client.mutate(MutationOptions(
          document: gql(addEventRecipeMutation),
          variables: {
            'input': {
              'foodEventId': widget.event['id'] as String,
              'recipeId': _recipeId,
              'mealType': _mealType,
              'targetTime': targetTime,
              'servings': servings,
              'notes': notes.isEmpty ? null : notes,
            }
          },
        ));
        if (result.hasException) throw result.exception!;
        if (_recipeId != null) {
          recordSelection(client, 'recipe', _recipeId!);
        }
      } else {
        final result = await client.mutate(MutationOptions(
          document: gql(updateEventRecipeMutation),
          variables: {
            'id': widget.slot!['id'] as String,
            'input': {
              'recipeId': _recipeId,
              'mealType': _mealType,
              'targetTime': targetTime,
              'servings': servings,
              'notes': notes.isEmpty ? null : notes,
            }
          },
        ));
        if (result.hasException) throw result.exception!;
      }
      if (mounted) Navigator.pop(context, true);
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.slot == null ? 'Add dish' : 'Edit dish'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DropdownButtonFormField<String?>(
              isExpanded: true,
              initialValue: _recipeId,
              decoration: const InputDecoration(labelText: 'Recipe'),
              items: [
                const DropdownMenuItem(
                  value: null,
                  child: Text('Free-form dish'),
                ),
                ...widget.recipes.map(
                  (r) => DropdownMenuItem(
                    value: r['id'] as String,
                    child: Text(r['name'] as String),
                  ),
                ),
              ],
              onChanged: (v) => setState(() => _recipeId = v),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              isExpanded: true,
              initialValue: _mealType,
              decoration: const InputDecoration(labelText: 'Meal type'),
              items: mealTypes
                  .map((m) => DropdownMenuItem(value: m, child: Text(m)))
                  .toList(),
              onChanged: (v) => setState(() => _mealType = v ?? 'dinner'),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              isExpanded: true,
              initialValue: _time,
              decoration: const InputDecoration(
                labelText: 'Serve time',
                helperText: 'Snaps to the event schedule',
              ),
              items: _times
                  .map((t) => DropdownMenuItem(value: t, child: Text(t)))
                  .toList(),
              onChanged: (v) => setState(() => _time = v ?? _time),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _servingsCtrl,
              decoration: const InputDecoration(
                labelText: 'Servings (optional)',
              ),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _notesCtrl,
              decoration: const InputDecoration(labelText: 'Notes (optional)'),
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(
                _error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context, false),
          child: const Text('Cancel'),
        ),
        TextButton(
          onPressed: _saving ? null : _save,
          child: _saving
              ? const SizedBox(
                  height: 16,
                  width: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Text('Save'),
        ),
      ],
    );
  }
}
