import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../responsive.dart';
import '../widgets/skeleton.dart';

import 'dashboard_content.dart';

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
            prepTimeMinutes
            cookTimeMinutes
          }
        }
      }
    }
    recommendedRecipes(limit: $limit) {
      recipe {
        id
        name
        description
        prepTimeMinutes
        cookTimeMinutes
        averageRating
        categories {
          name
          group {
            name
          }
        }
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
      builder: (
        QueryResult result, {
        VoidCallback? refetch,
        FetchMore? fetchMore,
      }) {
        _refetch = refetch;
        return Scaffold(
          appBar: AppBar(title: const Text('Dashboard')),
          body:
              ConstrainedContent(child: _body(context, result, refetch)),
        );
      },
    );
  }

  Widget _body(
      BuildContext context, QueryResult result, VoidCallback? refetch) {
    if (result.isLoading) {
      return const SkeletonList();
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
                TextButton(onPressed: refetch, child: const Text('Retry')),
              ],
            ),
          ),
        ),
      );
    }

    return DashboardContent(data: result.data ?? {}, onRefresh: refetch);
  }
}
