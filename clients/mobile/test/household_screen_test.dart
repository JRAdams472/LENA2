import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/household_screen.dart';

void main() {
  testWidgets(
    'HouseholdScreen renders its app bar while the query loads',
    (tester) async {
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: HouseholdScreen()),
        ),
      );
      await tester.pump();

      expect(find.text('Household'), findsOneWidget);
    },
  );
}
