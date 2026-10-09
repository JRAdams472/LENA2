import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/screens/edit_item_screen.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, Map<String, dynamic>> responses;

  _CaptureLink(this.responses);

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    yield Response(
      data: responses[_opName(request)] ?? <String, dynamic>{},
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

GraphQLClient _client(_CaptureLink link) => GraphQLClient(
      cache: GraphQLCache(),
      link: link,
      defaultPolicies: DefaultPolicies(
        query: Policies(fetch: FetchPolicy.noCache),
        mutate: Policies(fetch: FetchPolicy.noCache),
      ),
    );

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

Future<void> _open(
  WidgetTester tester,
  _CaptureLink link, {
  String? itemId,
}) async {
  await tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(_client(link)),
      child: MaterialApp(
        home: Builder(
          builder: (ctx) => Scaffold(
            body: TextButton(
              onPressed: () => Navigator.push(
                ctx,
                MaterialPageRoute(
                    builder: (_) => EditItemScreen(itemId: itemId)),
              ),
              child: const Text('open-form'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open-form'));
  await _settle(tester);
}

Map<String, dynamic> _catalogs() => {
      'Categories': {
        'categories': [
          {'id': 'cat-1', 'name': 'Produce'},
          {'id': 'cat-2', 'name': 'Dairy'},
        ],
      },
      'SearchBrands': {
        'searchBrands': [
          {'id': 'br-1', 'name': 'Acme'},
        ],
      },
    };

/// Two dropdowns, in order: Category, Brand.
DropdownButtonFormField<String?> _dropdown(WidgetTester tester, int index) =>
    tester
        .widgetList<DropdownButtonFormField<String?>>(
          find.byWidgetPredicate((w) => w is DropdownButtonFormField<String?>),
        )
        .elementAt(index);

void main() {
  testWidgets('create mode saves a fully-populated item', (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'CreateItem': {
        'createItem': {'id': 'i-new', 'name': 'Milk'},
      },
      'RecordSelection': {'recordSelection': true},
    });
    await _open(tester, link);

    expect(find.text('Create Item'), findsOneWidget);
    // The initial unfiltered brand search ran during load.
    expect(link.byName('SearchBrands'), isNotEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Name'), 'Milk');
    await tester.enterText(find.widgetWithText(TextField, 'Unit'), 'gallon');
    _dropdown(tester, 0).onChanged!('cat-2');
    _dropdown(tester, 1).onChanged!('br-1');
    await tester.enterText(
        find.widgetWithText(TextField, 'UPC-12 (optional)'), '012345678905');
    await tester.enterText(
        find.widgetWithText(TextField, 'UPC-14 (optional)'), '10012345678902');
    await _settle(tester);

    // Brand selection records telemetry.
    final selections = link.byName('RecordSelection');
    expect(selections, hasLength(1));
    expect(selections.single.variables['entityType'], 'brand');
    expect(selections.single.variables['entityId'], 'br-1');

    await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
    await _settle(tester);

    final creates = link.byName('CreateItem');
    expect(creates, hasLength(1));
    final input = creates.single.variables['input'] as Map<String, dynamic>;
    expect(input['name'], 'Milk');
    expect(input['unit'], 'gallon');
    expect(input['categoryId'], 'cat-2');
    expect(input['brandId'], 'br-1');
    expect(input['upc12'], '012345678905');
    expect(input['upc14'], '10012345678902');
    expect(find.text('open-form'), findsOneWidget);
  });

  testWidgets('empty optional fields serialize as null', (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'CreateItem': {
        'createItem': {'id': 'i-new', 'name': 'X'},
      },
    });
    await _open(tester, link);

    await tester.enterText(find.widgetWithText(TextField, 'Name'), 'Flour');
    await tester.enterText(find.widgetWithText(TextField, 'Unit'), 'kg');
    await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
    await _settle(tester);

    final input = link.byName('CreateItem').single.variables['input']
        as Map<String, dynamic>;
    expect(input['categoryId'], isNull);
    expect(input['brandId'], isNull);
    expect(input['upc12'], isNull);
    expect(input['upc14'], isNull);
  });

  testWidgets(
      'brand search debounces, records telemetry, and refreshes the list',
      (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'RecordSearch': {'recordSearch': true},
    });
    await _open(tester, link);

    await tester.enterText(
        find.widgetWithText(TextField, 'Search brands'), 'acme');
    await tester.pump(const Duration(milliseconds: 600));
    await _settle(tester);

    expect(
      link.byName('SearchBrands').map((r) => r.variables['term']),
      contains('acme'),
    );
    final records = link.byName('RecordSearch');
    expect(records.single.variables['entityType'], 'brand');
    expect(records.single.variables['term'], 'acme');
  });

  testWidgets('edit mode prefills the item and posts UpdateItem',
      (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'RecordView': {'recordView': true},
      'Item': {
        'item': {
          'id': 'i-1',
          'name': 'Whole Milk',
          'unit': 'gallon',
          'upc12': '011111111111',
          'upc14': '10011111111118',
          'brand': {'id': 'br-9', 'name': 'Store Brand'},
          'category': {'id': 'cat-1', 'name': 'Produce'},
        },
      },
      'UpdateItem': {
        'updateItem': {'id': 'i-1', 'name': 'Whole Milk'},
      },
    });
    await _open(tester, link, itemId: 'i-1');

    expect(find.text('Edit Item'), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.widgetWithText(TextField, 'Whole Milk'))
          .controller!
          .text,
      'Whole Milk',
    );
    expect(
      tester
          .widget<TextField>(find.widgetWithText(TextField, '011111111111'))
          .controller!
          .text,
      '011111111111',
    );
    expect(link.byName('RecordView').single.variables['entityId'], 'i-1');

    await tester.enterText(
        find.widgetWithText(TextField, 'Whole Milk'), 'Skim Milk');
    await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
    await _settle(tester);

    final updates = link.byName('UpdateItem');
    expect(updates, hasLength(1));
    expect(updates.single.variables['id'], 'i-1');
    expect(
      (updates.single.variables['input'] as Map)['name'],
      'Skim Milk',
    );
    expect(find.text('open-form'), findsOneWidget);
  });
}
