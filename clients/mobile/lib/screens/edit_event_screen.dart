import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String createFoodEventMutation = r'''
  mutation CreateFoodEvent($input: CreateFoodEventInput!) {
    createFoodEvent(input: $input) {
      id
    }
  }
''';

const String updateFoodEventMutation = r'''
  mutation UpdateFoodEvent($id: ID!, $input: UpdateFoodEventInput!) {
    updateFoodEvent(id: $id, input: $input) {
      id
    }
  }
''';

class EditEventScreen extends StatefulWidget {
  final Map<String, dynamic>? event;

  const EditEventScreen({super.key, this.event});

  @override
  State<EditEventScreen> createState() => _EditEventScreenState();
}

class _EditEventScreenState extends State<EditEventScreen> {
  final _nameCtrl = TextEditingController();
  final _dateCtrl = TextEditingController();
  int _granularity = 30;
  bool _isSaving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final event = widget.event;
    if (event != null) {
      _nameCtrl.text = (event['name'] as String?) ?? '';
      _dateCtrl.text = ((event['eventDate'] as String?) ?? '').split('T').first;
      _granularity = event['slotGranularityMinutes'] as int? ?? 30;
    }
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _dateCtrl.dispose();
    super.dispose();
  }

  Future<void> _pickDate() async {
    final initial = DateTime.tryParse(_dateCtrl.text) ?? DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime(2020),
      lastDate: DateTime(2100),
    );
    if (picked == null) return;
    setState(() {
      _dateCtrl.text =
          '${picked.year.toString().padLeft(4, '0')}-${picked.month.toString().padLeft(2, '0')}-${picked.day.toString().padLeft(2, '0')}';
    });
  }

  Future<void> _save() async {
    if (_nameCtrl.text.trim().isEmpty || _dateCtrl.text.trim().isEmpty) {
      setState(() => _error = 'Name and date are required.');
      return;
    }
    setState(() {
      _isSaving = true;
      _error = null;
    });
    try {
      final client = GraphQLProvider.of(context).value;
      final event = widget.event;
      if (event == null) {
        final result = await client.mutate(MutationOptions(
          document: gql(createFoodEventMutation),
          variables: {
            'input': {
              'name': _nameCtrl.text.trim(),
              'eventDate': _dateCtrl.text.trim(),
              'slotGranularityMinutes': _granularity,
            }
          },
        ));
        if (result.hasException) throw result.exception!;
      } else {
        final result = await client.mutate(MutationOptions(
          document: gql(updateFoodEventMutation),
          variables: {
            'id': event['id'] as String,
            'input': {
              'name': _nameCtrl.text.trim(),
              'eventDate': _dateCtrl.text.trim(),
              'slotGranularityMinutes': _granularity,
            }
          },
        ));
        if (result.hasException) throw result.exception!;
      }
      if (mounted) Navigator.pop(context, true);
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.event == null ? 'New Event' : 'Edit Event'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            TextField(
              controller: _nameCtrl,
              decoration: const InputDecoration(
                labelText: 'Name',
                hintText: 'Thanksgiving dinner',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _dateCtrl,
              readOnly: true,
              onTap: _pickDate,
              decoration: const InputDecoration(
                labelText: 'Date',
                suffixIcon: Icon(Icons.calendar_today),
              ),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<int>(
              isExpanded: true,
              value: _granularity,
              decoration: const InputDecoration(
                labelText: 'Schedule granularity',
                helperText:
                    'Serve times and timeline steps snap to this boundary.',
              ),
              items: const [
                DropdownMenuItem(
                  value: 15,
                  child: Text('15 minutes (precise)'),
                ),
                DropdownMenuItem(
                  value: 30,
                  child: Text('30 minutes (simpler)'),
                ),
              ],
              onChanged: (v) => setState(() => _granularity = v ?? 30),
            ),
            if (_error != null) ...[
              const SizedBox(height: 16),
              Text(_error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ],
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: _isSaving ? null : _save,
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
