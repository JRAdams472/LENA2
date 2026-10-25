import 'package:graphql_flutter/graphql_flutter.dart';

/// Runs a mutation and throws [QueryResult.exception] when it carries
/// one. `client.mutate` resolves failures instead of throwing, so a
/// discarded result makes a failed mutation look successful — screens
/// pop and snackbars claim "saved" on writes that never landed. Every
/// mutation goes through this unless the caller inspects the result
/// itself (partial-data handling, error-code branching).
Future<QueryResult<Object?>> mutateOrThrow(
  GraphQLClient client,
  MutationOptions<Object?> options,
) async {
  final result = await client.mutate(options);
  if (result.hasException) {
    throw result.exception ?? Exception('mutation failed');
  }
  return result;
}
