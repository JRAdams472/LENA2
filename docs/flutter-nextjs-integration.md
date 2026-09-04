# Flutter & Next.js Client Integration

This document describes how the Flutter mobile app and the Next.js web app connect to the LENA GraphQL BFF.

## Endpoint

- **URL**: `https://<host>/graphql`
- **Method**: `POST`
- **Content-Type**: `application/json`
- **Authentication**: `Authorization: Bearer <id_token>`

The Go monolith (`cmd/lena`) serves the GraphQL endpoint with the BFF resolver. In local development the URL is `http://localhost:8080/graphql`.

## Authentication

1. Client obtains an ID token from the configured OIDC provider (Google by default).
2. Include the token in the `Authorization` header:
   ```http
   Authorization: Bearer <id_token>
   ```
3. The BFF middleware validates the JWT and sets the current user context. If the user does not exist, a local LENA account is created on first request.

## Flutter

### Dependencies

```yaml
dependencies:
  graphql_flutter: ^5.1.0
  flutter_secure_storage: ^9.0.0
```

### Client Setup

```dart
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

final _storage = FlutterSecureStorage();

final HttpLink httpLink = HttpLink('https://api.lena.local/graphql');

final AuthLink authLink = AuthLink(
  getToken: () async {
    final token = await _storage.read(key: 'id_token');
    return token == null ? null : 'Bearer $token';
  },
);

final GraphQLClient client = GraphQLClient(
  cache: GraphQLCache(),
  link: authLink.concat(httpLink),
);
```

### Example Query

```dart
const String meQuery = r'''
  query Me {
    me {
      id
      email
      displayName
    }
  }
''';

final QueryOptions options = QueryOptions(document: gql(meQuery));
final QueryResult result = await client.query(options);
```

### Example Mutation

```dart
const String createItemMutation = r'''
  mutation CreateItem($input: CreateItemInput!) {
    createItem(input: $input) {
      id
      name
    }
  }
''';

final MutationOptions options = MutationOptions(
  document: gql(createItemMutation),
  variables: <String, dynamic>{
    'input': {
      'name': 'Almond Milk',
      'categoryId': '1',
      'unit': 'fl oz',
    },
  },
);
final QueryResult result = await client.mutate(options);
```

## Next.js

### Dependencies

```bash
npm install @apollo/client graphql
```

### Client Setup

```ts
// lib/apolloClient.ts
import { ApolloClient, InMemoryCache, createHttpLink } from '@apollo/client';
import { setContext } from '@apollo/client/link/context';

const httpLink = createHttpLink({
  uri: 'https://api.lena.local/graphql',
  credentials: 'include',
});

const authLink = setContext(async (_, { headers }) => {
  const token = await getIdToken(); // your OIDC helper
  return {
    headers: {
      ...headers,
      authorization: token ? `Bearer ${token}` : '',
    },
  };
});

export const apolloClient = new ApolloClient({
  link: authLink.concat(httpLink),
  cache: new InMemoryCache(),
});
```

### Example Query

```tsx
import { gql, useQuery } from '@apollo/client';

const ME_QUERY = gql`
  query Me {
    me {
      id
      email
      displayName
    }
  }
`;

function Profile() {
  const { data, loading, error } = useQuery(ME_QUERY);
  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;
  return <div>{data.me.email}</div>;
}
```

### Example Mutation

```tsx
import { gql, useMutation } from '@apollo/client';

const CREATE_ITEM = gql`
  mutation CreateItem($input: CreateItemInput!) {
    createItem(input: $input) {
      id
      name
    }
  }
`;

function AddItemForm() {
  const [createItem, { data, loading, error }] = useMutation(CREATE_ITEM);
  // ...
}
```

## Common Patterns

- Use `item(id: $id)` and `items(page: $page, pageSize: $pageSize)` for catalog browsing.
- Pantry and cellar data come from `userItems` and `userBottles`.
- Meal planning flows use `mealPlans`, `addMealSlot`, and `generateGroceryList`.
- Grocery shopping uses `groceryLists` and `toggleGroceryItemChecked`.

## Error Handling

- Return `401` / `403` responses mean the token is missing, expired, or invalid. Clients should refresh the OIDC token and retry.
- GraphQL errors are returned in the `errors` array even when HTTP status is `200`. Inspect `result.errors` (Flutter) or `error.graphQLErrors` (Apollo).
