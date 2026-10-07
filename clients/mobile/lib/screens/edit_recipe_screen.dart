import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/skeleton.dart';
import '../analytics/analytics.dart';
import '../allergy.dart';
import '../recipe_delta.dart';
import 'recipe_tweaks.dart';

const String itemsQuery = r'''
  query Items($search: String) {
    items(page: 1, pageSize: 50, search: $search) {
      items {
        id
        name
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

const String ingredientsQuery = r'''
  query Ingredients($search: String) {
    ingredients(page: 1, pageSize: 50, search: $search) {
      items {
        id
        name
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

const String getOrCreateIngredientMutation = r'''
  mutation GetOrCreateIngredient($input: CreateIngredientInput!) {
    getOrCreateIngredient(input: $input) {
      id
      name
    }
  }
''';

const String recipeQuery = r'''
  query Recipe($id: ID!, $view: RecipeView!) {
    recipe(id: $id) {
      id
      name
      description
      servings
      prepTimeMinutes
      cookTimeMinutes
      isFavorite
      categories {
        id
        name
        group {
          id
          name
          exclusive
          displayOrder
        }
      }
      items(view: $view) {
        id
        deltaKind
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
        section
        displayOrder
        notes
        isOptional
      }
      steps(view: $view) {
        id
        stepNumber
        instruction
        durationMinutes
        stepType
        isPassive
        dependsOnStepNumber
        appliance
        deltaKind
      }
      householdDelta {
        id
        stale
        orphanedItemCount
        orphanedStepCount
        updatedAt
        items {
          id
          kind
          recipeItemId
          item { id name }
          ingredient { id name }
          quantity
          unit
          unitId
          section
          displayOrder
          notes
          isOptional
          orphaned
        }
        steps {
          id
          kind
          stepId
          stepNumber
          instruction
          durationMinutes
          stepType
          isPassive
          dependsOnStepNumber
          appliance
          orphaned
        }
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
    me {
      id
      household {
        id
        myRole
      }
    }
  }
''';

const String setRecipeDeltaMutation = r'''
  mutation SetRecipeDelta(
    $recipeId: ID!
    $items: [RecipeDeltaItemInput!]!
    $steps: [RecipeDeltaStepInput!]!
  ) {
    setRecipeDelta(recipeId: $recipeId, items: $items, steps: $steps) {
      id
    }
  }
''';

const String clearRecipeDeltaMutation = r'''
  mutation ClearRecipeDelta($recipeId: ID!) {
    clearRecipeDelta(recipeId: $recipeId)
  }
''';

const String acknowledgeRecipeDeltaMutation = r'''
  mutation AcknowledgeRecipeDelta($recipeId: ID!) {
    acknowledgeRecipeDelta(recipeId: $recipeId) {
      id
    }
  }
''';

const String recipeCategoryGroupsQuery = r'''
  query RecipeCategoryGroups {
    recipeCategoryGroups {
      id
      name
      exclusive
      displayOrder
      categories {
        id
        name
      }
    }
  }
''';

const String setRecipeCategoriesMutation = r'''
  mutation SetRecipeCategories($recipeId: ID!, $categoryIds: [ID!]!) {
    setRecipeCategories(recipeId: $recipeId, categoryIds: $categoryIds) {
      id
    }
  }
''';

const String createRecipeMutation = r'''
  mutation CreateRecipe($input: CreateRecipeInput!) {
    createRecipe(input: $input) {
      id
    }
  }
''';

const String updateRecipeMutation = r'''
  mutation UpdateRecipe($id: ID!, $input: CreateRecipeInput!) {
    updateRecipe(id: $id, input: $input) {
      id
    }
  }
''';

const String setRecipeFavoriteMutation = r'''
  mutation SetRecipeFavorite($recipeId: ID!, $isFavorite: Boolean!) {
    setRecipeFavorite(recipeId: $recipeId, isFavorite: $isFavorite)
  }
''';

class EditRecipeScreen extends StatefulWidget {
  final String? recipeId;

  const EditRecipeScreen({super.key, this.recipeId});

  @override
  State<EditRecipeScreen> createState() => _EditRecipeScreenState();
}

class _EditRecipeScreenState extends State<EditRecipeScreen> {
  final _nameCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final _servingsCtrl = TextEditingController();
  final _prepCtrl = TextEditingController();
  final _cookCtrl = TextEditingController();
  final _qtyCtrl = TextEditingController();
  final _unitCtrl = TextEditingController();
  final _stepCtrl = TextEditingController();
  final _itemSearchCtrl = TextEditingController();
  final _ingredientSearchCtrl = TextEditingController();
  final _debouncer = Debouncer();

  String? _itemId;
  String? _ingredientId;
  bool _loaded = false;
  bool _isSaving = false;
  bool _isFavorite = false;
  bool _isTogglingFavorite = false;
  bool _isSavingCategories = false;
  List<Map<String, dynamic>> _items = [];
  List<Map<String, dynamic>> _ingredients = [];
  List<Map<String, dynamic>> _categoryGroups = [];
  List<Map<String, dynamic>> _allergyWarnings = [];
  List<Map<String, dynamic>> _allergenFlags = [];
  final Set<String> _selectedCategoryIds = {};

  // LEN-25 household delta state. The effective view is the recipe with
  // the household's change set applied; _view toggles the contents list
  // between it and the untouched canonical rows.
  String _view = 'effective';
  Map<String, dynamic>? _delta;
  List<Map<String, dynamic>> _rawDeltaItems = [];
  List<Map<String, dynamic>> _rawDeltaSteps = [];
  List<DeltaItemDraft> _itemDrafts = [];
  List<DeltaStepDraft> _stepDrafts = [];
  List<Map<String, dynamic>> _viewItems = [];
  List<Map<String, dynamic>> _viewSteps = [];
  Map<String, String> _itemBaseLabels = {};
  Map<String, int> _stepBaseNumbers = {};
  bool _canTweak = false;
  bool _isSavingDelta = false;

  bool get _deltaDirty =>
      !itemDraftsEqual(
          _itemDrafts, _rawDeltaItems.map(DeltaItemDraft.fromRow).toList()) ||
      !stepDraftsEqual(
          _stepDrafts, _rawDeltaSteps.map(DeltaStepDraft.fromRow).toList());

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _loadData();
    }
  }

  Future<void> _loadItems(String search) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(QueryOptions(
      document: gql(itemsQuery),
      variables: {'search': search.isEmpty ? null : search},
    ));
    if (!mounted) return;
    setState(() {
      final loaded = (result.data?['items']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      if (_itemId != null && !loaded.any((i) => i['id'] == _itemId)) {
        final prev = _items.where((i) => i['id'] == _itemId).toList();
        if (prev.isNotEmpty) loaded.insert(0, prev.first);
      }
      _items = loaded;
    });
  }

  void _onItemSearchChanged(String value) {
    _debouncer.run(() {
      final term = value.trim();
      if (term.isNotEmpty) {
        recordSearch(GraphQLProvider.of(context).value, 'item', term);
      }
      _loadItems(term);
    });
  }

  Future<void> _loadIngredients(String search) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(QueryOptions(
      document: gql(ingredientsQuery),
      variables: {'search': search.isEmpty ? null : search},
    ));
    if (!mounted) return;
    setState(() {
      final loaded = (result.data?['ingredients']?['items'] as List? ?? [])
          .cast<Map<String, dynamic>>();
      if (_ingredientId != null &&
          !loaded.any((i) => i['id'] == _ingredientId)) {
        final prev =
            _ingredients.where((i) => i['id'] == _ingredientId).toList();
        if (prev.isNotEmpty) loaded.insert(0, prev.first);
      }
      _ingredients = loaded;
    });
  }

  void _onIngredientSearchChanged(String value) {
    _debouncer.run(() {
      final term = value.trim();
      if (term.isNotEmpty) {
        recordSearch(GraphQLProvider.of(context).value, 'ingredient', term);
      }
      _loadIngredients(term);
    });
  }

  Future<void> _createIngredient(String name) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.mutate(MutationOptions(
      document: gql(getOrCreateIngredientMutation),
      variables: {
        'input': {'name': name},
      },
    ));
    final created =
        result.data?['getOrCreateIngredient'] as Map<String, dynamic>?;
    if (!mounted || created == null) return;
    setState(() {
      _ingredients = [
        created,
        ..._ingredients.where((i) => i['id'] != created['id']),
      ];
      _ingredientId = created['id'] as String;
    });
    recordSelection(client, 'ingredient', created['id'] as String);
  }

  Future<void> _loadData() async {
    final client = GraphQLProvider.of(context).value;
    await Future.wait([_loadItems(''), _loadIngredients('')]);

    if (widget.recipeId != null) {
      recordView(client, 'recipe', widget.recipeId!);

      final groupsResult = await client
          .query(QueryOptions(document: gql(recipeCategoryGroupsQuery)));
      setState(() {
        _categoryGroups =
            (groupsResult.data?['recipeCategoryGroups'] as List? ?? [])
                .cast<Map<String, dynamic>>();
      });

      final data = await _loadRecipe('effective');
      if (data != null) {
        final me = data['me'] as Map<String, dynamic>?;
        _canTweak = me?['household'] != null;
        _applyRecipe(data);
        final delta = (data['recipe'] as Map<String, dynamic>?)?['householdDelta'];
        // Canonical rows feed base labels for tweak descriptions and the
        // canonical edit form — fetch them whenever a delta exists (the
        // effective list alone can't name a removed/swapped line).
        if (delta != null) {
          final canonical = await _loadRecipe('canonical');
          if (canonical != null) {
            _applyCanonicalBase(canonical['recipe'] as Map<String, dynamic>?);
          }
        } else {
          _applyCanonicalBase(data['recipe'] as Map<String, dynamic>?);
        }
      }
    }
  }

  Future<Map<String, dynamic>?> _loadRecipe(String view) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(
      QueryOptions(
        document: gql(recipeQuery),
        variables: {'id': widget.recipeId, 'view': view},
        fetchPolicy: FetchPolicy.networkOnly,
      ),
    );
    if (result.hasException) return null;
    return result.data;
  }

  /// Populates view/delta state from a recipe query response.
  void _applyRecipe(Map<String, dynamic> data) {
    final recipe = data['recipe'] as Map<String, dynamic>?;
    if (recipe == null) return;
    final delta = recipe['householdDelta'] as Map<String, dynamic>?;
    setState(() {
      _viewItems = (recipe['items'] as List? ?? []).cast<Map<String, dynamic>>();
      _viewSteps = (recipe['steps'] as List? ?? []).cast<Map<String, dynamic>>();
      _delta = delta;
      _rawDeltaItems =
          (delta?['items'] as List? ?? []).cast<Map<String, dynamic>>();
      _rawDeltaSteps =
          (delta?['steps'] as List? ?? []).cast<Map<String, dynamic>>();
      _itemDrafts = _rawDeltaItems.map(DeltaItemDraft.fromRow).toList();
      _stepDrafts = _rawDeltaSteps.map(DeltaStepDraft.fromRow).toList();
    });
  }

  /// Canonical rows → anchor lookup maps + the admin edit form. The form
  /// must seed from canonical data, not the household's effective rows.
  void _applyCanonicalBase(Map<String, dynamic>? recipe) {
    if (recipe == null) return;
    final items =
        (recipe['items'] as List? ?? []).cast<Map<String, dynamic>>();
    final steps =
        (recipe['steps'] as List? ?? []).cast<Map<String, dynamic>>();
    setState(() {
      _itemBaseLabels = {
        for (final i in items)
          i['id'] as String: ((i['ingredient']?['name'] ??
                  i['item']?['name'] ??
                  'line ${i['id']}') as String),
      };
      _stepBaseNumbers = {
        for (final s in steps)
          s['id'] as String: (s['stepNumber'] as num).toInt(),
      };
    });
    if (!_formFilled) _fillForm(recipe);
  }

  bool _formFilled = false;

  void _fillForm(Map<String, dynamic> recipe) {
    _formFilled = true;
        final item = (recipe['items'] as List?)?[0] as Map<String, dynamic>?;
        final step = (recipe['steps'] as List?)?[0] as Map<String, dynamic>?;
        setState(() {
          _nameCtrl.text = (recipe['name'] as String?) ?? '';
          _descCtrl.text = (recipe['description'] as String?) ?? '';
          _servingsCtrl.text = recipe['servings']?.toString() ?? '';
          _prepCtrl.text = recipe['prepTimeMinutes']?.toString() ?? '';
          _cookCtrl.text = recipe['cookTimeMinutes']?.toString() ?? '';
          _itemId = item?['item']?['id'] as String?;
          _ingredientId = item?['ingredient']?['id'] as String?;
          final recipeItem = item?['item'] as Map<String, dynamic>?;
          if (recipeItem != null && !_items.any((i) => i['id'] == _itemId)) {
            _items = [recipeItem, ..._items];
          }
          final recipeIngredient = item?['ingredient'] as Map<String, dynamic>?;
          if (recipeIngredient != null &&
              !_ingredients.any((i) => i['id'] == _ingredientId)) {
            _ingredients = [recipeIngredient, ..._ingredients];
          }
          _qtyCtrl.text = item?['quantity']?.toString() ?? '';
          _unitCtrl.text = (item?['unit'] as String?) ?? '';
          _stepCtrl.text = (step?['instruction'] as String?) ?? '';
          _isFavorite = (recipe['isFavorite'] as bool?) ?? false;
          _selectedCategoryIds
            ..clear()
            ..addAll((recipe['categories'] as List? ?? [])
                .map((c) => c['id'] as String));
          _allergyWarnings = allergyWarningsOf(recipe);
          _allergenFlags = allergenFlagsOf(recipe);
        });
  }

  Future<void> _setCategories(Set<String> next) async {
    if (widget.recipeId == null) return;
    final previous = Set<String>.from(_selectedCategoryIds);
    setState(() {
      _selectedCategoryIds
        ..clear()
        ..addAll(next);
      _isSavingCategories = true;
    });
    try {
      final result =
          await GraphQLProvider.of(context).value.mutate(MutationOptions(
                document: gql(setRecipeCategoriesMutation),
                variables: {
                  'recipeId': widget.recipeId,
                  'categoryIds': _selectedCategoryIds.toList(),
                },
              ));
      if (result.hasException) {
        throw result.exception ?? Exception('setRecipeCategories failed');
      }
    } catch (_) {
      // setRecipeCategories is admin-only; on rejection the optimistic
      // toggle must roll back instead of showing an unsaved selection.
      if (mounted) {
        setState(() {
          _selectedCategoryIds
            ..clear()
            ..addAll(previous);
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not update categories')),
        );
      }
    } finally {
      if (mounted) setState(() => _isSavingCategories = false);
    }
  }

  Future<void> _toggleFavorite() async {
    if (widget.recipeId == null) return;
    setState(() => _isTogglingFavorite = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final next = !_isFavorite;
      await client.mutate(MutationOptions(
        document: gql(setRecipeFavoriteMutation),
        variables: {'recipeId': widget.recipeId, 'isFavorite': next},
      ));
      if (mounted) setState(() => _isFavorite = next);
    } finally {
      if (mounted) setState(() => _isTogglingFavorite = false);
    }
  }

  // ---------- household delta ----------

  Future<void> _switchView(String view) async {
    if (view == _view) return;
    final data = await _loadRecipe(view);
    if (data == null) return;
    setState(() => _view = view);
    final recipe = data['recipe'] as Map<String, dynamic>?;
    if (recipe != null) {
      setState(() {
        _viewItems =
            (recipe['items'] as List? ?? []).cast<Map<String, dynamic>>();
        _viewSteps =
            (recipe['steps'] as List? ?? []).cast<Map<String, dynamic>>();
        _delta = recipe['householdDelta'] as Map<String, dynamic>?;
      });
    }
  }

  Future<void> _acknowledgeDelta() async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.mutate(MutationOptions(
      document: gql(acknowledgeRecipeDeltaMutation),
      variables: {'recipeId': widget.recipeId},
    ));
    if (result.hasException || !mounted) return;
    setState(() => _delta?['stale'] = false);
  }

  Future<void> _saveDelta() async {
    setState(() => _isSavingDelta = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final result = await client.mutate(MutationOptions(
        document: gql(setRecipeDeltaMutation),
        variables: {
          'recipeId': widget.recipeId,
          'items': _itemDrafts.map(toDeltaItemInput).toList(),
          'steps': _stepDrafts.map(toDeltaStepInput).toList(),
        },
      ));
      if (result.hasException) {
        throw result.exception ?? Exception('setRecipeDelta failed');
      }
      final data = await _loadRecipe('effective');
      if (data != null) _applyRecipe(data);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not save tweaks')),
        );
      }
    } finally {
      if (mounted) setState(() => _isSavingDelta = false);
    }
  }

  Future<void> _clearDelta() async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.mutate(MutationOptions(
      document: gql(clearRecipeDeltaMutation),
      variables: {'recipeId': widget.recipeId},
    ));
    if (result.hasException || !mounted) return;
    setState(() {
      _delta = null;
      _rawDeltaItems = [];
      _rawDeltaSteps = [];
      _itemDrafts = [];
      _stepDrafts = [];
    });
  }

  void _discardDeltaEdits() {
    setState(() {
      _itemDrafts = _rawDeltaItems.map(DeltaItemDraft.fromRow).toList();
      _stepDrafts = _rawDeltaSteps.map(DeltaStepDraft.fromRow).toList();
    });
  }

  /// Apply a returned sheet draft to the item change set — one change per
  /// anchored line, so a re-tweak replaces the earlier draft.
  void _applyItemDraft(DeltaItemDraft draft) {
    setState(() {
      if (draft.recipeItemId != null) {
        _itemDrafts
            .removeWhere((d) => d.recipeItemId == draft.recipeItemId);
      }
      _itemDrafts.add(draft);
    });
  }

  void _applyStepDraft(DeltaStepDraft draft) {
    setState(() {
      if (draft.stepId != null) {
        _stepDrafts.removeWhere((d) => d.stepId == draft.stepId);
      }
      _stepDrafts.add(draft);
    });
  }

  Future<void> _tweakItem(Map<String, dynamic> line) async {
    final label = (line['ingredient']?['name'] ?? line['item']?['name'] ??
        'this line') as String;
    final draft = await showItemTweakSheet(
      context: context,
      anchorId: line['id'] as String?,
      anchorLabel: label,
    );
    if (draft != null) _applyItemDraft(draft);
  }

  Future<void> _tweakStep(Map<String, dynamic> step) async {
    final draft = await showStepTweakSheet(
      context: context,
      anchorStepId: step['id'] as String?,
      anchorNumber: (step['stepNumber'] as num?)?.toInt(),
      nextStepNumber: _viewSteps.length + 1,
    );
    if (draft != null) _applyStepDraft(draft);
  }

  Future<void> _addLineTweak() async {
    final draft = await showItemTweakSheet(context: context);
    if (draft != null) _applyItemDraft(draft);
  }

  Future<void> _addStepTweak() async {
    final draft = await showStepTweakSheet(
      context: context,
      nextStepNumber: _viewSteps.length + 1,
    );
    if (draft != null) _applyStepDraft(draft);
  }

  void _removeDraft(bool isStep, Object draft) {
    setState(() {
      if (isStep) {
        _stepDrafts.remove(draft);
      } else {
        _itemDrafts.remove(draft);
      }
    });
  }

  /// "2 cup milk" — quantity + unit + ingredient/item name.
  String _lineLabel(Map<String, dynamic> line) {
    final name = (line['ingredient']?['name'] ?? line['item']?['name'] ?? '')
        as String;
    final qty = line['quantity'];
    final unit = line['unit'] as String?;
    final bits = [
      if (qty != null) '$qty',
      if (unit != null && unit.isNotEmpty) unit,
      name,
    ].join(' ');
    return bits.trim().isEmpty ? '(line)' : bits.trim();
  }

  Widget _contentsRow({
    required IconData icon,
    required String label,
    required String? deltaKind,
    required bool tweakable,
    required VoidCallback onTweak,
  }) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        children: [
          Icon(icon, size: 16, color: theme.colorScheme.primary),
          const SizedBox(width: 6),
          Expanded(
            child: Wrap(
              spacing: 6,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(label),
                if (deltaKind != null && deltaKindLabels.containsKey(deltaKind))
                  Chip(
                    label: Text(deltaKindLabels[deltaKind]!),
                    visualDensity: VisualDensity.compact,
                    labelStyle: theme.textTheme.labelSmall?.copyWith(
                        color: theme.colorScheme.onPrimary),
                    backgroundColor: theme.colorScheme.primary,
                  ),
              ],
            ),
          ),
          if (tweakable)
            TextButton(
              onPressed: onTweak,
              style: TextButton.styleFrom(
                  visualDensity: VisualDensity.compact),
              child: const Text('Tweak'),
            ),
        ],
      ),
    );
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _descCtrl.dispose();
    _servingsCtrl.dispose();
    _prepCtrl.dispose();
    _cookCtrl.dispose();
    _qtyCtrl.dispose();
    _unitCtrl.dispose();
    _stepCtrl.dispose();
    _itemSearchCtrl.dispose();
    _ingredientSearchCtrl.dispose();
    _debouncer.dispose();
    super.dispose();
  }

  Future<void> _save(BuildContext context) async {
    setState(() => _isSaving = true);
    try {
      final client = GraphQLProvider.of(context).value;
      final input = <String, dynamic>{
        'name': _nameCtrl.text,
        'description': _descCtrl.text.isEmpty ? null : _descCtrl.text,
        'servings': _servingsCtrl.text.isEmpty
            ? null
            : int.tryParse(_servingsCtrl.text),
        'prepTimeMinutes':
            _prepCtrl.text.isEmpty ? null : int.tryParse(_prepCtrl.text),
        'cookTimeMinutes':
            _cookCtrl.text.isEmpty ? null : int.tryParse(_cookCtrl.text),
        'items': [
          {
            'itemId': _itemId,
            'ingredientId': _ingredientId,
            'quantity': double.tryParse(_qtyCtrl.text) ?? 0,
            'unit': _unitCtrl.text,
            'notes': null,
            'isOptional': false,
          },
        ],
        'steps': [
          {
            'stepNumber': 1,
            'instruction': _stepCtrl.text,
          },
        ],
      };
      if (widget.recipeId == null) {
        await client.mutate(MutationOptions(
          document: gql(createRecipeMutation),
          variables: {'input': input},
        ));
      } else {
        await client.mutate(MutationOptions(
          document: gql(updateRecipeMutation),
          variables: {'id': widget.recipeId, 'input': input},
        ));
      }
      if (mounted) Navigator.pop(context);
    } finally {
      setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_items.isEmpty || _ingredients.isEmpty) {
      return Scaffold(
        appBar: AppBar(
          title:
              Text(widget.recipeId == null ? 'Create Recipe' : 'Edit Recipe'),
        ),
        body: const SkeletonForm(),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.recipeId == null ? 'Create Recipe' : 'Edit Recipe'),
        actions: widget.recipeId == null
            ? null
            : [
                IconButton(
                  icon: Icon(_isFavorite ? Icons.star : Icons.star_border),
                  onPressed: _isTogglingFavorite ? null : _toggleFavorite,
                ),
              ],
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: ListView(
          children: [
            if (_allergyWarnings.isNotEmpty)
              Card(
                color: allergyWarningSevere(_allergyWarnings)
                    ? Theme.of(context)
                        .colorScheme
                        .error
                        .withValues(alpha: 0.10)
                    : Theme.of(context)
                        .colorScheme
                        .tertiary
                        .withValues(alpha: 0.10),
                child: Padding(
                  padding: const EdgeInsets.all(12.0),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      AllergyWarningBadge(warnings: _allergyWarnings),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            for (final w in _allergyWarnings)
                              Text(allergyWarningText(w)),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 4.0),
              child: Wrap(
                spacing: 6,
                runSpacing: 4,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  Text('Allergens:',
                      style: Theme.of(context).textTheme.bodySmall),
                  if (_allergenFlags.isEmpty)
                    Text('No allergen information',
                        style: Theme.of(context)
                            .textTheme
                            .bodySmall
                            ?.copyWith(fontStyle: FontStyle.italic))
                  else
                    for (final f in _allergenFlags)
                      Chip(
                        label: Text(
                          f['kind'] == 'contains'
                              ? '${(f['allergen'] as Map?)?['name']}'
                              : '${(f['allergen'] as Map?)?['name']} (may contain)',
                          style: Theme.of(context)
                              .textTheme
                              .labelSmall
                              ?.copyWith(
                                  letterSpacing: 0,
                                  color:
                                      Theme.of(context).colorScheme.onSurface),
                        ),
                        visualDensity: VisualDensity.compact,
                      ),
                ],
              ),
            ),
            // LEN-25 — household delta surface (members only).
            if (widget.recipeId != null && _canTweak) ...[
              if (_delta?['stale'] == true)
                Card(
                  color: Theme.of(context)
                      .colorScheme
                      .tertiary
                      .withValues(alpha: 0.10),
                  child: Padding(
                    padding: const EdgeInsets.all(12.0),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'The original recipe changed since your tweaks '
                          'were saved — review and mark as checked when '
                          'happy.${((_delta?['orphanedItemCount'] as num? ?? 0) +
                                  (_delta?['orphanedStepCount'] as num? ?? 0)) >
                              0 ? ' Some tweaks no longer apply.' : ''}',
                        ),
                        Align(
                          alignment: Alignment.centerRight,
                          child: TextButton(
                            onPressed: _acknowledgeDelta,
                            child: const Text('Mark reviewed'),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              if (_delta != null) ...[
                const SizedBox(height: 8),
                Row(
                  children: [
                    Chip(
                      label: const Text('Household version'),
                      visualDensity: VisualDensity.compact,
                      labelStyle:
                          Theme.of(context).textTheme.labelSmall?.copyWith(
                                color:
                                    Theme.of(context).colorScheme.primary,
                              ),
                      side: BorderSide(
                          color: Theme.of(context).colorScheme.primary),
                    ),
                    const Spacer(),
                    SegmentedButton<String>(
                      style: SegmentedButton.styleFrom(
                          visualDensity: VisualDensity.compact),
                      segments: const [
                        ButtonSegment(
                            value: 'effective', label: Text('Household')),
                        ButtonSegment(
                            value: 'canonical', label: Text('Original')),
                      ],
                      selected: {_view},
                      onSelectionChanged: (s) => _switchView(s.first),
                    ),
                  ],
                ),
              ],
              const SizedBox(height: 8),
              const Text('Contents (household view)',
                  style: TextStyle(fontWeight: FontWeight.bold)),
              if (_viewItems.isEmpty && _viewSteps.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 4),
                  child: Text('No contents yet.'),
                ),
              for (final line in _viewItems)
                _contentsRow(
                  icon: Icons.restaurant,
                  label: _lineLabel(line),
                  deltaKind: line['deltaKind'] as String?,
                  tweakable: _view == 'effective' && line['id'] != '0',
                  onTweak: () => _tweakItem(line),
                ),
              for (final step in _viewSteps)
                _contentsRow(
                  icon: Icons.format_list_numbered,
                  label:
                      '${step['stepNumber']}. ${step['instruction']}',
                  deltaKind: step['deltaKind'] as String?,
                  tweakable: _view == 'effective' && step['id'] != '0',
                  onTweak: () => _tweakStep(step),
                ),
              const SizedBox(height: 8),
              RecipeTweaksCard(
                itemDrafts: _itemDrafts,
                stepDrafts: _stepDrafts,
                itemBaseLabels: _itemBaseLabels,
                stepBaseNumbers: _stepBaseNumbers,
                dirty: _deltaDirty,
                saving: _isSavingDelta,
                onAddLineTweak: _addLineTweak,
                onAddStepTweak: _addStepTweak,
                onRemoveDraft: _removeDraft,
                onSave: _saveDelta,
                onDiscard: _discardDeltaEdits,
                onClearAll: _clearDelta,
              ),
              const SizedBox(height: 8),
            ],
            TextField(
              controller: _nameCtrl,
              decoration: const InputDecoration(labelText: 'Name'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _descCtrl,
              decoration: const InputDecoration(labelText: 'Description'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _servingsCtrl,
              decoration: const InputDecoration(labelText: 'Servings'),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _prepCtrl,
              decoration: const InputDecoration(labelText: 'Prep minutes'),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _cookCtrl,
              decoration: const InputDecoration(labelText: 'Cook minutes'),
              keyboardType: TextInputType.number,
            ),
            const SizedBox(height: 16),
            const Text('Ingredient',
                style: TextStyle(fontWeight: FontWeight.bold)),
            TextField(
              controller: _ingredientSearchCtrl,
              decoration: const InputDecoration(
                labelText: 'Search ingredients',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _onIngredientSearchChanged,
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String?>(
              isExpanded: true,
              value: _ingredientId,
              decoration: const InputDecoration(labelText: 'Ingredient'),
              items: [
                ..._ingredients.map(
                  (i) => DropdownMenuItem(
                    value: i['id'] as String,
                    child: Text(
                      i['name'] as String,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ),
                if (_ingredientSearchCtrl.text.trim().isNotEmpty)
                  DropdownMenuItem(
                    value: '__create__',
                    child: Text(
                      'Create "${_ingredientSearchCtrl.text.trim()}"',
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
              ],
              onChanged: (v) => setState(() {
                if (v == '__create__') {
                  _createIngredient(_ingredientSearchCtrl.text.trim());
                  return;
                }
                _ingredientId = v;
                if (v != null) {
                  recordSelection(
                      GraphQLProvider.of(context).value, 'ingredient', v);
                }
              }),
            ),
            const SizedBox(height: 8),
            const Text('Preferred brand (optional)',
                style: TextStyle(fontWeight: FontWeight.bold)),
            TextField(
              controller: _itemSearchCtrl,
              decoration: const InputDecoration(
                labelText: 'Search items',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: _onItemSearchChanged,
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String?>(
              isExpanded: true,
              value: _itemId,
              decoration:
                  const InputDecoration(labelText: 'Preferred brand item'),
              items: _items
                  .map((i) => DropdownMenuItem(
                        value: i['id'] as String,
                        child: Text(
                          i['name'] as String,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ))
                  .toList(),
              onChanged: (v) => setState(() {
                _itemId = v;
                if (v != null) {
                  recordSelection(GraphQLProvider.of(context).value, 'item', v);
                }
              }),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _qtyCtrl,
              decoration: const InputDecoration(labelText: 'Quantity'),
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _unitCtrl,
              decoration: const InputDecoration(labelText: 'Unit'),
            ),
            const SizedBox(height: 16),
            const Text('Step 1', style: TextStyle(fontWeight: FontWeight.bold)),
            TextField(
              controller: _stepCtrl,
              decoration: const InputDecoration(labelText: 'Instruction'),
            ),
            if (widget.recipeId != null && _categoryGroups.isNotEmpty) ...[
              const SizedBox(height: 16),
              Row(
                children: [
                  const Expanded(
                    child: Text('Categories',
                        style: TextStyle(fontWeight: FontWeight.bold)),
                  ),
                  if (_isSavingCategories)
                    const SizedBox(
                      height: 16,
                      width: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                ],
              ),
              for (final group in _categoryGroups) ...[
                const SizedBox(height: 8),
                Text(
                  '${group['name']}'
                  '${group['exclusive'] == true ? ' (pick one)' : ''}',
                  style: const TextStyle(fontStyle: FontStyle.italic),
                ),
                Wrap(
                  spacing: 8,
                  children: [
                    for (final cat in (group['categories'] as List? ?? []))
                      if (group['exclusive'] == true)
                        ChoiceChip(
                          label: Text(cat['name'] as String),
                          selected: _selectedCategoryIds.contains(cat['id']),
                          onSelected: (sel) {
                            final ids = (group['categories'] as List? ?? [])
                                .map((c) => c['id'] as String)
                                .toSet();
                            final next = _selectedCategoryIds.difference(ids);
                            if (sel) next.add(cat['id'] as String);
                            _setCategories(next);
                          },
                        )
                      else
                        FilterChip(
                          label: Text(cat['name'] as String),
                          selected: _selectedCategoryIds.contains(cat['id']),
                          onSelected: (sel) {
                            final next = {..._selectedCategoryIds};
                            if (sel) {
                              next.add(cat['id'] as String);
                            } else {
                              next.remove(cat['id']);
                            }
                            _setCategories(next);
                          },
                        ),
                  ],
                ),
              ],
            ],
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
