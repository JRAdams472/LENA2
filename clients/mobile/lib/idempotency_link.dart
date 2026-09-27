import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:uuid/uuid.dart';

/// Adds an `Idempotency-Key` header to mutation operations so the server's
/// dedup layer can replay a retry instead of re-executing it. Each operation
/// instance gets its own key — retries that reuse the same Request reuse the
/// same key, which is exactly what the dedup contract wants.
class IdempotencyLink extends Link {
  IdempotencyLink({Uuid? uuid}) : _uuid = uuid ?? const Uuid();

  final Uuid _uuid;
  static const _header = 'Idempotency-Key';

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    assert(forward != null, 'IdempotencyLink must not be the last link');
    if (request.operation.getOperationType() == OperationType.mutation) {
      request = request.updateContextEntry<HttpLinkHeaders>(
        (headers) => HttpLinkHeaders(
          headers: {
            ...?headers?.headers,
            _header: _uuid.v4(),
          },
        ),
      );
    }
    yield* forward!(request);
  }
}
