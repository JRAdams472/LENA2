import 'package:graphql_flutter/graphql_flutter.dart';
import 'auth/auth_service.dart';
import 'idempotency_link.dart';

// lenaApiUrl now lives in auth_service.dart (session endpoints derive
// from it); re-exported here for existing callers.
export 'auth/auth_service.dart' show lenaApiUrl;

final AuthLink authLink = AuthLink(
  // getValidToken proactively rotates the refresh token when the access
  // token is expired, so requests rarely reach the wire with a dead token.
  getToken: () async {
    final token = await authService.getValidToken();
    return token == null ? null : 'Bearer $token';
  },
);

// Requests already retried once after a session refresh — Expando marks
// the same Request instance without growing unboundedly.
final _retriedRequests = Expando<bool>();

final ErrorLink errorLink = ErrorLink(
  onGraphQLError: (request, forward, response) {
    final errors = response.errors;
    if (errors != null) {
      final unauthorized = errors.any(
        (e) =>
            e.message.toLowerCase().contains('unauthenticated') ||
            e.message.toLowerCase().contains('unauthorized'),
      );
      if (unauthorized) {
        authService.signOut();
      }
    }
    return null;
  },
  // HTTP 401 with a live refresh token: rotate once and replay the
  // request; a failed rotation signs out.
  onException: (request, forward, exception) {
    if (exception is ServerException &&
        exception.statusCode == 401 &&
        _retriedRequests[request] == null) {
      _retriedRequests[request] = true;
      return Stream.fromFuture(authService.refreshSession()).asyncExpand((
        ok,
      ) {
        if (ok) return forward(request);
        authService.signOut();
        return Stream<Response>.error(exception);
      });
    }
    return null;
  },
);

final GraphQLClient graphQLClient = GraphQLClient(
  cache: GraphQLCache(),
  link: Link.from([
    errorLink,
    authLink,
    // Before HttpLink so the key reaches the wire; the server's dedup layer
    // replays retried mutations instead of re-executing them.
    IdempotencyLink(),
    HttpLink(lenaApiUrl),
  ]),
);
