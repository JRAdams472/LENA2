import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String lenaApiUrl = 'http://localhost:8080/graphql';

const _storage = FlutterSecureStorage();

Future<String?> getIdToken() async {
  return _storage.read(key: 'id_token');
}

final HttpLink httpLink = HttpLink(lenaApiUrl);

final AuthLink authLink = AuthLink(
  getToken: () async {
    final token = await getIdToken();
    return token == null ? null : 'Bearer $token';
  },
);

final GraphQLClient graphQLClient = GraphQLClient(
  cache: GraphQLCache(),
  link: authLink.concat(httpLink),
);
