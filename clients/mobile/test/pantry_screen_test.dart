import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/screens/pantry_screen.dart';
import 'package:lena_mobile/widgets/paged_list_view.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

/// A real GraphQLClient whose HttpLink terminates at a MockClient — the
/// Query widget and client.mutate run their normal pipelines.
class _MockGraphQL {
  _MockGraphQL(this.responder);
  final Map<String, dynamic> Function(Map<String, dynamic> body) responder;
  final requests = <Map<String, dynamic>>[];
  late final client = GraphQLClient(
    cache: GraphQLCache(),
    link: HttpLink(
      'http://test/graphql',
      httpClient: http_testing.MockClient((req) async {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        requests.add(body);
        return http.Response(
          jsonEncode(responder(body)),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    ),
  );

  List<Map<String, dynamic>> matching(String needle) =>
      requests.where((b) => (b['query'] as String).contains(needle)).toList();
}

Map<String, dynamic> _userItem(
  String id,
  String name, {
  num qty = 1,
  num? min,
  String? notes,
}) =>
    {
      '__typename': 'UserItem',
      'id': id,
      'item': {'__typename': 'Item', 'name': name},
      'currentQty': qty,
      'minQty': min,
      'notes': notes,
    };

Map<String, dynamic> _pantryResponse(
  List<Map<String, dynamic>> items, {
  int? totalCount,
  int pageNumber = 1,
}) =>
    {
      'data': {
        '__typename': 'Query',
        'userItems': {
          '__typename': 'UserItemPage',
          'items': items,
          'pageInfo': {
            '__typename': 'PageInfo',
            'totalCount': totalCount ?? items.length,
            'pageNumber': pageNumber,
          },
        },
      },
    };

Future<void> _pump(WidgetTester tester, _MockGraphQL mock) {
  return tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(mock.client),
      child: const MaterialApp(home: PantryScreen()),
    ),
  );
}

void main() {
  testWidgets('shows the skeleton while loading', (tester) async {
    final mock = _MockGraphQL((_) => _pantryResponse([_userItem('1', 'Milk')]));
    await _pump(tester, mock);
    expect(find.byType(SkeletonList), findsOneWidget);
    await tester.pumpAndSettle();
  });

  testWidgets('shows the error when the query fails', (tester) async {
    final mock = _MockGraphQL((_) => {
          'errors': [
            {'message': 'boom'},
          ],
        });
    await _pump(tester, mock);
    await tester.pumpAndSettle();
    expect(find.textContaining('Error:'), findsOneWidget);
  });

  testWidgets('shows the empty state when the pantry is empty', (tester) async {
    final mock = _MockGraphQL((_) => _pantryResponse(const []));
    await _pump(tester, mock);
    await tester.pumpAndSettle();
    expect(find.text('Your pantry is empty'), findsOneWidget);
  });

  testWidgets('renders quantity, minimum, and notes', (tester) async {
    final mock = _MockGraphQL((_) => _pantryResponse([
          _userItem('1', 'Milk', qty: 2, min: 1, notes: 'Whole only'),
          _userItem('2', 'Eggs', qty: 12),
        ]));
    await _pump(tester, mock);
    await tester.pumpAndSettle();
    expect(find.text('Milk'), findsOneWidget);
    expect(find.text('Eggs'), findsOneWidget);
    expect(find.textContaining('Qty: 2'), findsOneWidget);
    expect(find.textContaining('(min: 1)'), findsOneWidget);
    expect(find.textContaining('Whole only'), findsOneWidget);
  });

  testWidgets('debounced search sends variables and records the search',
      (tester) async {
    final mock = _MockGraphQL((body) {
      if ((body['query'] as String).contains('recordSearch')) {
        return {
          'data': {'__typename': 'Mutation', 'recordSearch': true},
        };
      }
      return _pantryResponse([_userItem('1', 'Milk')]);
    });
    await _pump(tester, mock);
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField), 'mil');
    // Debouncer is a 400ms trailing timer — advance past it.
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pumpAndSettle();

    final searches = mock.matching('userItems').where((b) {
      return (b['variables'] as Map)['search'] == 'mil';
    });
    expect(searches, isNotEmpty);

    final recorded = mock.matching('recordSearch');
    expect(recorded, hasLength(1));
    expect((recorded.single['variables'] as Map)['term'], 'mil');
    expect((recorded.single['variables'] as Map)['entityType'], 'item');
  });

  testWidgets('load-more fetches the next page', (tester) async {
    final page1 = [
      for (var i = 0; i < 25; i++) _userItem('$i', 'Item $i'),
    ];
    final mock = _MockGraphQL((body) {
      final vars = body['variables'] as Map<String, dynamic>;
      if (vars['page'] == 2) {
        return _pantryResponse([_userItem('25', 'Item 25')],
            totalCount: 26, pageNumber: 2);
      }
      return _pantryResponse(page1, totalCount: 26);
    });
    await _pump(tester, mock);
    await tester.pumpAndSettle();
    if (mock.matching('userItems').length == 1) {
      await tester.drag(find.byType(PagedListView), const Offset(0, -6000));
      await tester.pumpAndSettle();
    }
    final pages = mock
        .matching('userItems')
        .map((b) => (b['variables'] as Map)['page'])
        .toSet();
    expect(pages, contains(2));
    expect(find.text('Item 25'), findsOneWidget);
  });
}
