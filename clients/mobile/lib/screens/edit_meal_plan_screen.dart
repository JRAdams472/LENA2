import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String mealPlanQuery = r'''
  query MealPlan($id: ID!) {
    mealPlan(id: $id) {
      id
      name
      weekStartDate
      isActive
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
  bool _isSaving = false;
  bool _loaded = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _loadData();
    }
  }

  Future<void> _loadData() async {
    if (widget.mealPlanId == null) return;
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(
      QueryOptions(
        document: gql(mealPlanQuery),
        variables: {'id': widget.mealPlanId},
      ),
    );
    final plan = result.data?['mealPlan'] as Map<String, dynamic>?;
    if (plan != null) {
      setState(() {
        _nameCtrl.text = (plan['name'] as String?) ?? '';
        _dateCtrl.text = (plan['weekStartDate'] as String?) ?? '';
        _dayCtrl.text = (plan['weekStartDayOfWeek']?.toString()) ?? '0';
      });
    }
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _dateCtrl.dispose();
    _dayCtrl.dispose();
    super.dispose();
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

  @override
  Widget build(BuildContext context) {
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
              decoration: const InputDecoration(
                labelText: 'Week start date (YYYY-MM-DD)',
              ),
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
          ],
        ),
      ),
    );
  }
}
