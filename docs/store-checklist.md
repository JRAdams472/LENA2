# Store Submission Checklist

Pre-flight list for App Store and Google Play submission of the Lena
companion app (`clients/mobile`). Each section notes what is already done
in the repo and what remains a console-side step.

## App identity

- **Application ID / bundle ID:** `app.lenaskitchen.lena` — set in
  `clients/mobile/android/app/build.gradle.kts` (`applicationId` +
  `namespace`, `MainActivity` under `kotlin/app/lenaskitchen/lena/`) and in
  `ios/Runner.xcodeproj` (`PRODUCT_BUNDLE_IDENTIFIER`; tests use
  `app.lenaskitchen.lena.RunnerTests`).
- **Display name:** "Lena" (`android:label`, `CFBundleDisplayName`/`CFBundleName`).

**Console-side follow-ups after the rename:**

- Firebase: register the Android app under `app.lenaskitchen.lena` and
  replace the gitignored `android/app/google-services.json`; same for the
  iOS `GoogleService-Info.plist`.
- Google sign-in: the Android OAuth client must be created for
  `app.lenaskitchen.lena` + the release-key SHA-1
  (`docs/google-oauth-client-id.md`); the iOS client's reversed ID feeds
  `GOOGLE_REVERSED_CLIENT_ID` in `Runner.xcconfig`.
- Apple sign-in: register `app.lenaskitchen.lena` as the app's bundle ID
  with the *Sign In with Apple* capability, and set
  `LENA_APPLE_BUNDLE_ID=app.lenaskitchen.lena` on the API so native
  identityTokens verify.
- Provider redirect URIs stay unchanged — mobile OAuth uses the
  `lena://oauth/<provider>` scheme, not the package name.

## Android

- **SDK levels:** `compileSdk`/`targetSdk`/`minSdk` come from the Flutter
  toolchain (Flutter 3.47: compile/target **36**, min **24**, NDK
  28.2.x). Play requires a recent targetSdk — current until mid-2027.
- **Release signing:** upload key is *not* in the repo. Before the first
  Play build:
  ```bash
  keytool -genkey -v -keystore %USERPROFILE%\lena-upload.jks \
    -keyalg RSA -keysize 2048 -validity 10950 -alias lena-upload
  ```
  then create `clients/mobile/android/key.properties` (gitignored):
  ```properties
  storeFile=C:/Users/<you>/lena-upload.jks
  storePassword=<store password>
  keyAlias=lena-upload
  keyPassword=<key password>
  ```
  `build.gradle.kts` picks it up automatically; without the file, release
  builds fall back to the debug signature (fine for `flutter run
  --release`, rejected by Play). Enroll the app in **Play App Signing** on
  first upload so Google holds the app-signing key.
- **Build artifact:** submit an AAB — `flutter build appbundle`, never the
  APK.
- **Minification:** not enabled; if you turn on R8 later, re-test the
  scan screen and sign-in on a real device first (no ProGuard rules are
  shipped).
- **Permissions:** `INTERNET`, `CAMERA` (barcode scan), and
  `POST_NOTIFICATIONS` (Android 13+, requested after a rationale sheet —
  see `lib/push/permission_rationale.dart`). No SMS/contacts/location.
- **Data-safety form:** declare email + name (social sign-in), device
  push token (notifications), and household content the user enters. All
  data in transit is TLS; users can delete the account in-app (Household
  tab → danger zone), which Play requires to be disclosed.
- **Privacy policy URL:** `https://lenaskitchen.app/privacy` — required
  by both the store listing and the in-app prompt.

## iOS

- **Deployment target:** iOS 15.0; device families iPhone + iPad.
- **Entitlements:** Sign In with Apple (`Runner.entitlements`) —
  required because the app offers third-party sign-in.
- **Privacy manifest:** `Runner/PrivacyInfo.xcprivacy` ships with empty
  collected-data/accessed-API arrays; nutrition labels in App Store
  Connect carry the declarations (email, name, device token, user
  content).
- **Purpose strings:** `NSCameraUsageDescription` is set ("scan
  barcodes"). Notification permission prompts after the in-app rationale
  sheet; no plist key needed.
- **Encryption:** `ITSAppUsesNonExemptEncryption=false` is set in
  `Info.plist` (TLS only) — the export-compliance question is answered in
  the binary and won't block TestFlight processing.
- **Pending verification:** the native Apple sign-in sheet + entitlement
  still need a macOS/Xcode device build — code and config are in place
  (LEN-136 P2) but the flow was exercised only through tests.

## Content rating

The app surfaces alcohol references (wine cellar, "cocktail ideas",
wine pairing suggestions — all gated by the birthdate check):

- **App Store:** answer *yes* to "Alcohol, Tobacco, or Drug Use or
  References" → expect **12+** (infrequent/mild).
- **Play:** content-rating questionnaire → "references to alcohol" —
  typically **Teen**.

## Reviewer notes

- The app gates new users behind admin approval. Give the reviewer a
  pre-approved demo account in the submission notes, plus the login
  steps — a reviewer signing up fresh hits the approval screen and cannot
  reach the app.
- Demo credentials: create a reviewer account on the staging/production
  admin (`/admin` → pending users → approve) and put the login in the
  App Review "notes" field and Play's app-access instructions.
- Deep links `lena://oauth/<provider>` are required for mobile OAuth —
  no reviewer action needed, but note it in case a reviewer asks why a
  Custom Tab appears during sign-in.

## Store assets still needed

- App icon: repo now ships the sage-cloche icon on both platforms; Play
  listing also wants a 512×512 hi-res icon and a 1024×500 feature
  graphic.
- Screenshots: iPhone 6.7"/6.5" (required), iPad 13"/12.9" (required if
  keeping iPad in targeted families), Play phone + 7"/10" tablet.
