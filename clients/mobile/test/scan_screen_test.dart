import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/scan_screen.dart';

/// A real GraphQLClient whose HttpLink terminates at a MockClient.
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

Map<String, dynamic> _itemByUpc(Map<String, dynamic>? item) => {
      'data': {
        '__typename': 'Query',
        'itemByUpc': item,
      },
    };

Map<String, dynamic> _catalogItem({
  Map<String, dynamic>? ingredient,
  Map<String, dynamic>? householdIngredient,
}) =>
    {
      '__typename': 'Item',
      'id': '5',
      'name': 'Whole Milk',
      'brand': 'StoreBrand',
      'upc12': '012345678905',
      'upc14': null,
      'unit': 'gal',
      'category': {'__typename': 'Category', 'id': '3', 'name': 'Dairy'},
      'ingredient': ingredient,
      'householdIngredient': householdIngredient,
      'nutrients': [],
    };

/// Answers every operation the scan flow can send; each branch keys off the
/// operation text in the request body.
Map<String, dynamic> _scanResponder(Map<String, dynamic> body) {
  final q = body['query'] as String;
  if (q.contains('itemByUpc')) {
    return _itemByUpc(_catalogItem(
        ingredient: {'__typename': 'Ingredient', 'id': '7', 'name': 'milk'}));
  }
  if (q.contains('incrementUserItem')) {
    return {
      'data': {
        '__typename': 'Mutation',
        'incrementUserItem': {
          '__typename': 'UserItem',
          'id': '50',
          'currentQty': 4,
        },
      },
    };
  }
  if (q.contains('submitItem')) {
    return {
      'data': {
        '__typename': 'Mutation',
        'submitItem': {'__typename': 'Item', 'id': '60', 'name': 'New Item'},
      },
    };
  }
  if (q.contains('setItemNutrients')) {
    return {
      'data': {
        '__typename': 'Mutation',
        'setItemNutrients': [],
      },
    };
  }
  if (q.contains('ingredients')) {
    return {
      'data': {
        '__typename': 'Query',
        'ingredients': {
          '__typename': 'IngredientPage',
          'items': [
            {'__typename': 'Ingredient', 'id': '9', 'name': 'corn'},
          ],
        },
      },
    };
  }
  if (q.contains('setHouseholdItemIngredient')) {
    return {
      'data': {
        '__typename': 'Mutation',
        'setHouseholdItemIngredient': true,
      },
    };
  }
  return {'data': {}};
}

/// Stubs the mobile_scanner platform channels: the method channel answers
/// start/stop/permission calls, and the event channel's stream sink lets
/// tests inject barcode captures through the real onDetect pipeline.
class _ScannerHarness {
  MockStreamHandlerEventSink? _events;

  void install() {
    const method =
        MethodChannel('dev.steenbakker.mobile_scanner/scanner/method');
    const events = EventChannel('dev.steenbakker.mobile_scanner/scanner/event');
    const orientation = EventChannel(
        'dev.steenbakker.mobile_scanner/scanner/deviceOrientation');
    final messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;

    messenger.setMockMethodCallHandler(method, (call) async {
      switch (call.method) {
        case 'state':
          return 1; // authorized
        case 'request':
          return true;
        case 'start':
          return {
            'textureId': 0,
            'size': {'width': 100.0, 'height': 100.0},
            'cameraDirection': 0,
            'currentTorchState': 0,
            'numberOfCameras': 1,
          };
        default:
          return null;
      }
    });
    messenger.setMockStreamHandler(
      events,
      MockStreamHandler.inline(
        onListen: (arguments, sink) {
          _events = sink;
        },
      ),
    );
    messenger.setMockStreamHandler(
      orientation,
      MockStreamHandler.inline(onListen: (arguments, sink) {}),
    );

    addTearDown(() {
      messenger.setMockMethodCallHandler(method, null);
      messenger.setMockStreamHandler(events, null);
      messenger.setMockStreamHandler(orientation, null);
    });
  }

  void emitBarcode(String rawValue) {
    _events?.success({
      'name': 'barcode',
      'data': [
        {'rawValue': rawValue, 'format': 0, 'type': 0},
      ],
    });
  }

  /// Channel events hop through the real messenger codec — a plain pump is
  /// not enough, so runAsync lets the platform-side futures complete.
  Future<void> emitAndSettle(WidgetTester tester, String rawValue) async {
    emitBarcode(rawValue);
    await tester
        .runAsync(() => Future.delayed(const Duration(milliseconds: 50)));
    await tester.pumpAndSettle();
  }
}

Future<void> _pumpScan(WidgetTester tester, _MockGraphQL mock) {
  return tester.pumpWidget(
    GraphQLProvider(
      client: ValueNotifier(mock.client),
      child: const MaterialApp(home: ScanScreen()),
    ),
  );
}

void main() {
  group('resolvedIngredientOf', () {
    test('household override wins over the catalog link', () {
      final item = {
        'ingredient': {'id': '7', 'name': 'corn'},
        'householdIngredient': {'id': '9', 'name': 'maize'},
      };
      expect(resolvedIngredientOf(item)?['id'], '9');
      expect(resolvedIngredientOf(item)?['name'], 'maize');
    });

    test('falls back to the catalog ingredient', () {
      final item = {
        'ingredient': {'id': '7', 'name': 'corn'},
        'householdIngredient': null,
      };
      expect(resolvedIngredientOf(item)?['name'], 'corn');
    });

    test('returns null when neither link exists', () {
      final item = {'ingredient': null, 'householdIngredient': null};
      expect(resolvedIngredientOf(item), isNull);
    });
  });

  testWidgets('ScanScreen renders its app bar', (tester) async {
    await tester.pumpWidget(
      GraphQLProvider(
        client: ValueNotifier(graphQLClient),
        child: const MaterialApp(home: ScanScreen()),
      ),
    );
    await tester.pump();

    expect(find.text('Scan Item'), findsOneWidget);
  });

  group('scan flow', () {
    late _ScannerHarness scanner;
    setUp(() {
      scanner = _ScannerHarness()..install();
    });

    testWidgets('a detected barcode looks the item up', (tester) async {
      final mock = _MockGraphQL(_scanResponder);
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      final lookups = mock.matching('itemByUpc');
      expect(lookups, hasLength(1));
      expect((lookups.single['variables'] as Map)['code'], '012345678905');
      expect(find.text('Whole Milk'), findsOneWidget);
      expect(find.text('Brand: StoreBrand'), findsOneWidget);
      expect(find.text('Unit: gal'), findsOneWidget);
      expect(find.text('UPC: 012345678905'), findsOneWidget);
      expect(find.text('Ingredient: milk'), findsOneWidget);
    });

    testWidgets('an unsupported barcode shows an error', (tester) async {
      final mock = _MockGraphQL(_scanResponder);
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, 'ab');

      expect(find.textContaining('Unsupported barcode: ab'), findsOneWidget);
      expect(mock.matching('itemByUpc'), isEmpty);
    });

    testWidgets('a missed scan shows the submit form', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(null);
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      expect(find.text('Item not found'), findsOneWidget);
      expect(find.text('UPC: 012345678905'), findsOneWidget);
      expect(find.text('Submit for approval'), findsOneWidget);
    });

    testWidgets('submitting an unknown item calls submitItem', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(null);
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.enterText(
          find.widgetWithText(TextField, 'Item name'), 'New Item');
      await tester.enterText(
          find.widgetWithText(TextField, 'Unit (e.g., oz, lb)'), 'oz');
      await tester.enterText(
          find.widgetWithText(TextField, 'Category ID'), '3');
      await tester.ensureVisible(find.text('Submit for approval'));
      await tester.tap(find.text('Submit for approval'));
      await tester.pump();
      await tester.pump();

      final submits = mock.matching('submitItem');
      expect(submits, hasLength(1));
      final input = (submits.single['variables'] as Map)['input'] as Map;
      expect(input['name'], 'New Item');
      expect(input['unit'], 'oz');
      expect(input['categoryId'], '3');
      expect(input['upc12'], '012345678905');
      // The submit view is dismissed back to the scanner.
      expect(find.text('Item not found'), findsNothing);
      expect(find.text('Center a barcode in the camera view'), findsOneWidget);
      // The 2s confirmation timer resets to the scanner view.
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
    });

    testWidgets('submit validates required fields', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(null);
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.ensureVisible(find.text('Submit for approval'));
      await tester.tap(find.text('Submit for approval'));
      await tester.pump();

      expect(find.text('Please fill in name, unit, and category.'),
          findsOneWidget);
      expect(mock.matching('submitItem'), isEmpty);
    });

    testWidgets('nutrient rows attach a setItemNutrients call', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(null);
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.ensureVisible(find.text('Add nutrient'));
      await tester.tap(find.text('Add nutrient'));
      await tester.pump();

      await tester.enterText(
          find.widgetWithText(TextField, 'Nutrient ID'), '1008');
      await tester.enterText(find.widgetWithText(TextField, 'Amount'), '150');
      await tester.enterText(
          find.widgetWithText(TextField, 'Item name'), 'New Item');
      await tester.enterText(
          find.widgetWithText(TextField, 'Unit (e.g., oz, lb)'), 'oz');
      await tester.enterText(
          find.widgetWithText(TextField, 'Category ID'), '3');
      await tester.ensureVisible(find.text('Submit for approval'));
      await tester.tap(find.text('Submit for approval'));
      await tester.pump();
      await tester.pump();

      final nutrients = mock.matching('setItemNutrients');
      expect(nutrients, hasLength(1));
      final vars = nutrients.single['variables'] as Map;
      expect(vars['itemId'], '60');
      expect(vars['nutrients'], [
        {'nutrientId': '1008', 'amount': 150.0},
      ]);
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
    });

    testWidgets('add to pantry increments inventory then resets',
        (tester) async {
      final mock = _MockGraphQL(_scanResponder);
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.ensureVisible(find.text('Add to Pantry'));
      await tester.tap(find.text('Add to Pantry'));
      await tester.pump();
      await tester.pump();

      final increments = mock.matching('incrementUserItem');
      expect(increments, hasLength(1));
      final vars = increments.single['variables'] as Map;
      expect(vars['itemId'], '5');
      expect(vars['delta'], 1.0);
      expect(find.text('Whole Milk added to pantry.'), findsOneWidget);

      // After the 2s confirmation delay the scanner resets for the next item.
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
      expect(find.text('Whole Milk'), findsNothing);
      expect(find.text('Center a barcode in the camera view'), findsOneWidget);
    });

    testWidgets('remove from pantry decrements by the entered quantity',
        (tester) async {
      final mock = _MockGraphQL(_scanResponder);
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.enterText(find.widgetWithText(TextField, 'Quantity'), '2.5');
      await tester.ensureVisible(find.text('Remove from Pantry'));
      await tester.tap(find.text('Remove from Pantry'));
      await tester.pump();
      await tester.pump();

      final vars =
          mock.matching('incrementUserItem').single['variables'] as Map;
      expect(vars['delta'], -2.5);
      expect(find.text('Whole Milk removed to pantry.'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
    });

    testWidgets('household override displays with the household marker',
        (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(_catalogItem(
            ingredient: {'__typename': 'Ingredient', 'id': '7', 'name': 'milk'},
            householdIngredient: {
              '__typename': 'Ingredient',
              'id': '9',
              'name': '2% milk'
            },
          ));
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      expect(find.text('Ingredient: 2% milk (household)'), findsOneWidget);
    });

    testWidgets('link ingredient searches and binds via the override',
        (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(_catalogItem());
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.tap(find.text('Link ingredient'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('corn'));
      await tester.pumpAndSettle();

      final links = mock.matching('setHouseholdItemIngredient');
      expect(links, hasLength(1));
      final vars = links.single['variables'] as Map;
      expect(vars['itemId'], '5');
      expect(vars['ingredientId'], '9');
      expect(find.text('Linked to corn.'), findsOneWidget);
      expect(find.text('Ingredient: corn (household)'), findsOneWidget);
    });

    testWidgets('cancelling the link dialog sends no mutation', (tester) async {
      final mock = _MockGraphQL((body) {
        if ((body['query'] as String).contains('itemByUpc')) {
          return _itemByUpc(_catalogItem());
        }
        return _scanResponder(body);
      });
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.tap(find.text('Link ingredient'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();

      expect(mock.matching('setHouseholdItemIngredient'), isEmpty);
    });

    testWidgets('scan another resets to the scanner view', (tester) async {
      final mock = _MockGraphQL(_scanResponder);
      await _pumpScan(tester, mock);
      await tester.pumpAndSettle();

      await scanner.emitAndSettle(tester, '012345678905');

      await tester.ensureVisible(find.text('Scan another'));
      await tester.tap(find.text('Scan another'));
      await tester.pumpAndSettle();

      expect(find.text('Center a barcode in the camera view'), findsOneWidget);
      expect(find.text('Whole Milk'), findsNothing);
    });
  });
}
