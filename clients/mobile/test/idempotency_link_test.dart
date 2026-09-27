import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:uuid/uuid.dart';
import 'package:lena_mobile/idempotency_link.dart';

Request _request(String document) => Request(
      operation: Operation(document: gql(document)),
    );

Stream<Response> _noopForward(Request _) => const Stream.empty();

void main() {
  group('IdempotencyLink', () {
    test('adds Idempotency-Key header to mutations', () async {
      Request? captured;
      final link = IdempotencyLink();
      await link.request(
        _request('mutation { toggleGroceryItemChecked(id: "1") { id } }'),
        (req) {
          captured = req;
          return _noopForward(req);
        },
      ).drain();

      final headers = captured!.context.entry<HttpLinkHeaders>()?.headers ?? {};
      expect(headers['Idempotency-Key'], isNotNull);
      expect(headers['Idempotency-Key'], isNotEmpty);
    });

    test('does not add the header to queries', () async {
      Request? captured;
      final link = IdempotencyLink();
      await link.request(
        _request('query { me { id } }'),
        (req) {
          captured = req;
          return _noopForward(req);
        },
      ).drain();

      final headers = captured!.context.entry<HttpLinkHeaders>()?.headers ?? {};
      expect(headers.containsKey('Idempotency-Key'), isFalse);
    });

    test('generates a fresh key per operation', () async {
      final keys = <String>[];
      final link = IdempotencyLink();
      for (var i = 0; i < 2; i++) {
        await link.request(
          _request('mutation { toggleGroceryItemChecked(id: "1") { id } }'),
          (req) {
            keys.add(req.context
                    .entry<HttpLinkHeaders>()
                    ?.headers['Idempotency-Key'] ??
                '');
            return _noopForward(req);
          },
        ).drain();
      }
      expect(keys[0], isNot(keys[1]));
    });

    test('respects operationName when selecting the operation type', () async {
      Request? captured;
      final link = IdempotencyLink();
      await link.request(
        Request(
          operation: Operation(
            document: gql('query A { me { id } } mutation B { noop }'),
            operationName: 'A',
          ),
        ),
        (req) {
          captured = req;
          return _noopForward(req);
        },
      ).drain();

      final headers = captured!.context.entry<HttpLinkHeaders>()?.headers ?? {};
      expect(headers.containsKey('Idempotency-Key'), isFalse);
    });

    test('preserves existing headers while adding the key', () async {
      Request? captured;
      final link = IdempotencyLink();
      final base =
          _request('mutation { noop }').updateContextEntry<HttpLinkHeaders>(
        (_) => const HttpLinkHeaders(headers: {'X-Other': 'keep'}),
      );
      await link.request(base, (req) {
        captured = req;
        return _noopForward(req);
      }).drain();

      final headers = captured!.context.entry<HttpLinkHeaders>()?.headers ?? {};
      expect(headers['X-Other'], 'keep');
      expect(headers['Idempotency-Key'], isNotNull);
    });

    test('injected uuid determines the key', () async {
      Request? captured;
      final link = IdempotencyLink(uuid: const Uuid());
      await link.request(_request('mutation { noop }'), (req) {
        captured = req;
        return _noopForward(req);
      }).drain();

      final key = captured!.context
          .entry<HttpLinkHeaders>()
          ?.headers['Idempotency-Key'];
      expect(key, matches(RegExp(r'^[0-9a-f-]{36}$')));
    });
  });
}
