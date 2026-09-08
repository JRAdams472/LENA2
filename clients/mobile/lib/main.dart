import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:provider/provider.dart';
import 'auth/auth_gate.dart';
import 'auth/auth_service.dart';
import 'graphql_config.dart';

void main() {
  runApp(const LenaApp());
}

class LenaApp extends StatelessWidget {
  const LenaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return GraphQLProvider(
      client: ValueNotifier(graphQLClient),
      child: ChangeNotifierProvider.value(
        value: authService,
        child: MaterialApp(
          title: 'LENA',
          theme: ThemeData(primarySwatch: Colors.teal),
          home: const AuthGate(),
        ),
      ),
    );
  }
}
