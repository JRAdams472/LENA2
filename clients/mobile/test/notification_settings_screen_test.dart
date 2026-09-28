import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/notification_settings_screen.dart';

void main() {
  testWidgets(
    'NotificationSettingsScreen renders its app bar while the query loads',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: NotificationSettingsScreen()),
        ),
      );
      await tester.pump();

      expect(find.text('Notification settings'), findsOneWidget);
    },
  );
}
