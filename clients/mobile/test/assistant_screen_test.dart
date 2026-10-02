import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/assistant_screen.dart';

void main() {
  testWidgets(
    'AssistantScreen shows the unavailable notice when aiAvailable is false or the query fails',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: AssistantScreen()),
        ),
      );
      // The availability check fires on first build; the dead test endpoint
      // resolves it as a failure, which maps to the unavailable notice.
      await tester.pumpAndSettle(const Duration(seconds: 5));

      expect(find.textContaining("isn't configured"), findsOneWidget);
      expect(find.text('Ask Dot'), findsOneWidget);
    },
  );
}
