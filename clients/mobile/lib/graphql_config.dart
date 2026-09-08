import 'package:graphql_flutter/graphql_flutter.dart';
import 'auth/auth_service.dart';

const String lenaApiUrl = String.fromEnvironment(
  'LENA_API_URL',
  defaultValue: 'http://localhost:8080/graphql',
);

final AuthLink authLink = AuthLink(
  getToken: () async {
    final token = authService.idToken;
    return token == null ? null : 'Bearer $token';
  },
);

final ErrorLink errorLink = ErrorLink(
  onGraphQLError: (request, forward, response) {
    final errors = response.errors;
    if (errors != null) {
      final unauthorized = errors.any(
        (e) =>
            e.message.toLowerCase().contains('unauthenticated') ||
            e.message.toLowerCase().contains('unauthorized'),
      );
      if (unauthorized) {
        authService.signOut();
      }
    }
    return null;
  },
);

final GraphQLClient graphQLClient = GraphQLClient(
  cache: GraphQLCache(),
  link: Link.from([errorLink, authLink, HttpLink(lenaApiUrl)]),
);
