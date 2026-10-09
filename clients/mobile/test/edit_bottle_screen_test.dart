import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/screens/edit_bottle_screen.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

typedef _Responder = Map<String, dynamic> Function(Request request);

class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, _Responder> responders;

  _CaptureLink(Map<String, Map<String, dynamic>> responses,
      {Map<String, _Responder> fns = const {}})
      : responders = {
          for (final e in responses.entries) e.key: (_) => e.value,
          ...fns,
        };

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    yield Response(
      data: responders[_opName(request)]?.call(request) ?? <String, dynamic>{},
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

/// Pushes the form from a stub home so save-pop is observable.
Future<void> _open(
  WidgetTester tester,
  _CaptureLink link, {
  String? bottleId,
}) async {
  // The form is taller than the default 800x600 surface — enlarge it so
  // every field is laid out without scrolling.
  tester.view.physicalSize = const Size(1200, 2600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
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
                    builder: (_) => EditBottleScreen(bottleId: bottleId)),
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
      'Types': {
        'types': [
          {'id': 't-1', 'name': 'Red'},
          {'id': 't-2', 'name': 'White'},
        ],
      },
      'Countries': {
        'countries': [
          {'id': 'c-1', 'name': 'France'},
          {'id': 'c-2', 'name': 'USA'},
        ],
      },
    };

/// The three dropdowns render in a fixed order: Type, Country, Region.
DropdownButtonFormField<String?> _dropdown(WidgetTester tester, int index) =>
    tester
        .widgetList<DropdownButtonFormField<String?>>(
          find.byWidgetPredicate((w) => w is DropdownButtonFormField<String?>),
        )
        .elementAt(index);

void main() {
  testWidgets('shows the skeleton until the catalogs land', (tester) async {
    // No responses → _types/_countries stay empty → SkeletonForm persists.
    final link = _CaptureLink({});
    await _open(tester, link);
    expect(find.text('Create Bottle'), findsOneWidget);
    expect(find.byType(DropdownButtonFormField<String?>), findsNothing);
  });

  testWidgets('create mode fills every field and posts CreateBottle',
      (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'CreateBottle': {
        'createBottle': {'id': 'b-new'},
      },
    });
    await _open(tester, link);

    _dropdown(tester, 0).onChanged!('t-2');
    _dropdown(tester, 1).onChanged!('c-2');
    await _settle(tester);
    // Country selected → Regions fired for it.
    expect(
      link.byName('Regions').map((r) => r.variables['countryId']),
      contains('c-2'),
    );

    await tester.enterText(
        find.widgetWithText(TextField, 'Vintage year'), '2012');
    await tester.enterText(
        find.widgetWithText(TextField, 'Bottle size'), '1.5L');
    await tester.enterText(
        find.widgetWithText(TextField, 'Vineyard'), 'Test Vineyard');
    await tester.enterText(find.widgetWithText(TextField, 'ABV'), '13.5');
    await tester.enterText(find.widgetWithText(TextField, 'Acidity'), '3');
    await tester.enterText(find.widgetWithText(TextField, 'Tannin level'), '4');
    await tester.enterText(find.widgetWithText(TextField, 'Body'), '2');
    await tester.enterText(find.widgetWithText(TextField, 'Sweetness'), '1');
    await tester.tap(find.widgetWithText(CheckboxListTile, 'Oak integration'));
    await tester.pump();

    await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
    await _settle(tester);

    final creates = link.byName('CreateBottle');
    expect(creates, hasLength(1));
    final input = creates.single.variables['input'] as Map<String, dynamic>;
    expect(input['typeId'], 't-2');
    expect(input['countryId'], 'c-2');
    expect(input['regionId'], isNull);
    expect(input['vintageYear'], 2012);
    expect(input['bottleSize'], '1.5L');
    expect(input['vineyard'], 'Test Vineyard');
    expect(input['abv'], 13.5);
    expect(input['acidity'], 3);
    expect(input['tanninLevel'], 4);
    expect(input['body'], 2);
    expect(input['sweetness'], 1);
    expect(input['oakIntegration'], isTrue);
    // Popped back to the stub home.
    expect(find.text('open-form'), findsOneWidget);
  });

  testWidgets('empty optional fields serialize as null', (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'CreateBottle': {
        'createBottle': {'id': 'b-new'},
      },
    });
    await _open(tester, link);

    await tester.enterText(
        find.widgetWithText(TextField, 'Vintage year'), 'abc');
    await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
    await _settle(tester);

    final input = link.byName('CreateBottle').single.variables['input']
        as Map<String, dynamic>;
    expect(input['vintageYear'], isNull);
    expect(input['vineyard'], isNull);
    expect(input['abv'], isNull);
    expect(input['bottleSize'], '750ml'); // controller default
    expect(input['oakIntegration'], isFalse);
  });

  testWidgets('edit mode prefills from the bottle query and updates',
      (tester) async {
    final link = _CaptureLink({
      ..._catalogs(),
      'RecordView': {'recordView': true},
      'Bottle': {
        'bottle': {
          'id': 'b-1',
          'typeId': 't-1',
          'countryId': 'c-1',
          'regionId': 'r-1',
          'vineyard': 'Chateau Margaux',
          'vintageYear': 2015,
          'bottleSize': '750ml',
          'abv': 13.0,
          'acidity': 4,
          'tanninLevel': 4,
          'body': 4,
          'sweetness': 1,
          'oakIntegration': true,
        },
      },
      'UpdateBottle': {
        'updateBottle': {'id': 'b-1'},
      },
    }, fns: {
      // Regions are per-country — c-1 has Bordeaux, c-2 has Napa.
      'Regions': (req) => {
            'regions': [
              req.variables['countryId'] == 'c-2'
                  ? {'id': 'r-2', 'name': 'Napa'}
                  : {'id': 'r-1', 'name': 'Bordeaux'},
            ],
          },
    });
    await _open(tester, link, bottleId: 'b-1');

    expect(find.text('Edit Bottle'), findsOneWidget);
    // Prefilled fields.
    expect(
      tester
          .widget<TextField>(find.widgetWithText(TextField, '2015'))
          .controller!
          .text,
      '2015',
    );
    expect(
      tester
          .widget<TextField>(find.widgetWithText(TextField, 'Chateau Margaux'))
          .controller!
          .text,
      'Chateau Margaux',
    );
    // RecordView fired for the opened bottle.
    expect(link.byName('RecordView').single.variables['entityId'], 'b-1');
    // Regions loaded for the prefill country.
    expect(
      link.byName('Regions').map((r) => r.variables['countryId']),
      contains('c-1'),
    );

    // Switch country → region clears and reloads for the new country.
    _dropdown(tester, 1).onChanged!('c-2');
    await _settle(tester);
    expect(
      link.byName('Regions').map((r) => r.variables['countryId']),
      contains('c-2'),
    );
    // Pick the new region + edit a field.
    _dropdown(tester, 2).onChanged!('r-2');
    await tester.enterText(find.widgetWithText(TextField, '2015'), '2016');

    await tester.tap(find.widgetWithText(ElevatedButton, 'Save'));
    await _settle(tester);

    final updates = link.byName('UpdateBottle');
    expect(updates, hasLength(1));
    expect(updates.single.variables['id'], 'b-1');
    final input = updates.single.variables['input'] as Map<String, dynamic>;
    expect(input['countryId'], 'c-2');
    expect(input['regionId'], 'r-2');
    expect(input['vintageYear'], 2016);
    expect(find.text('open-form'), findsOneWidget);
  });
}
