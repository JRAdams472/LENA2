import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'graphql_config.dart';
import 'screens/main_screen.dart';

void main() {
  runApp(const LenaApp());
}

class LenaApp extends StatelessWidget {
  const LenaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return GraphQLProvider(
      client: ValueNotifier(graphQLClient),
      child: MaterialApp(
        title: 'LENA',
        theme: ThemeData(primarySwatch: Colors.teal),
        home: const MainScreen(),
      ),
    );
  }
}
