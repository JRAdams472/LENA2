import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/widgets/search_picker.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

/// Link that answers each request from a FIFO of prepared responses (or
/// errors), so tests control exactly when and what every round-trip
/// resolves to — required for stale-response ordering.
class _QueueLink extends Link {
  final List<Completer<Response>> queue = [];
  final List<Request> requests = [];

  @override
  Stream<Response> request(Request request, [NextLink? forward]) {
    requests.add(request);
    final c = Completer<Response>();
    queue.add(c);
    return Stream.fromFuture(c.future);
  }

  /// Completers for requests to `opName`, in issue order — analytics
  /// mutations share the link, so positional indexing is wrong.
  List<Completer<Response>> forOp(String opName) => [
        for (var i = 0; i < requests.length; i++)
          if (_opName(requests[i]) == opName) queue[i],
      ];

  /// Completes every still-open request with an empty payload — fire-and-
  /// forget calls (e.g. recordSearch) otherwise keep stream-timeout timers
  /// pending past teardown.
  void drain() {
    for (final c in queue) {
      if (!c.isCompleted) {
        c.complete(const Response(data: {}, response: {}));
      }
    }
  }
}

GraphQLClient _client(Link link) => GraphQLClient(
      cache: GraphQLCache(),
      link: link,
      defaultPolicies: DefaultPolicies(
        query: Policies(fetch: FetchPolicy.noCache),
        mutate: Policies(fetch: FetchPolicy.noCache),
      ),
    );

Response _page(List<Map<String, dynamic>> items) => Response(
      data: {
        'things': {'items': items}
      },
      response: const {},
    );

Response _flat(List<Map<String, dynamic>> items) => Response(
      data: {'categories': items},
      response: const {},
    );

const _connDoc = r'''
  query Things($search: String) {
    things(search: $search) { items { id name } }
  }
''';

const _flatDoc = r'''
  query Categories { categories { id name } }
''';

Future<void> _pumpSheet(
  WidgetTester tester,
  Link link, {
  bool isFlatList = false,
  String connectionField = 'things',
  String document = _connDoc,
}) async {
  await tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(_client(link)),
      child: MaterialApp(
        home: Scaffold(
          body: SearchPickerSheet(
            title: 'Pick',
            document: document,
            connectionField: connectionField,
            variablesFor: (term) => {'search': term.isEmpty ? null : term},
            itemLabel: (i) => i['name'] as String,
            searchEntityType: 'thing',
            isFlatList: isFlatList,
            onSelected: (_) {},
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  testWidgets('a query error renders retry instead of "No matches"',
      (tester) async {
    final link = _QueueLink();
    await _pumpSheet(tester, link);

    link.queue.single.completeError(const ServerException(
      parsedResponse: null,
      statusCode: 500,
    ));
    await tester.pump();

    expect(find.textContaining('Could not load'), findsOneWidget);
    expect(find.text('No matches.'), findsNothing);
    expect(find.text('Retry'), findsOneWidget);
  });

  testWidgets('a clean empty result still shows "No matches"', (tester) async {
    final link = _QueueLink();
    await _pumpSheet(tester, link);

    link.queue.single.complete(_page(const []));
    await tester.pump();

    expect(find.text('No matches.'), findsOneWidget);
    expect(find.textContaining('Could not load'), findsNothing);
  });

  testWidgets('a stale response cannot overwrite a newer search',
      (tester) async {
    final link = _QueueLink();
    await _pumpSheet(tester, link);

    // Initial load.
    link.forOp('Things')[0].complete(_page(const [
          {'id': '1', 'name': 'first'}
        ]));
    await tester.pump();

    // Two searches in flight: 'a' then 'ab'.
    await tester.enterText(find.byType(TextField), 'a');
    await tester.pump(const Duration(milliseconds: 450));
    await tester.enterText(find.byType(TextField), 'ab');
    await tester.pump(const Duration(milliseconds: 450));
    final things = link.forOp('Things');
    expect(things, hasLength(3));

    // Newest resolves first — 'fresh' lands.
    things[2].complete(_page(const [
      {'id': '2', 'name': 'fresh'}
    ]));
    await tester.pump();
    expect(find.text('fresh'), findsOneWidget);

    // The older 'a' response arrives late — the request token must drop it.
    things[1].complete(_page(const [
      {'id': '3', 'name': 'stale'}
    ]));
    await tester.pump();
    expect(find.text('fresh'), findsOneWidget);
    expect(find.text('stale'), findsNothing);

    // Settle the fire-and-forget recordSearch mutations before teardown.
    link.drain();
    await tester.pump();
  });

  testWidgets('flat-list mode fetches once and filters client-side',
      (tester) async {
    final link = _QueueLink();
    await _pumpSheet(
      tester,
      link,
      isFlatList: true,
      connectionField: 'categories',
      document: _flatDoc,
    );

    link.queue.single.complete(_flat(const [
      {'id': '1', 'name': 'Dairy'},
      {'id': '2', 'name': 'Produce'},
      {'id': '3', 'name': 'Dry Goods'},
    ]));
    await tester.pump();
    expect(find.text('Dairy'), findsOneWidget);
    expect(find.text('Produce'), findsOneWidget);

    // Typing filters the loaded vocabulary — no second request.
    await tester.enterText(find.byType(TextField), 'dair');
    await tester.pump(const Duration(milliseconds: 450));
    await tester.pump();

    expect(find.text('Dairy'), findsOneWidget);
    expect(find.text('Produce'), findsNothing);
    expect(link.requests, hasLength(1));
  });
}
