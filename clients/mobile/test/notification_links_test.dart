import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/notification_links.dart';
import 'package:lena_mobile/screens/edit_recipe_screen.dart';
import 'package:lena_mobile/screens/event_detail_screen.dart';
import 'package:lena_mobile/screens/pantry_screen.dart';

void main() {
  group('notificationDestination', () {
    test('recipeId routes to the recipe editor with the id', () {
      final dest = notificationDestination({'recipeId': 'r-1'});
      expect(dest, isA<EditRecipeScreen>());
      expect((dest as EditRecipeScreen).recipeId, 'r-1');
    });

    test('itemId routes to the pantry', () {
      expect(notificationDestination({'itemId': 'i-1'}), isA<PantryScreen>());
    });

    test('foodEventId routes to the event detail with the id', () {
      final dest = notificationDestination({'foodEventId': 'e-9'});
      expect(dest, isA<EventDetailScreen>());
      expect((dest as EventDetailScreen).foodEventId, 'e-9');
    });

    test('push-style string maps resolve the same as feed maps', () {
      // RemoteMessage.data arrives as Map<String, String>; feed rows are
      // Map<String, dynamic>. Both shapes take the same route.
      final dest = notificationDestination(<String, String>{'recipeId': 'r-2'});
      expect(dest, isA<EditRecipeScreen>());
    });

    test('kinds with no link fields have no destination', () {
      expect(notificationDestination({'kind': 'member_joined'}), isNull);
      expect(notificationDestination({'householdId': 'h-1'}), isNull);
      expect(notificationDestination(const {}), isNull);
    });
  });

  group('openNotificationLink', () {
    testWidgets('pushes the resolved destination onto the Navigator', (
      tester,
    ) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: MaterialApp(
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  onPressed: () =>
                      openNotificationLink(context, {'itemId': 'i-1'}),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pump();
      await tester.pump();

      expect(find.byType(PantryScreen), findsOneWidget);
    });

    testWidgets('does nothing when the notification has no destination', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () => openNotificationLink(
                  context,
                  {'kind': 'member_joined'},
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pump();
      await tester.pump();

      // Still on the source route — nothing was pushed.
      expect(find.text('open'), findsOneWidget);
      expect(find.byType(PantryScreen), findsNothing);
      expect(find.byType(EditRecipeScreen), findsNothing);
    });
  });
}
