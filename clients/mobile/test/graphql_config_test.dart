import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:lena_mobile/auth/auth_service.dart';
import 'package:lena_mobile/graphql_config.dart';

Request _request() => Request(
      operation: Operation(
        document: gql('query Probe { probe }'),
        operationName: 'Probe',
      ),
    );

Stream<Response> _errors(List<GraphQLError> errors) => Stream.value(
      Response(data: const {}, errors: errors, response: const {}),
    );

void main() {
  // Gives the global authService a real binding so its lazy init resolves
  // (secure storage is absent in tests — its exceptions are swallowed).
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    await authService.ready;
    // _init's storage restore races test listeners — let it finish first.
    await pumpEventQueue();
  });

  group('errorLink', () {
    test('passes clean responses through untouched', () async {
      final responses = await errorLink
          .request(
            _request(),
            (req) =>
                Stream.value(const Response(data: {'probe': 1}, response: {})),
          )
          .toList();
      expect(responses.single.data!['probe'], 1);
    });

    test('a GraphQL unauthenticated error signs out and passes through',
        () async {
      var notified = 0;
      void listener() => notified++;
      authService.addListener(listener);
      try {
        final responses = await errorLink
            .request(
              _request(),
              (req) =>
                  _errors(const [GraphQLError(message: 'Unauthenticated')]),
            )
            .toList();
        // Handler returns null — the original errored response still lands.
        expect(responses.single.errors, isNotEmpty);
        await pumpEventQueue();
        expect(notified, greaterThan(0));
      } finally {
        authService.removeListener(listener);
      }
    });

    test('an "unauthorized" wording also signs out', () async {
      var notified = 0;
      void listener() => notified++;
      authService.addListener(listener);
      try {
        await errorLink
            .request(
              _request(),
              (req) => _errors(
                  const [GraphQLError(message: 'Forbidden: unauthorized')]),
            )
            .toList();
        await pumpEventQueue();
        expect(notified, greaterThan(0));
      } finally {
        authService.removeListener(listener);
      }
    });

    test('non-auth GraphQL errors do not sign out', () async {
      var notified = 0;
      void listener() => notified++;
      authService.addListener(listener);
      try {
        final responses = await errorLink
            .request(
              _request(),
              (req) =>
                  _errors(const [GraphQLError(message: 'validation failed')]),
            )
            .toList();
        expect(responses.single.errors, isNotEmpty);
        await pumpEventQueue();
        expect(notified, 0);
      } finally {
        authService.removeListener(listener);
      }
    });

    test('HTTP 401 with no refresh token signs out and rethrows', () async {
      var notified = 0;
      void listener() => notified++;
      authService.addListener(listener);
      try {
        final stream = errorLink.request(
          _request(),
          (req) => Stream<Response>.error(const ServerException(
            statusCode: 401,
          )),
        );
        await expectLater(stream, emitsError(isA<ServerException>()));
        await pumpEventQueue();
        expect(notified, greaterThan(0));
      } finally {
        authService.removeListener(listener);
      }
    });

    test('a request already retried once is not replayed again', () async {
      final req = _request();
      Stream<Response> fail(Request r) => Stream<Response>.error(
            const ServerException(statusCode: 401),
          );
      // First pass marks the request and yields the handler's error stream.
      await expectLater(
          errorLink.request(req, fail), emitsError(isA<ServerException>()));
      // Second pass: _retriedRequests[req] is set → handler returns null and
      // the original error propagates.
      await expectLater(
          errorLink.request(req, fail), emitsError(isA<ServerException>()));
    });

    test('non-401 exceptions propagate unchanged', () async {
      await expectLater(
        errorLink.request(
          _request(),
          (req) =>
              Stream<Response>.error(const ServerException(statusCode: 500)),
        ),
        emitsError(isA<ServerException>()),
      );
    });

    group('403 approval gate', () {
      tearDown(() async {
        // The gate lives on the global authService — don't leak it into
        // the next test.
        await authService.signOut();
      });

      test('account_pending in the decoded body flags the gate', () async {
        await expectLater(
          errorLink.request(
            _request(),
            (req) => Stream<Response>.error(const ServerException(
              statusCode: 403,
              parsedResponse: Response(
                response: {'message': 'account_pending'},
              ),
            )),
          ),
          emitsError(isA<ServerException>()),
        );
        await pumpEventQueue();
        expect(authService.accountGate, 'pending');
      });

      test('account_denied read from the raw http body flags the gate',
          () async {
        await expectLater(
          errorLink.request(
            _request(),
            (req) => Stream<Response>.error(HttpLinkServerException(
              statusCode: 403,
              response: http.Response(
                '{"message": "account_denied"}',
                403,
              ),
              parsedResponse: const Response(response: {}),
            )),
          ),
          emitsError(isA<ServerException>()),
        );
        await pumpEventQueue();
        expect(authService.accountGate, 'denied');
      });

      test('a 403 without an account_* message does not flag the gate',
          () async {
        await expectLater(
          errorLink.request(
            _request(),
            (req) => Stream<Response>.error(const ServerException(
              statusCode: 403,
              parsedResponse: Response(
                response: {'message': 'Forbidden'},
              ),
            )),
          ),
          emitsError(isA<ServerException>()),
        );
        await pumpEventQueue();
        expect(authService.accountGate, isNull);
      });
    });
  });

  group('authLink', () {
    test('unsigned sessions send requests without a bearer', () async {
      Request? seen;
      final end = _ProbeLink((req) {
        seen = req;
        return Stream.value(const Response(data: {}, response: {}));
      });
      final chain = Link.from([authLink, end]);

      await chain.request(_request()).toList();

      expect(seen, isNotNull);
      // Signed-out global authService → getValidToken is null → no header.
      expect(
        seen!.context.entry<HttpLinkHeaders>()?.headers['authorization'],
        isNull,
      );
    });
  });

  test('graphQLClient wires the full production chain', () {
    expect(graphQLClient.link, isNotNull);
    expect(graphQLClient.cache, isA<GraphQLCache>());
  });
}

/// Terminal link that observes the request as composed by upstream links.
class _ProbeLink extends Link {
  _ProbeLink(this._fn);
  final Stream<Response> Function(Request) _fn;

  @override
  Stream<Response> request(Request request, [NextLink? forward]) =>
      _fn(request);
}
