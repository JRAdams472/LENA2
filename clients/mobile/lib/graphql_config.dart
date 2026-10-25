import 'dart:convert';

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
    // Approval barrier (LEN-127): a 403 carrying account_pending /
    // account_denied flips the app to the gate screen. The credential
    // stays valid — this is not a sign-out. `parsedResponse.data` is
    // GraphQL data — null for the BFF's plain {"message": ...} body —
    // so read the decoded raw body (or the http.Response directly).
    if (exception is ServerException && exception.statusCode == 403) {
      final message = _exceptionBodyMessage(exception);
      if (message != null &&
          message.startsWith('account_') &&
          message.length > 'account_'.length) {
        authService.flagAccountGate(message.substring('account_'.length));
      }
    }
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

/// Extracts `{"message": ...}` from a failed HTTP response — first the
/// already-decoded raw body (`parsedResponse.response`), then the wire
/// bytes for bodies that never made it through the response parser.
String? _exceptionBodyMessage(ServerException exception) {
  final decoded = exception.parsedResponse?.response['message'];
  if (decoded is String) return decoded;
  if (exception is HttpLinkServerException) {
    try {
      final body = jsonDecode(exception.response.body);
      if (body is Map && body['message'] is String) {
        return body['message'] as String;
      }
    } catch (_) {
      // Non-JSON error body (proxy page, empty body) — no gate message.
    }
  }
  return null;
}

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
