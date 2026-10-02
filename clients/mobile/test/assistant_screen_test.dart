import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/assistant_screen.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets(
    'AssistantScreen offers the on-device model when the server has no AI provider',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: AssistantScreen()),
        ),
      );
      // The availability check fires on first build; the dead test endpoint
      // resolves it as a failure. With no server provider and no on-device
      // model, the screen now offers the local-model opt-in instead of a
      // dead end.
      for (var i = 0; i < 10; i++) {
        await tester.pump(const Duration(seconds: 1));
      }

      expect(find.text('Ask Dot'), findsOneWidget);
      expect(find.text('Run Dot on this device'), findsOneWidget);
      expect(find.text('Download model'), findsOneWidget);
    },
  );
}
