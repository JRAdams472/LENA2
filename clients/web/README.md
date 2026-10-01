# LENA Web Client

A Next.js (App Router) TypeScript frontend for LENA.

## Requirements

- Node.js 20+
- A running LENA API — either the compose stack (`docker compose up`, app
  served behind Caddy at `http://localhost`) or `go run ./cmd/lena` on
  `http://localhost:8080`.

## Getting Started

1. Install dependencies:

   ```bash
   npm install
   ```

2. Copy `.env.example` to `.env.local` and update the values:

   ```bash
   cp .env.example .env.local
   ```

   Example `.env.local`:

   ```
   NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
   NEXT_PUBLIC_GOOGLE_CLIENT_ID=__YOUR_GOOGLE_CLIENT_ID__
   ```

   `NEXT_PUBLIC_API_BASE_URL` defaults to `/graphql` (same-origin — Caddy
   routes `/graphql` and `/auth/*` to the API). Point it at the API host
   only when running the dev server without the compose stack.

   `NEXT_PUBLIC_GOOGLE_CLIENT_ID` must be the same client ID the API trusts
   in `LENA_AUTH_AUDIENCES`. See
   [docs/google-oauth-client-id.md](../../docs/google-oauth-client-id.md) for
   creating the client ID and adding authorized JavaScript origins.

3. Start the development server:

   ```bash
   npm run dev
   ```

   Open [http://localhost:3000](http://localhost:3000) in your browser.

4. The API CORS allowlist is `LENA_CORS_ALLOWED_ORIGINS` on the server —
   include the dev origin (`http://localhost:3000`) when running outside
   the compose proxy.

5. In the Google Cloud Console, add the frontend origin to the OAuth
   client's **Authorized JavaScript origins** (`http://localhost:3000` for
   dev; the production origin as well).

## Sign-in providers

Google sign-in uses the OIDC ID token as the bearer directly. Discord,
Microsoft, and Facebook are OAuth2 redirect flows: the browser collects a
`code` at `/auth/{provider}/callback` and the server exchanges it
(server-side `client_secret`) for a LENA session. Each button renders only
when its `NEXT_PUBLIC_*_CLIENT_ID` build-time env is set; see
`.env.example` and `lib/oauth.ts`. Facebook additionally requires the
`nonce` round-trip — handled automatically.

## Sessions

When the server sets `LENA_SESSION_SECRET`, sign-in exchanges the provider
credential for a LENA session: a ~15-minute access token in
`sessionStorage` plus a ~30-day rotating refresh token in `localStorage`.
`lib/api.ts` refreshes single-flight and retries a request once on 401;
sign-out revokes the session server-side. Without the secret the client
falls back to passing the provider token as the bearer (OIDC-only mode).

## Build

```bash
npm run build
```

## Tech Stack

- Next.js (App Router, `output: "standalone"`)
- TypeScript
- Material UI
- React Query
