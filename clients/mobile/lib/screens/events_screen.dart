import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/adaptive_detail.dart';
import '../widgets/empty_state.dart';
import '../widgets/paged_list_view.dart';
import '../widgets/skeleton.dart';
import 'edit_event_screen.dart';
import 'event_detail_screen.dart';

const int foodEventsPageSize = 50;

const String foodEventsQuery = r'''
  query FoodEvents($page: Int) {
    foodEvents(page: $page, pageSize: 50) {
      items {
        id
        name
        eventDate
        slotGranularityMinutes
        isActive
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class EventsScreen extends StatefulWidget {
  const EventsScreen({super.key});

  @override
  State<EventsScreen> createState() => _EventsScreenState();
}

class _EventsScreenState extends State<EventsScreen> {
  VoidCallback? _refetch;
  String? _selectedId;

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(foodEventsQuery),
        variables: const {'page': 1},
      ),
      builder: (QueryResult result,
          {VoidCallback? refetch, FetchMore? fetchMore}) {
        _refetch = refetch;
        return Scaffold(
          appBar: AppBar(title: const Text('Events')),
          body: AdaptiveDetail(
            list: _body(result, refetch, fetchMore),
            detail: _selectedId == null
                ? null
                : EventDetailScreen(
                    key: ValueKey(_selectedId), foodEventId: _selectedId!),
            placeholderIcon: Icons.event,
            placeholderTitle: 'Select an event',
            onDetailClosed: () => setState(() => _selectedId = null),
          ),
          floatingActionButton: FloatingActionButton(
            heroTag: 'fab-events',
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: (_) => const EditEventScreen()),
            ).then((_) => _refetch?.call()),
            child: const Icon(Icons.add),
          ),
        );
      },
    );
  }

  Widget _body(
      QueryResult result, VoidCallback? refetch, FetchMore? fetchMore) {
    if (result.isLoading) {
      return const SkeletonList();
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }

    final items = result.data?['foodEvents']?['items'] as List? ?? [];
    final total =
        result.data?['foodEvents']?['pageInfo']?['totalCount'] as int? ??
            items.length;
    if (items.isEmpty) {
      return const EmptyState(
        icon: Icons.event,
        title: 'No events yet',
        description: 'Tap + to plan one.',
      );
    }

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: PagedListView(
        loadedCount: items.length,
        totalCount: total,
        onLoadMore: () async {
          if (fetchMore == null) return;
          await fetchMore(FetchMoreOptions(
            variables: {
              'page': nextPageFor(items.length, foodEventsPageSize),
            },
            updateQuery: appendPageItems('foodEvents'),
          ));
        },
        itemBuilder: (context, index) {
          final event = items[index] as Map<String, dynamic>;
          final isActive = event['isActive'] as bool? ?? false;
          final granularity = event['slotGranularityMinutes'] as int? ?? 30;
          return Card(
            child: ListTile(
              leading: const Icon(Icons.event),
              title: Text(event['name'] as String),
              subtitle: Text(
                '${event['eventDate']} · $granularity-min schedule',
              ),
              trailing: Chip(label: Text(isActive ? 'Active' : 'Inactive')),
              onTap: () => openOrSelect(
                context,
                select: () =>
                    setState(() => _selectedId = event['id'] as String),
                builder: (_) =>
                    EventDetailScreen(foodEventId: event['id'] as String),
              )?.then((_) => refetch?.call()),
            ),
          );
        },
      ),
    );
  }
}
