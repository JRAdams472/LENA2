# LENA Frontend

A Next.js 16 (App Router) TypeScript frontend for the LENA admin dashboard.

## Requirements

- Node.js 20+
- A running `LENA.API` instance (default: `http://localhost:5059`)

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
   NEXT_PUBLIC_API_BASE_URL=http://localhost:5059
   NEXT_PUBLIC_GOOGLE_CLIENT_ID=__YOUR_GOOGLE_CLIENT_ID__
   ```

   `NEXT_PUBLIC_GOOGLE_CLIENT_ID` must be the same client ID that `LENA.API` uses as the JWT audience (`Authentication:Google:ClientId`).

   See [docs/google-oauth-client-id.md](../docs/google-oauth-client-id.md) for how to create the client ID and add the authorized JavaScript origins.

3. Start the development server:

   ```bash
   npm run dev
   ```

   Open [http://localhost:3000](http://localhost:3000) in your browser.

4. Confirm CORS origin matches the API. The API `Program.cs` is configured with `AllowExternal` allowing any origin, header, and method. If you change the frontend origin, ensure the API CORS policy allows it.

5. In the Google Cloud Console for the OAuth client, add the frontend origin to the **Authorized JavaScript origins** list. For local development this is `http://localhost` (or `http://localhost:3000`). The production origin must also be added (e.g. `https://yourdomain.com`).

## Build

```bash
npm run build
```

## Tech Stack

- Next.js (App Router)
- TypeScript
- Material UI
- React Query
