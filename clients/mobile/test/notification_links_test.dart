import 'package:flutter_test/flutter_test.dart';
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
}
