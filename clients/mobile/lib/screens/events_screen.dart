import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'edit_event_screen.dart';
import 'event_detail_screen.dart';

const String foodEventsQuery = r'''
  query FoodEvents {
    foodEvents(page: 1, pageSize: 50) {
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

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(document: gql(foodEventsQuery)),
      builder: (QueryResult result,
          {VoidCallback? refetch, FetchMore? fetchMore}) {
        _refetch = refetch;
        return Scaffold(
          appBar: AppBar(title: const Text('Events')),
          body: _body(result, refetch),
          floatingActionButton: FloatingActionButton(
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

  Widget _body(QueryResult result, VoidCallback? refetch) {
    if (result.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (result.hasException) {
      return Center(child: Text('Error: ${result.exception.toString()}'));
    }

    final items = result.data?['foodEvents']?['items'] as List? ?? [];
    if (items.isEmpty) {
      return const Center(
        child: Text('No events yet — tap + to plan one.'),
      );
    }

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView.builder(
        padding: const EdgeInsets.all(16.0),
        itemCount: items.length,
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
              onTap: () => Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) =>
                      EventDetailScreen(foodEventId: event['id'] as String),
                ),
              ).then((_) => refetch?.call()),
            ),
          );
        },
      ),
    );
  }
}
