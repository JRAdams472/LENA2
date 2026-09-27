import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/event_detail_screen.dart';
import 'package:lena_mobile/screens/event_timeline_screen.dart';
import 'package:lena_mobile/screens/events_screen.dart';

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

  testWidgets(
    'EventDetailScreen renders its scaffold',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(
            home: EventDetailScreen(foodEventId: '1'),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(Scaffold), findsOneWidget);
    },
  );

  testWidgets(
    'EventTimelineScreen renders its app bar while the query loads',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(
            home: EventTimelineScreen(foodEventId: '1', eventName: 'Dinner'),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Dinner — Timeline'), findsOneWidget);
    },
  );

  test('serveTimeOptions returns granularity-aligned times', () {
    final opts15 = serveTimeOptions(15);
    expect(opts15.length, 96);
    expect(opts15.first, '00:00');
    expect(opts15.last, '23:45');

    final opts30 = serveTimeOptions(30);
    expect(opts30.length, 48);
    expect(opts30.last, '23:30');
  });

  test('toTargetTime emits a UTC timestamp on the event date', () {
    expect(toTargetTime('2026-11-26', '18:30'), '2026-11-26T18:30:00Z');
  });

  test('hhmmOf renders UTC hh:mm', () {
    expect(hhmmOf('2026-11-26T18:30:00Z'), '18:30');
    expect(hhmmOf(null), '');
    expect(hhmmOf('not-a-date'), '');
  });
}
