import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/edit_event_screen.dart';
import 'package:lena_mobile/screens/event_detail_screen.dart';
import 'package:lena_mobile/screens/event_timeline_screen.dart';
import 'package:lena_mobile/screens/events_screen.dart';
import 'package:lena_mobile/widgets/empty_state.dart';
import 'package:lena_mobile/widgets/paged_list_view.dart';
import 'package:lena_mobile/widgets/skeleton.dart';

/// The normalized cache drops objects without `__typename`; inject one
/// per object keyed by the field that produced it.
String _typename(String key) {
  var s = key;
  if (s.endsWith('ies')) {
    s = '${s.substring(0, s.length - 3)}y';
  } else if (s.endsWith('s')) {
    s = s.substring(0, s.length - 1);
  }
  return s[0].toUpperCase() + s.substring(1);
}

dynamic _typed(String key, dynamic value) {
  if (value is Map) {
    final out = <String, dynamic>{'__typename': _typename(key)};
    for (final e in value.entries) {
      out[e.key as String] = _typed(e.key as String, e.value);
    }
    return out;
  }
  if (value is List) {
    return value.map((v) => _typed(key, v)).toList();
  }
  return value;
}

Map<String, dynamic> _withTypenames(Map<String, dynamic> data) {
  final out = <String, dynamic>{'__typename': 'Query'};
  for (final e in data.entries) {
    out[e.key] = _typed(e.key, e.value);
  }
  return out;
}

http.Response gqlOk(Map<String, dynamic> data) => http.Response(
      jsonEncode({'data': _withTypenames(data)}),
      200,
      headers: {'content-type': 'application/json'},
    );

http.Response gqlErr() => http.Response(
      jsonEncode({
        'errors': [
          {'message': 'boom'}
        ]
      }),
      200,
      headers: {'content-type': 'application/json'},
    );

Map<String, dynamic> eventItem({
  String id = 'e1',
  String name = 'Friendsgiving',
  String eventDate = '2026-11-26',
  int granularity = 15,
  bool isActive = true,
}) =>
    {
      'id': id,
      'name': name,
      'eventDate': eventDate,
      'slotGranularityMinutes': granularity,
      'isActive': isActive,
    };

Map<String, dynamic> eventDetail({
  String id = 'e1',
  String name = 'Friendsgiving',
  String eventDate = '2026-11-26',
  int granularity = 15,
  bool isActive = true,
  List<Map<String, dynamic>>? recipes,
}) =>
    {
      'id': id,
      'name': name,
      'eventDate': eventDate,
      'slotGranularityMinutes': granularity,
      'isActive': isActive,
      'recipes': recipes ?? [],
    };

Map<String, dynamic> eventSlot({
  String id = 's1',
  String? recipeId = 'r1',
  String? recipeName = 'Roast Turkey',
  String mealType = 'dinner',
  String targetTime = '2026-11-26T18:00:00Z',
  int? servings = 8,
  int? baseServings = 4,
  double scalingFactor = 2.0,
  String? notes,
}) =>
    {
      'id': id,
      'mealType': mealType,
      'targetTime': targetTime,
      'servings': servings,
      'baseServings': baseServings,
      'scalingFactor': scalingFactor,
      'notes': notes,
      'recipe': recipeId == null ? null : {'id': recipeId, 'name': recipeName},
      'allergyWarnings': [],
      'allergens': [],
    };

/// Captures requests and answers with per-test overrides (`onRequest`)
/// ahead of canned mutation/query responses.
class _Capture {
  final List<Map<String, String>> requests = [];

  /// First crack at every request; return null to fall through.
  http.Response? Function(String query)? onRequest;

  /// Fallback for unmatched queries (mutations are canned below).
  http.Response? Function(String query)? onQuery;

  Future<http.Response> responder(http.Request req) async {
    final body = jsonDecode(utf8.decode(req.bodyBytes)) as Map<String, dynamic>;
    final query = body['query'] as String;
    requests.add({'query': query, 'variables': jsonEncode(body['variables'])});
    final custom = onRequest?.call(query);
    if (custom != null) return custom;
    return defaults(query);
  }

  http.Response defaults(String query) {
    if (query.contains('recordSelection')) {
      return gqlOk({'recordSelection': true});
    }
    if (query.contains('recordSearch')) {
      return gqlOk({'recordSearch': true});
    }
    if (query.contains('deleteFoodEvent')) {
      return gqlOk({'deleteFoodEvent': true});
    }
    if (query.contains('addEventRecipe')) {
      return gqlOk({
        'addEventRecipe': {'id': 's9'}
      });
    }
    if (query.contains('updateEventRecipe')) {
      return gqlOk({
        'updateEventRecipe': {'id': 's1'}
      });
    }
    if (query.contains('removeEventRecipe')) {
      return gqlOk({'removeEventRecipe': true});
    }
    if (query.contains('createFoodEvent')) {
      return gqlOk({
        'createFoodEvent': {'id': 'e9'}
      });
    }
    if (query.contains('updateFoodEvent')) {
      return gqlOk({
        'updateFoodEvent': {'id': 'e1'}
      });
    }
    return onQuery?.call(query) ?? gqlOk(const {});
  }

  int countOf(String fragment) =>
      requests.where((r) => r['query']!.contains(fragment)).length;

  Map<String, String>? lastMatching(String fragment) {
    final hits = requests.where((r) => r['query']!.contains(fragment));
    return hits.isEmpty ? null : hits.last;
  }
}

GraphQLClient _makeClient(_Capture cap) => GraphQLClient(
      link: HttpLink('https://example.test/graphql',
          httpClient: MockClient(cap.responder)),
      cache: GraphQLCache(),
    );

Widget _harness(_Capture cap, Widget child) => GraphQLProvider(
      client: ValueNotifier(_makeClient(cap)),
      child: MaterialApp(home: child),
    );

/// Host page so pushed screens have somewhere to pop back to.
class _PushHost extends StatelessWidget {
  final WidgetBuilder builder;
  const _PushHost({required this.builder});

  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(
          child: TextButton(
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: builder),
            ),
            child: const Text('go'),
          ),
        ),
      );
}

void main() {
  testWidgets(
    'EventsScreen renders its app bar while the query loads',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: EventsScreen()),
        ),
      );
      await tester.pump();

      expect(find.text('Events'), findsOneWidget);
    },
  );

  group('EventsScreen', () {
    testWidgets('shows a loading skeleton while the query runs',
        (tester) async {
      final cap = _Capture()
        ..onQuery = (query) => query.contains('foodEvents(')
            ? gqlOk({
                'foodEvents': {
                  'items': [eventItem()],
                  'pageInfo': {'totalCount': 1}
                }
              })
            : gqlOk(const {});
      // Checked right after pumpWidget — the HTTP future has not completed
      // during the synchronous first build, so the skeleton is still up.
      await tester.pumpWidget(_harness(cap, const EventsScreen()));
      expect(find.byType(SkeletonList), findsOneWidget);
      await tester.pumpAndSettle();
      expect(find.text('Friendsgiving'), findsOneWidget);
    });

    testWidgets('shows an error when the query fails', (tester) async {
      final cap = _Capture()..onRequest = (_) => gqlErr();
      await tester.pumpWidget(_harness(cap, const EventsScreen()));
      await tester.pumpAndSettle();
      expect(find.textContaining('Error:'), findsOneWidget);
    });

    testWidgets('shows the empty state with no events', (tester) async {
      final cap = _Capture()
        ..onQuery = (_) => gqlOk({
              'foodEvents': {
                'items': const [],
                'pageInfo': {'totalCount': 0}
              }
            });
      await tester.pumpWidget(_harness(cap, const EventsScreen()));
      await tester.pumpAndSettle();
      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('No events yet'), findsOneWidget);
    });

    testWidgets('renders events with active chips and granularity',
        (tester) async {
      final cap = _Capture()
        ..onQuery = (_) => gqlOk({
              'foodEvents': {
                'items': [
                  eventItem(),
                  eventItem(
                      id: 'e2',
                      name: 'Quiet Lunch',
                      isActive: false,
                      granularity: 30),
                ],
                'pageInfo': {'totalCount': 2}
              }
            });
      await tester.pumpWidget(_harness(cap, const EventsScreen()));
      await tester.pumpAndSettle();
      expect(find.text('Friendsgiving'), findsOneWidget);
      expect(find.text('2026-11-26 · 15-min schedule'), findsOneWidget);
      expect(find.text('Active'), findsOneWidget);
      expect(find.text('Inactive'), findsOneWidget);
      expect(find.text('Quiet Lunch'), findsOneWidget);
      expect(find.textContaining('30-min schedule'), findsOneWidget);
    });

    testWidgets('loads the next page via Load more', (tester) async {
      final cap = _Capture();
      cap.onRequest = (query) {
        if (query.contains('foodEvents(') &&
            cap.requests.last['variables']!.contains('"page":2')) {
          return gqlOk({
            'foodEvents': {
              'items': [eventItem(id: 'e99', name: 'Page Two Event')],
              'pageInfo': {'totalCount': 51}
            }
          });
        }
        if (query.contains('foodEvents(')) {
          return gqlOk({
            'foodEvents': {
              'items': List.generate(
                  50, (i) => eventItem(id: 'e$i', name: 'Event $i')),
              'pageInfo': {'totalCount': 51}
            }
          });
        }
        return null;
      };
      await tester.pumpWidget(_harness(cap, const EventsScreen()));
      await tester.pumpAndSettle();
      // PagedListView auto-loads near the bottom — drag to the end.
      await tester.drag(find.byType(PagedListView), const Offset(0, -6000));
      await tester.pumpAndSettle();
      final pages = cap.requests
          .where((r) => r['query']!.contains('foodEvents('))
          .map((r) => jsonDecode(r['variables']!)['page'])
          .toSet();
      expect(pages, contains(2));
      expect(find.text('Page Two Event'), findsOneWidget);
    });

    testWidgets('FAB pushes EditEventScreen and refreshes on return',
        (tester) async {
      final cap = _Capture()
        ..onQuery = (query) => query.contains('foodEvents(')
            ? gqlOk({
                'foodEvents': {
                  'items': [eventItem()],
                  'pageInfo': {'totalCount': 1}
                }
              })
            : gqlOk(const {});
      await tester.pumpWidget(_harness(cap, const EventsScreen()));
      await tester.pumpAndSettle();
      final before = cap.countOf('foodEvents(');
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();
      expect(find.byType(EditEventScreen), findsOneWidget);
      expect(find.text('New Event'), findsOneWidget);
      Navigator.of(tester.element(find.byType(EditEventScreen))).pop();
      await tester.pumpAndSettle();
      expect(cap.countOf('foodEvents('), greaterThan(before));
    });
  });

  group('EventDetailScreen', () {
    _Capture detailCap({
      List<Map<String, dynamic>>? recipes,
      bool notFound = false,
    }) =>
        _Capture()
          ..onQuery = (query) {
            if (query.contains('foodEvent(')) {
              return gqlOk({
                'foodEvent': notFound ? null : eventDetail(recipes: recipes),
              });
            }
            if (query.contains('recipes(')) {
              return gqlOk({
                'recipes': {
                  'items': [
                    {'id': 'r7', 'name': 'Sheet Pan Chicken'}
                  ]
                }
              });
            }
            return gqlOk(const {});
          };

    Future<void> pumpDetail(WidgetTester tester, _Capture cap) async {
      await tester.pumpWidget(_harness(
        cap,
        _PushHost(
          builder: (_) => const EventDetailScreen(foodEventId: 'e1'),
        ),
      ));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
    }

    testWidgets('shows the error state and retries', (tester) async {
      var calls = 0;
      final cap = _Capture()
        ..onQuery = (query) {
          if (query.contains('foodEvent(')) {
            calls++;
            return calls == 1
                ? gqlOk({'foodEvent': null})
                : gqlOk({'foodEvent': eventDetail()});
          }
          return gqlOk(const {});
        };
      await pumpDetail(tester, cap);
      expect(find.text('Error: Event not found.'), findsOneWidget);
      await tester.tap(find.text('Retry'));
      await tester.pumpAndSettle();
      expect(find.text('Friendsgiving'), findsWidgets);
      expect(find.text('2026-11-26'), findsOneWidget);
    });

    testWidgets('renders dishes with servings, scale, and notes',
        (tester) async {
      await pumpDetail(
        tester,
        detailCap(recipes: [
          eventSlot(notes: 'Brine overnight'),
          eventSlot(
            id: 's2',
            recipeId: null,
            mealType: 'snack',
            targetTime: '2026-11-26T15:00:00Z',
            servings: null,
            baseServings: null,
            scalingFactor: 1.0,
          ),
        ]),
      );
      expect(find.text('Roast Turkey'), findsOneWidget);
      expect(find.textContaining('serve 18:00 · dinner'), findsOneWidget);
      expect(find.textContaining('8 servings · ×2 of 4'), findsOneWidget);
      expect(find.textContaining('Brine overnight'), findsOneWidget);
      expect(find.text('Free-form dish'), findsOneWidget);
      expect(find.text('15-min schedule'), findsOneWidget);
      expect(find.text('View cooking timeline'), findsOneWidget);
    });

    testWidgets('shows the empty-dishes card', (tester) async {
      await pumpDetail(tester, detailCap());
      expect(find.text('No dishes yet'), findsOneWidget);
    });

    testWidgets('deletes the event after confirmation', (tester) async {
      final cap = detailCap();
      await pumpDetail(tester, cap);
      await tester.tap(find.byTooltip('Delete event'));
      await tester.pumpAndSettle();
      expect(find.text('Delete event?'), findsOneWidget);
      await tester.tap(find.text('Delete'));
      await tester.pumpAndSettle();
      expect(cap.countOf('deleteFoodEvent'), 1);
      expect(find.byType(EventDetailScreen), findsNothing);
    });

    testWidgets('cancel leaves the event in place', (tester) async {
      final cap = detailCap();
      await pumpDetail(tester, cap);
      await tester.tap(find.byTooltip('Delete event'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(cap.countOf('deleteFoodEvent'), 0);
      expect(find.byType(EventDetailScreen), findsOneWidget);
    });

    testWidgets('adds a free-form dish through the slot dialog',
        (tester) async {
      final cap = detailCap();
      await pumpDetail(tester, cap);
      await tester.tap(find.text('Add dish'));
      await tester.pumpAndSettle();
      expect(find.text('Add dish'), findsWidgets);

      await tester.ensureVisible(find.text('Save'));
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('addEventRecipe')!;
      expect(req['variables'], contains('"foodEventId":"e1"'));
      expect(req['variables'], contains('"mealType":"dinner"'));
      expect(req['variables'], contains('"targetTime":"2026-11-26T'));
      // Saving reloads the event.
      expect(cap.countOf('foodEvent('), greaterThanOrEqualTo(2));
    });

    testWidgets('adds a dish with a picked recipe and servings',
        (tester) async {
      final cap = detailCap();
      await pumpDetail(tester, cap);
      await tester.tap(find.text('Add dish'));
      await tester.pumpAndSettle();

      // Open the recipe picker sheet and pick a server-ranked recipe.
      await tester.tap(find.ancestor(
          of: find.text('Free-form dish'), matching: find.byType(InkWell)));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Sheet Pan Chicken'));
      await tester.pumpAndSettle();

      await tester
          .ensureVisible(find.widgetWithText(TextField, 'Servings (optional)'));
      await tester.enterText(
          find.widgetWithText(TextField, 'Servings (optional)'), '6');
      await tester
          .ensureVisible(find.widgetWithText(TextField, 'Notes (optional)'));
      await tester.enterText(
          find.widgetWithText(TextField, 'Notes (optional)'), 'Extra gravy');
      await tester.ensureVisible(find.text('Save'));
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('addEventRecipe')!;
      expect(req['variables'], contains('"recipeId":"r7"'));
      expect(req['variables'], contains('"servings":6'));
      expect(req['variables'], contains('"notes":"Extra gravy"'));
      expect(cap.countOf('recordSelection'), 1);
    });

    testWidgets('edits an existing dish via the slot dialog', (tester) async {
      final cap = detailCap(recipes: [eventSlot()]);
      await pumpDetail(tester, cap);
      await tester.tap(find.text('Roast Turkey'));
      await tester.pumpAndSettle();
      expect(find.text('Edit dish'), findsOneWidget);
      // Prefilled from the slot.
      expect(
          tester
              .widget<TextField>(
                  find.widgetWithText(TextField, 'Servings (optional)'))
              .controller!
              .text,
          '8');

      await tester
          .ensureVisible(find.widgetWithText(TextField, 'Servings (optional)'));
      await tester.enterText(
          find.widgetWithText(TextField, 'Servings (optional)'), '10');
      await tester.ensureVisible(find.text('Save'));
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('updateEventRecipe')!;
      expect(req['variables'], contains('"id":"s1"'));
      expect(req['variables'], contains('"servings":10'));
    });

    testWidgets('removes a dish via the trailing delete', (tester) async {
      final cap = detailCap(recipes: [eventSlot()]);
      await pumpDetail(tester, cap);
      await tester.tap(find.byIcon(Icons.delete_outline));
      await tester.pumpAndSettle();
      expect(cap.countOf('removeEventRecipe'), 1);
      expect(cap.lastMatching('removeEventRecipe')!['variables'],
          contains('"id":"s1"'));
    });
  });

  group('EventTimelineScreen', () {
    Map<String, dynamic> timelineStep({
      int stepNumber = 1,
      String instruction = 'Preheat the oven',
      bool isPassive = true,
      bool estimated = true,
      String? appliance = 'oven',
      String? startTime = '2026-11-26T15:30:00Z',
      String? endTime = '2026-11-26T16:00:00Z',
      List<String> conflicts = const [],
    }) =>
        {
          'stepNumber': stepNumber,
          'instruction': instruction,
          'stepType': 'cook',
          'isPassive': isPassive,
          'appliance': appliance,
          'durationMinutes': 30,
          'scheduledMinutes': 30,
          'estimated': estimated,
          'startTime': startTime,
          'endTime': endTime,
          'conflicts': conflicts,
        };

    _Capture timelineCap(Map<String, dynamic>? timeline) =>
        _Capture()..onQuery = (_) => gqlOk({'eventTimeline': timeline});

    Future<void> pumpTimeline(WidgetTester tester, _Capture cap) async {
      await tester.pumpWidget(_harness(
        cap,
        const EventTimelineScreen(
            foodEventId: 'e1', eventName: 'Friendsgiving'),
      ));
      await tester.pumpAndSettle();
    }

    testWidgets('renders warnings, recipes, steps, and conflicts',
        (tester) async {
      await pumpTimeline(
        tester,
        timelineCap({
          'foodEventId': 'e1',
          'warnings': ['Oven is double-booked'],
          'recipes': [
            {
              'eventRecipeId': 's1',
              'name': 'Roast Turkey',
              'targetTime': '2026-11-26T18:00:00Z',
              'servings': 8,
              'baseServings': 4,
              'startBy': '2026-11-26T16:30:00Z',
              'unschedulable': false,
              'warnings': ['Tight timing'],
              'steps': [
                timelineStep(),
                timelineStep(
                  stepNumber: 2,
                  instruction: 'Roast',
                  isPassive: false,
                  estimated: false,
                  appliance: null,
                  startTime: '2026-11-26T16:00:00Z',
                  endTime: '2026-11-26T17:30:00Z',
                  conflicts: ['Overlaps with Pie'],
                ),
              ],
            },
            {
              'eventRecipeId': 's2',
              'name': 'Impossible Dish',
              'targetTime': '2026-11-26T12:00:00Z',
              'servings': null,
              'baseServings': null,
              'startBy': null,
              'unschedulable': true,
              'warnings': [],
              'steps': [],
            },
          ],
        }),
      );
      expect(find.text('Friendsgiving — Timeline'), findsOneWidget);
      expect(find.text('Warnings'), findsOneWidget);
      expect(find.text('• Oven is double-booked'), findsOneWidget);
      expect(find.text('Roast Turkey'), findsOneWidget);
      expect(
          find.textContaining(
              'Serve 18:00 · start by 16:30 — 8 servings (recipe makes 4)'),
          findsOneWidget);
      expect(find.text('⚠ Tight timing'), findsOneWidget);
      expect(find.text('1. Preheat the oven'), findsOneWidget);
      expect(find.text('15:30–16:00'), findsOneWidget);
      expect(find.text('est. · hands-off · oven'), findsOneWidget);
      expect(find.text('⚠ Overlaps with Pie'), findsOneWidget);
      expect(find.textContaining('Cannot be scheduled'), findsOneWidget);
    });

    testWidgets('shows the nothing-to-schedule card', (tester) async {
      await pumpTimeline(
        tester,
        timelineCap({
          'foodEventId': 'e1',
          'warnings': const [],
          'recipes': const [],
        }),
      );
      expect(find.text('Nothing to schedule'), findsOneWidget);
    });

    testWidgets('shows a message when the timeline is null', (tester) async {
      await pumpTimeline(tester, timelineCap(null));
      expect(find.text('No timeline available.'), findsOneWidget);
    });

    testWidgets('shows an error when the query fails', (tester) async {
      final cap = _Capture()..onRequest = (_) => gqlErr();
      await pumpTimeline(tester, cap);
      expect(find.textContaining('Error:'), findsOneWidget);
    });
  });

  group('EditEventScreen', () {
    Future<void> pumpEdit(WidgetTester tester, _Capture cap,
        {Map<String, dynamic>? event}) async {
      await tester.pumpWidget(_harness(
        cap,
        _PushHost(builder: (_) => EditEventScreen(event: event)),
      ));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
    }

    testWidgets('creates an event with name, date, and granularity',
        (tester) async {
      final cap = _Capture();
      await pumpEdit(tester, cap);
      expect(find.text('New Event'), findsOneWidget);

      await tester.enterText(
          find.widgetWithText(TextField, 'Name'), 'Christmas Dinner');
      // The date field is read-only — pick through the real dialog.
      await tester.tap(find.widgetWithText(TextField, 'Date'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('15').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('30 minutes (simpler)'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('15 minutes (precise)').last);
      await tester.pumpAndSettle();

      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('createFoodEvent')!;
      expect(req['variables'], contains('"name":"Christmas Dinner"'));
      expect(req['variables'], contains('-15"'));
      expect(req['variables'], contains('"slotGranularityMinutes":15'));
      expect(find.byType(EditEventScreen), findsNothing);
    });

    testWidgets('updates an existing event', (tester) async {
      final cap = _Capture();
      await pumpEdit(tester, cap, event: eventItem());
      expect(find.text('Edit Event'), findsOneWidget);
      expect(
          tester
              .widget<TextField>(find.widgetWithText(TextField, 'Name'))
              .controller!
              .text,
          'Friendsgiving');
      expect(
          tester
              .widget<TextField>(find.widgetWithText(TextField, 'Date'))
              .controller!
              .text,
          '2026-11-26');

      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      final req = cap.lastMatching('updateFoodEvent')!;
      expect(req['variables'], contains('"id":"e1"'));
      expect(req['variables'], contains('"slotGranularityMinutes":15'));
    });

    testWidgets('shows a validation error when fields are empty',
        (tester) async {
      final cap = _Capture();
      await pumpEdit(tester, cap);
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();
      expect(find.text('Name and date are required.'), findsOneWidget);
      expect(cap.countOf('createFoodEvent'), 0);
    });

    testWidgets('shows a mutation error inline', (tester) async {
      final cap = _Capture()
        ..onRequest =
            (query) => query.contains('createFoodEvent') ? gqlErr() : null;
      await pumpEdit(tester, cap);
      await tester.enterText(find.widgetWithText(TextField, 'Name'), 'Doomed');
      await tester.tap(find.widgetWithText(TextField, 'Date'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Exception'), findsOneWidget);
      expect(find.byType(EditEventScreen), findsOneWidget);
    });

    testWidgets('picks a date through the date picker', (tester) async {
      final cap = _Capture();
      await pumpEdit(tester, cap);
      await tester.tap(find.widgetWithText(TextField, 'Date'));
      await tester.pumpAndSettle();
      expect(find.byType(DatePickerDialog), findsOneWidget);
      await tester.tap(find.text('15').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(
          tester
              .widget<TextField>(find.widgetWithText(TextField, 'Date'))
              .controller!
              .text,
          contains('-15'));
    });
  });
}
