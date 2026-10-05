import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/skeleton.dart';
import 'event_detail_screen.dart' show hhmmOf;

const String eventTimelineQuery = r'''
  query EventTimeline($foodEventId: ID!) {
    eventTimeline(foodEventId: $foodEventId) {
      foodEventId
      warnings
      recipes {
        eventRecipeId
        name
        targetTime
        servings
        baseServings
        startBy
        unschedulable
        warnings
        steps {
          stepNumber
          instruction
          stepType
          isPassive
          appliance
          durationMinutes
          scheduledMinutes
          estimated
          startTime
          endTime
          conflicts
        }
      }
    }
  }
''';

class EventTimelineScreen extends StatelessWidget {
  final String foodEventId;
  final String eventName;

  const EventTimelineScreen({
    super.key,
    required this.foodEventId,
    required this.eventName,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('$eventName — Timeline')),
      body: Query(
        options: QueryOptions(
          document: gql(eventTimelineQuery),
          variables: {'foodEventId': foodEventId},
          fetchPolicy: FetchPolicy.networkOnly,
        ),
        builder: (QueryResult result,
            {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const SkeletonList();
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }
          final timeline =
              result.data?['eventTimeline'] as Map<String, dynamic>?;
          if (timeline == null) {
            return const Center(child: Text('No timeline available.'));
          }
          return _TimelineBody(timeline: timeline, refetch: refetch);
        },
      ),
    );
  }
}

class _TimelineBody extends StatelessWidget {
  final Map<String, dynamic> timeline;
  final VoidCallback? refetch;

  const _TimelineBody({required this.timeline, this.refetch});

  @override
  Widget build(BuildContext context) {
    final warnings = (timeline['warnings'] as List? ?? []).cast<String>();
    final recipes =
        (timeline['recipes'] as List? ?? []).cast<Map<String, dynamic>>();

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          if (warnings.isNotEmpty)
            Card(
              color: Theme.of(context).colorScheme.errorContainer,
              child: Padding(
                padding: const EdgeInsets.all(12.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Warnings',
                      style: TextStyle(fontWeight: FontWeight.bold),
                    ),
                    for (final w in warnings) Text('• $w'),
                  ],
                ),
              ),
            ),
          if (recipes.isEmpty)
            const Card(
              child: ListTile(
                title: Text('Nothing to schedule'),
                subtitle: Text('Add dishes to the event first.'),
              ),
            )
          else
            ...recipes.map((r) => _RecipeTimelineCard(recipe: r)),
        ],
      ),
    );
  }
}

class _RecipeTimelineCard extends StatelessWidget {
  final Map<String, dynamic> recipe;

  const _RecipeTimelineCard({required this.recipe});

  @override
  Widget build(BuildContext context) {
    final name = recipe['name'] as String? ?? 'Dish';
    final startBy = recipe['startBy'] as String?;
    final unschedulable = recipe['unschedulable'] as bool? ?? false;
    final warnings = (recipe['warnings'] as List? ?? []).cast<String>();
    final servings = recipe['servings'] as int?;
    final baseServings = recipe['baseServings'] as int?;
    final steps = (recipe['steps'] as List? ?? []).cast<Map<String, dynamic>>();

    final scaleHint =
        (servings != null && baseServings != null && servings != baseServings)
            ? ' — $servings servings (recipe makes $baseServings)'
            : servings != null
                ? ' — $servings servings'
                : '';

    return Card(
      margin: const EdgeInsets.symmetric(vertical: 6),
      child: Padding(
        padding: const EdgeInsets.all(12.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(name, style: Theme.of(context).textTheme.titleMedium),
            Text(
              unschedulable
                  ? 'Cannot be scheduled — see warnings$scaleHint'
                  : 'Serve ${hhmmOf(recipe['targetTime'] as String?)}'
                      '${startBy != null ? ' · start by ${hhmmOf(startBy)}' : ''}$scaleHint',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            for (final w in warnings)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  '⚠ $w',
                  style: Theme.of(context)
                      .textTheme
                      .bodySmall
                      ?.copyWith(color: Theme.of(context).colorScheme.error),
                ),
              ),
            const SizedBox(height: 8),
            ...steps.map((s) => _StepRow(step: s)),
          ],
        ),
      ),
    );
  }
}

class _StepRow extends StatelessWidget {
  final Map<String, dynamic> step;

  const _StepRow({required this.step});

  @override
  Widget build(BuildContext context) {
    final conflicts = (step['conflicts'] as List? ?? []).cast<String>();
    final isPassive = step['isPassive'] as bool? ?? false;
    final estimated = step['estimated'] as bool? ?? false;
    final appliance = step['appliance'] as String?;
    final hasConflict = conflicts.isNotEmpty;

    final start = hhmmOf(step['startTime'] as String?);
    final end = hhmmOf(step['endTime'] as String?);

    final tags = <String>[
      if (estimated) 'est.',
      if (isPassive) 'hands-off',
      if (appliance != null && appliance.isNotEmpty) appliance,
    ];

    return Container(
      padding: const EdgeInsets.symmetric(vertical: 6),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: Theme.of(context).dividerColor),
        ),
        color: hasConflict
            ? Theme.of(context)
                .colorScheme
                .errorContainer
                .withValues(alpha: 0.4)
            : null,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 96,
            child: Text(
              '$start–$end',
              style: Theme.of(context).textTheme.titleSmall,
            ),
          ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${step['stepNumber']}. ${step['instruction']}',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                if (tags.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      tags.join(' · '),
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Theme.of(context).colorScheme.secondary),
                    ),
                  ),
                for (final c in conflicts)
                  Text(
                    '⚠ $c',
                    style: Theme.of(context)
                        .textTheme
                        .bodySmall
                        ?.copyWith(color: Theme.of(context).colorScheme.error),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
