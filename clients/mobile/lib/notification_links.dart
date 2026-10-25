import 'package:flutter/material.dart';
import 'screens/edit_recipe_screen.dart';
import 'screens/event_detail_screen.dart';
import 'screens/pantry_screen.dart';

/// Maps a notification's link fields to its destination screen. Used by the
/// feed rows on HouseholdScreen and by push-notification taps — the server
/// sends the same link columns (recipeId / itemId / foodEventId / inviteId /
/// householdId) in the push data payload that feed rows carry inline.
///
/// Feed rows pass the notification map as-is; the values are dynamic because
/// GraphQL IDs arrive as strings. Push taps pass `RemoteMessage.data`, where
/// every value is already a String.
/// Reads a link id tolerantly — push `RemoteMessage.data` values are
/// Strings, but feed rows can carry num/null payloads; a hard `as String?`
/// cast on the wrong type throws in the tap handler.
String? _linkId(Map<dynamic, dynamic> fields, String key) {
  final v = fields[key];
  return v is String ? v : v?.toString();
}

Widget? notificationDestination(Map<dynamic, dynamic> fields) {
  final recipeId = _linkId(fields, 'recipeId');
  if (recipeId != null) {
    return EditRecipeScreen(recipeId: recipeId);
  }
  if (_linkId(fields, 'itemId') != null) {
    return const PantryScreen();
  }
  final eventId = _linkId(fields, 'foodEventId');
  if (eventId != null) {
    return EventDetailScreen(foodEventId: eventId);
  }
  return null;
}

/// Pushes a notification's destination onto the nearest Navigator.
void openNotificationLink(BuildContext context, Map<dynamic, dynamic> fields) {
  final dest = notificationDestination(fields);
  if (dest == null) return;
  Navigator.push(context, MaterialPageRoute(builder: (_) => dest));
}
