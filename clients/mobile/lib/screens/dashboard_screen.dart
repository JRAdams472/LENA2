import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String dashboardQuery = r'''
  query Dashboard($limit: Int) {
    mealPlans(page: 1, pageSize: 1) {
      items {
        id
        weekStartDate
        slots {
          id
          dayOfWeek
          mealType
          servings
          recipe {
            id
            name
          }
        }
      }
    }
    recommendedRecipes(limit: $limit) {
      recipe {
        id
        name
      }
      reason
      score
    }
    me {
      id
      email
      displayName
    }
    householdInvites {
      id
      status
      fromUser {
        id
        displayName
        firstName
        lastName
      }
      toUser {
        id
      }
    }
    unreadNotificationCount
  }
''';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  VoidCallback? _refetch;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    // Poll invites/notifications so the badge reflects invites sent while
    // the app sits open. Timer.periodic (not QueryOptions.pollInterval)
    // because it cancels on dispose, keeping widget tests timer-clean.
    _poll = Timer.periodic(
      const Duration(seconds: 30),
      (_) => _refetch?.call(),
    );
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(dashboardQuery),
        variables: const {'limit': 10},
      ),
      builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
        _refetch = refetch;
        return Scaffold(
          appBar: AppBar(title: const Text('Dashboard')),
          body: _body(context, result, refetch),
        );
      },
    );
  }

  Widget _body(BuildContext context, QueryResult result, VoidCallback? refetch) {
    if (result.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (result.hasException) {
      return SingleChildScrollView(
        padding: const EdgeInsets.all(16.0),
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 600),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(
                  'Error: ${result.exception.toString()}',
                  textAlign: TextAlign.center,
                  softWrap: true,
                ),
                const SizedBox(height: 16),
                TextButton(
                  onPressed: refetch,
                  child: const Text('Retry'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    final me = result.data?['me'] as Map<String, dynamic>?;
    final name = (me?['displayName'] as String?) ?? (me?['email'] as String?) ?? 'there';
    final plans = result.data?['mealPlans']?['items'] as List? ?? [];
    final recommendations = result.data?['recommendedRecipes'] as List? ?? [];
    final myId = me?['id'] as String?;
    final incomingInvites = (result.data?['householdInvites'] as List? ?? [])
        .cast<Map<String, dynamic>>()
        .where((i) =>
            (i['toUser'] as Map?)?['id'] == myId && i['status'] == 'PENDING')
        .toList();
    final unread = result.data?['unreadNotificationCount'] as int? ?? 0;

    final slots = _todaysSlots(plans);

    return RefreshIndicator(
      onRefresh: () async => refetch?.call(),
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          Text('Hello, $name', style: Theme.of(context).textTheme.headlineSmall),
          if (incomingInvites.isNotEmpty || unread > 0) ...[
            const SizedBox(height: 8),
            for (final inv in incomingInvites)
              Card(
                color: Theme.of(context).colorScheme.secondaryContainer,
                child: ListTile(
                  leading: const Icon(Icons.mail_outline),
                  title: Text(
                    '${_inviteSender(inv['fromUser'] as Map<String, dynamic>?)} invited you to their household',
                  ),
                  subtitle: const Text('Open the Household tab to respond'),
                ),
              ),
            if (unread > 0)
              Card(
                child: ListTile(
                  leading: Badge(
                    label: Text('$unread'),
                    child: const Icon(Icons.notifications),
                  ),
                  title: Text(
                    unread == 1
                        ? '1 unread household notification'
                        : '$unread unread household notifications',
                  ),
                ),
              ),
          ],
          const SizedBox(height: 24),
          Text('Today\'s Meal Plan', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (slots.isEmpty)
            const Card(
              child: ListTile(
                title: Text('No slots for today'),
                subtitle: Text('Create a meal plan to see it here.'),
              ),
            )
          else
            ...slots.map((slot) {
              final recipe = slot['recipe'] as Map<String, dynamic>?;
              return Card(
                child: ListTile(
                  title: Text(recipe?['name'] as String? ?? 'Unknown recipe'),
                  subtitle: Text('Meal: ${slot['mealType']} — ${slot['servings']} servings'),
                ),
              );
            }),
          const SizedBox(height: 24),
          Text('Suggested for You', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (recommendations.isEmpty)
            const Card(
              child: ListTile(
                title: Text('No suggestions yet'),
                subtitle: Text('Rate and plan recipes to get recommendations.'),
              ),
            )
          else
            ...recommendations.take(5).map((rec) {
              final recipe = rec['recipe'] as Map<String, dynamic>?;
              final reason = rec['reason'] as String? ?? '';
              return Card(
                child: ListTile(
                  title: Text(recipe?['name'] as String? ?? 'Unknown'),
                  subtitle: Text('Reason: $reason'),
                ),
              );
            }),
        ],
      ),
    );
  }

  String _inviteSender(Map<String, dynamic>? u) {
    if (u == null) return 'Someone';
    final display = u['displayName'] as String?;
    if (display != null && display.isNotEmpty) return display;
    final combined = [u['firstName'], u['lastName']]
        .whereType<String>()
        .join(' ');
    return combined.isNotEmpty ? combined : 'Someone';
  }

  List<Map<String, dynamic>> _todaysSlots(List<dynamic> plans) {
    if (plans.isEmpty) return [];
    final plan = plans.first as Map<String, dynamic>;
    final slots = plan['slots'] as List? ?? [];
    final today = DateTime.now().weekday;
    return slots
        .cast<Map<String, dynamic>>()
        .where((s) => (s['dayOfWeek'] as int?) == today)
        .toList();
  }
}
