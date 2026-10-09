import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:wakelock_plus/wakelock_plus.dart';
import '../widgets/skeleton.dart';

const String cookRecipeQuery = r'''
  query CookRecipe($id: ID!) {
    recipe(id: $id) {
      id
      name
      servings
      items {
        quantity
        unit
        displayOrder
        notes
        isOptional
        item { name }
        ingredient { name }
      }
      steps {
        stepNumber
        instruction
        durationMinutes
        stepType
        isPassive
      }
    }
  }
''';

/// Full-screen cook mode (LEN-14 P4): effective-view steps in a swipeable
/// pager with big type, step-type/hands-off badges, and a per-step
/// countdown chip. Keeps the screen awake via wakelock_plus while open.
/// Ingredients live in a bottom sheet on narrow panes and a left rail on
/// wide ones. The schema has no step→item relationship, so ingredients
/// are global to the recipe, not per-step.
class CookModeScreen extends StatefulWidget {
  final String recipeId;

  const CookModeScreen({super.key, required this.recipeId});

  @override
  State<CookModeScreen> createState() => _CookModeScreenState();
}

class _CookModeScreenState extends State<CookModeScreen> {
  final _pager = PageController();
  Map<String, dynamic>? _recipe;
  bool _loading = true;
  Object? _error;
  bool _loaded = false;
  int _page = 0;

  Timer? _timer;
  int? _remainingSeconds;

  @override
  void initState() {
    super.initState();
    // MissingPluginException on platforms without the plugin — cook mode
    // still works, the screen just won't stay awake.
    unawaited(WakelockPlus.enable().catchError((_) => false));
  }

  @override
  void dispose() {
    _timer?.cancel();
    _pager.dispose();
    unawaited(WakelockPlus.disable().catchError((_) => false));
    super.dispose();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_loaded) {
      _loaded = true;
      _load();
    }
  }

  Future<void> _load() async {
    try {
      final client = GraphQLProvider.of(context).value;
      final result = await client.query(QueryOptions(
        document: gql(cookRecipeQuery),
        variables: {'id': widget.recipeId},
        fetchPolicy: FetchPolicy.networkOnly,
      ));
      if (!mounted) return;
      setState(() {
        _recipe = result.data?['recipe'] as Map<String, dynamic>?;
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

  List<Map<String, dynamic>> get _steps {
    final steps =
        (_recipe?['steps'] as List? ?? []).cast<Map<String, dynamic>>();
    steps.sort((a, b) => ((a['stepNumber'] as num?) ?? 0)
        .compareTo((b['stepNumber'] as num?) ?? 0));
    return steps;
  }

  List<Map<String, dynamic>> get _items {
    final items =
        (_recipe?['items'] as List? ?? []).cast<Map<String, dynamic>>();
    items.sort((a, b) => ((a['displayOrder'] as num?) ?? 0)
        .compareTo((b['displayOrder'] as num?) ?? 0));
    return items;
  }

  void _goTo(int page) {
    _timer?.cancel();
    setState(() {
      _remainingSeconds = null;
      _page = page;
    });
    _pager.animateToPage(
      page,
      duration: const Duration(milliseconds: 250),
      curve: Curves.easeOut,
    );
  }

  void _startCountdown(int minutes) {
    _timer?.cancel();
    setState(() => _remainingSeconds = minutes * 60);
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) {
        t.cancel();
        return;
      }
      setState(() {
        if ((_remainingSeconds ?? 0) <= 1) {
          _remainingSeconds = 0;
          t.cancel();
        } else {
          _remainingSeconds = _remainingSeconds! - 1;
        }
      });
    });
  }

  String _fmtRemaining() {
    final s = _remainingSeconds ?? 0;
    return '${s ~/ 60}:${(s % 60).toString().padLeft(2, '0')}';
  }

  Widget _ingredientList() {
    final items = _items;
    if (items.isEmpty) return const Text('No ingredients listed.');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final it in items)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 3),
            child: Text(
              '• ${it['quantity']} ${it['unit']} '
              '${it['item']?['name'] ?? it['ingredient']?['name'] ?? '?'}'
              '${it['isOptional'] == true ? ' (optional)' : ''}',
            ),
          ),
      ],
    );
  }

  void _showIngredientsSheet() {
    showModalBottomSheet(
      context: context,
      builder: (ctx) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          padding: const EdgeInsets.all(20),
          children: [
            Text('Ingredients', style: Theme.of(ctx).textTheme.titleLarge),
            const SizedBox(height: 8),
            _ingredientList(),
          ],
        ),
      ),
    );
  }

  Widget _stepPage(Map<String, dynamic> step, int index, int total) {
    final theme = Theme.of(context);
    final duration = (step['durationMinutes'] as num?)?.toInt();
    final timing = _remainingSeconds != null;
    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 16, 24, 96),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('${index + 1} / $total',
              style: theme.textTheme.displaySmall
                  ?.copyWith(fontWeight: FontWeight.bold)),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            children: [
              if ((step['stepType'] as String? ?? '').isNotEmpty)
                Chip(
                  avatar: const Icon(Icons.category_outlined, size: 16),
                  label: Text(step['stepType'] as String),
                ),
              if (step['isPassive'] == true)
                const Chip(
                  avatar: Icon(Icons.hourglass_empty, size: 16),
                  label: Text('Hands-off'),
                ),
              if (duration != null)
                ActionChip(
                  avatar: Icon(
                      timing ? Icons.restart_alt : Icons.timer_outlined,
                      size: 16),
                  label: Text(timing ? _fmtRemaining() : '$duration min'),
                  onPressed: () => _startCountdown(duration),
                ),
            ],
          ),
          const SizedBox(height: 24),
          Expanded(
            child: SingleChildScrollView(
              child: Text(
                step['instruction'] as String? ?? '',
                style: theme.textTheme.headlineSmall?.copyWith(height: 1.4),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _pagerView() {
    final steps = _steps;
    if (steps.isEmpty) {
      return const Center(child: Text('No steps in this recipe.'));
    }
    return Stack(
      children: [
        PageView.builder(
          controller: _pager,
          itemCount: steps.length,
          onPageChanged: (i) => setState(() {
            _timer?.cancel();
            _remainingSeconds = null;
            _page = i;
          }),
          itemBuilder: (ctx, i) => _stepPage(steps[i], i, steps.length),
        ),
        Positioned(
          left: 8,
          right: 8,
          bottom: 16,
          child: Row(
            children: [
              IconButton(
                icon: const Icon(Icons.chevron_left, size: 36),
                onPressed: _page > 0 ? () => _goTo(_page - 1) : null,
              ),
              Expanded(
                child: LinearProgressIndicator(
                  value: steps.length <= 1 ? 1 : (_page + 1) / steps.length,
                ),
              ),
              IconButton(
                icon: const Icon(Icons.chevron_right, size: 36),
                onPressed:
                    _page < steps.length - 1 ? () => _goTo(_page + 1) : null,
              ),
            ],
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final name = _recipe?['name'] as String? ?? 'Cook mode';
    return Scaffold(
      appBar: AppBar(title: Text(name)),
      body: _loading
          ? const SkeletonList()
          : _error != null || _recipe == null
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text('Could not load recipe: ${_error ?? 'not found'}'),
                      TextButton(onPressed: _load, child: const Text('Retry')),
                    ],
                  ),
                )
              // Pane width decides the ingredients placement — inside
              // AdaptiveDetail's right pane the cook view may be narrow.
              : LayoutBuilder(
                  builder: (ctx, c) {
                    if (c.maxWidth >= 700) {
                      return Row(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          SizedBox(
                            width: 280,
                            child: ListView(
                              padding: const EdgeInsets.all(16),
                              children: [
                                Text('Ingredients',
                                    style: Theme.of(ctx).textTheme.titleMedium),
                                const SizedBox(height: 8),
                                _ingredientList(),
                              ],
                            ),
                          ),
                          const VerticalDivider(width: 1),
                          Expanded(child: _pagerView()),
                        ],
                      );
                    }
                    return Stack(
                      children: [
                        _pagerView(),
                        Positioned(
                          right: 16,
                          bottom: 72,
                          child: FloatingActionButton.extended(
                            heroTag: 'fab-ingredients',
                            onPressed: _showIngredientsSheet,
                            icon: const Icon(Icons.list),
                            label: const Text('Ingredients'),
                          ),
                        ),
                      ],
                    );
                  },
                ),
    );
  }
}
