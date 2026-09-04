# Getting a Google OAuth Client ID

LENA uses Google Sign-In for both the Next.js frontend and the Flutter mobile app. The same **Web** OAuth client ID is the JWT audience for `LENA.API`.

## 1. Create or open a Google Cloud project

1. Go to [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new project or select an existing one.
3. Make sure the **Google Identity Services** API is enabled for the project.

## 2. Configure the OAuth consent screen

1. Navigate to **APIs & Services > OAuth consent screen**.
2. Choose **External** (for personal use with any Google account) or **Internal** (if you are using Google Workspace).
3. Fill in the required app name, support email, and developer contact information.

## 3. Create the Web client ID

1. Go to **APIs & Services > Credentials**.
2. Click **Create Credentials > OAuth client ID**.
3. Select **Web application**.
4. Add the local development origin to **Authorized JavaScript origins**:
   - `http://localhost`
   - `http://localhost:3000`
5. Add your production origin when you deploy, e.g. `https://yourdomain.com`.
6. Click **Create**.
7. Copy the **Client ID** (it looks like `123456789-abc...apps.googleusercontent.com`).

> **Note:** LENA only uses the ID token, so no authorized redirect URI is required.

## 4. Use the same client ID everywhere

Use the **Web** client ID for all three components:

- `LENA.API` — `Authentication:Google:ClientId`
- `frontend/.env.local` — `NEXT_PUBLIC_GOOGLE_CLIENT_ID`
- Mobile — `GOOGLE_SERVER_CLIENT_ID` passed as a `--dart-define`

The API validates the `aud` claim of the Google ID token against this value, so all three must match exactly.

## 5. Mobile-specific setup (Android)

For the Flutter `google_sign_in` plugin you also need a Google Services config for the Android package name:

1. In Google Cloud Console, create an **Android** OAuth client ID for your package name (default is `com.example.lena` unless you changed it in `mobile/android/app/build.gradle.kts`).
2. Download `google-services.json` and place it in `mobile/android/app/`.
3. The `GOOGLE_SERVER_CLIENT_ID` value remains the **Web** client ID from step 3.

For iOS, create an **iOS** client ID and use `GoogleService-Info.plist` in `mobile/ios/Runner/`.

## 6. Docker Compose

For `docker compose up --build`, copy `.env.example` to `.env` in the repo root and set the values:

```bash
cp .env.example .env
```

```ini
# Google OAuth Web client ID (public)
GOOGLE_CLIENT_ID=<your-web-client-id>

# Frontend API base URL (Docker uses 'http://localhost', local dev uses 'http://localhost:5059')
NEXT_PUBLIC_API_BASE_URL=http://localhost

# SQL Server 'sa' password for the local db container
MSSQL_SA_PASSWORD=<your-strong-sql-sa-password>

# Application SQL login password used by the API (least-privilege 'lena_app' user)
LENA_DB_PASSWORD=<your-strong-app-password>
```

`docker-compose.yml` uses `GOOGLE_CLIENT_ID` for both the API `Authentication:Google:ClientId` and the `ui` build arg `NEXT_PUBLIC_GOOGLE_CLIENT_ID`. `MSSQL_SA_PASSWORD` is used by the `db` and `db-init` services, and `LENA_DB_PASSWORD` is used to create the `lena_app` SQL login/user and as the API connection-string password. The `.env` file is ignored by Git, so secrets stay local.

> **Note:** The API connection string in Docker uses `TrustServerCertificate=True` for local development only. Real deployments must provision a trusted TLS certificate for SQL Server and remove this flag.

## 7. Common error: `invalid_client`

This error from the Google sign-in popup means the client ID passed to Google does not exist, is not a Web client ID, or the calling origin is not in **Authorized JavaScript origins**. Double-check that:

- The value is the full Web client ID string, not a placeholder.
- The origin you are browsing from is allowed in the Web client settings.
- For Android, `google-services.json` is present and matches the package name.
