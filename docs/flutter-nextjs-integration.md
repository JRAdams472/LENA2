# LENA — Flutter & Next.js GraphQL Integration

## 1. Endpoint

Both clients call a single endpoint:

```
POST /graphql
Authorization: Bearer <id_token>
```

## 2. Flutter

### Dependencies

Add to `mobile/pubspec.yaml`:

```yaml
dependencies:
  flutter:
    sdk: flutter
  google_sign_in: ^6.2.2
  graphql_flutter: ^5.1.2
  flutter_secure_storage: ^9.2.2
```

### Client setup

```dart
import 'package:graphql_flutter/graphql_flutter.dart';

final HttpLink httpLink = HttpLink('http://localhost/graphql');

final AuthLink authLink = AuthLink(
  getToken: () async => 'Bearer ${await secureStorage.read(key: 'id_token')}',
);

final Link link = authLink.concat(httpLink);

final ValueNotifier<GraphQLClient> client = ValueNotifier(
  GraphQLClient(
    link: link,
    cache: GraphQLCache(store: InMemoryStore()),
  ),
);
```

### Example query

```dart
const String getGroceryList = r'''
  query GetGroceryList($id: ID!) {
    groceryList(id: $id) {
      id
      generatedAt
      items {
        id
        item { name unit }
        manualItemName
        quantityNeeded
        isChecked
      }
    }
  }
''';```

### Example mutation

```dart
const String generateList = r'''
  mutation GenerateList($mealPlanId: ID!) {
    generateGroceryList(mealPlanId: $mealPlanId) {
      id
      generatedAt
    }
  }
''';```

## 3. Next.js

### Dependencies

```bash
npm i @apollo/client graphql
```

### Client setup

```ts
// lib/apollo.ts
import { ApolloClient, InMemoryCache, createHttpLink } from '@apollo/client';
import { setContext } from '@apollo/client/link/context';

const httpLink = createHttpLink({
  uri: process.env.NEXT_PUBLIC_GRAPHQL_URL,
});

const authLink = setContext((_, { headers }) => {
  const token = localStorage.getItem('lena_id_token');
  return {
    headers: {
      ...headers,
      authorization: token ? `Bearer ${token}` : '',
    },
  };
});

export const client = new ApolloClient({
  link: authLink.concat(httpLink),
  cache: new InMemoryCache(),
});
```

### Example hook

```tsx
// app/grocery-lists/[id]/page.tsx
import { useQuery, gql } from '@apollo/client';

const GET_GROCERY_LIST = gql`
  query GetGroceryList($id: ID!) {
    groceryList(id: $id) {
      id
      generatedAt
      items {
        id
        item { name unit }
        manualItemName
        quantityNeeded
        isChecked
      }
    }
  }
`;

export default function GroceryListPage({ params }: { params: { id: string } }) {
  const { data, loading, error } = useQuery(GET_GROCERY_LIST, {
    variables: { id: params.id },
  });
  // render
}
```

## 4. Shared Patterns

- All GraphQL requests carry the ID token in the `Authorization` header.
- On `401` / `UNAUTHENTICATED`, the client should clear the token and return to the sign-in screen.
- Use fragments for `Item`, `Bottle`, `Recipe`, `MealSlot`, etc. to keep queries DRY.

## 5. Migration from REST

| Old REST | New GraphQL |
|---|---|
| `GET /api/GroceryList/{id}` | `query { groceryList(id) }` |
| `POST /api/Item/items/{id}/quantity` | `mutation { adjustUserItem(itemId, quantity) }` |
| `POST /api/Item/items/{id}/favorite` | `mutation { setItemFavorite(itemId, isFavorite) }` |
| `POST /api/GroceryList/generate?mealPlanId=...` | `mutation { generateGroceryList(mealPlanId) }` |
| `GET /api/auth/me` | `query { me }` |

## 6. Recommended Client Structure

- Flutter: `lib/graphql/` with one `.dart` file per domain (queries + mutations).
- Next.js: `lib/queries/` and `lib/mutations/` or co-locate GraphQL documents with pages.