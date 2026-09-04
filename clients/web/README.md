# LENA Next.js Web Client

A minimal Next.js (Pages Router) starter that consumes the LENA GraphQL BFF.

## Getting started

```bash
cd clients/web
npm install
cp .env.local.example .env.local
npm run dev
```

## Configuration

Create `.env.local`:

```env
NEXT_PUBLIC_LENA_API_URL=http://localhost:8080/graphql
```

The app expects an OIDC provider to issue an ID token. In development you can set the token in the browser console or replace the `authLink` in `lib/apolloClient.ts` with a mock.

## Project structure

- `lib/apolloClient.ts` — Apollo Client setup with an auth link.
- `pages/_app.tsx` — Wraps every page in `ApolloProvider`.
- `pages/index.tsx` — Example dashboard using the `Me` query and pantry list.
- `pages/grocery.tsx` — Example grocery list query and `toggleGroceryItemChecked` mutation.
