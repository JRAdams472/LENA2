import 'dart:async';

import 'package:graphql_flutter/graphql_flutter.dart';

const String _recordSelectionMutation = r'''
  mutation RecordSelection($entityType: EntityType!, $entityId: ID!) {
    recordSelection(entityType: $entityType, entityId: $entityId)
  }
''';

const String _recordSearchMutation = r'''
  mutation RecordSearch($entityType: EntityType!, $term: String!) {
    recordSearch(entityType: $entityType, term: $term)
  }
''';

const String _recordViewMutation = r'''
  mutation RecordView($entityType: EntityType!, $entityId: ID!) {
    recordView(entityType: $entityType, entityId: $entityId)
  }
''';

/// Fire-and-forget analytics signals. Failures are ignored — telemetry must
/// never block the UI. Server ranking consumes these events through the
/// decayed engagement tiers.
void recordSelection(
  GraphQLClient client,
  String entityType,
  String entityId,
) {
  client.mutate(MutationOptions(
    document: gql(_recordSelectionMutation),
    variables: {'entityType': entityType, 'entityId': entityId},
  ));
}

void recordSearch(GraphQLClient client, String entityType, String term) {
  client.mutate(MutationOptions(
    document: gql(_recordSearchMutation),
    variables: {'entityType': entityType, 'term': term},
  ));
}

void recordView(GraphQLClient client, String entityType, String entityId) {
  client.mutate(MutationOptions(
    document: gql(_recordViewMutation),
    variables: {'entityType': entityType, 'entityId': entityId},
  ));
}

/// Collapses rapid input (search typing) into a single trailing call.
class Debouncer {
  Debouncer({this.delay = const Duration(milliseconds: 400)});

  final Duration delay;
  Timer? _timer;

  void run(void Function() action) {
    _timer?.cancel();
    _timer = Timer(delay, action);
  }

  void dispose() => _timer?.cancel();
}
