# Mobile Redesign Megaplan — mobile-redesign-p0…p5: Auth, Dashboard, Grocery→Inventory, Barcode Scan, Item Approval

Redesign the Flutter mobile client (clients/mobile) with Google OIDC auth, a Dashboard, grocery-list check-off that syncs to inventory, and a barcode Scan Item flow with a user-submitted / admin-approved catalog pipeline — delivered as named phases mobile-redesign-p0 through p5 (backend approval workflow + UPC lookup, grocery↔inventory sync, web admin approval page, mobile auth/dashboard/grocery, scan flow, CI/docs).

## Context & current state

- **Backend**: Go monolith, GraphQL BFF at `/graphql` (Echo + graphql-go). Auth = OIDC ID token bearer (`internal/bff/auth.go`); issuer `https://accounts.google.com`, audience from `LENA_AUTH_AUDIENCES`. Web stores the Google credential under `lena_id_token` and sends `Authorization: Bearer <token>` (`clients/web/app/auth/AuthProvider.tsx`).
- **Mobile**: `clients/mobile` is a minimal Flutter skeleton — `graphql_flutter` + `flutter_secure_storage` already in `pubspec.yaml`, but `getIdToken()` is a stub and screens are placeholders (`lib/screens/*.dart`).
- **Grocery**: `toggleGroceryItemChecked` only flips `is_checked` (`internal/bff/resolver_grocery.go`); nothing writes to `inventory.user_item`. `adjustUserItem` (set semantics) and `deleteUserItem` exist in `resolver_userprefs.go`, backed by `userprefs.UpsertUserItem` (`ON CONFLICT (user_id, item_id) DO UPDATE`).
- **Items**: `inventory.item` has unique `upc12`/`upc14` but **no** approval/status/owner columns and **no** UPC lookup query. `createItem`, `updateItem`, `addFoodNutrient` are all `requireAdmin` (`internal/bff/resolver_inventory.go`).
- **Dashboard**: web `app/page.tsx` shows today's meal-plan slots + `recommendedRecipes` — the mobile Dashboard mirrors this.
- **Conventions**: AGENTS.md prescribes `phase-<N>` branches. For this effort we use a named series instead — **`mobile-redesign-p0`, `mobile-redesign-p1`, …** — so each branch/PR is self-describing. Migration number used: `0020`. sqlc via `sqlc generate` (config `sqlc.yaml`). Verification per phase: `go build ./...`, `go vet ./...`, `gofmt`, `go test -short ./...`, `golangci-lint run`; web: `tsc --noEmit`, `eslint`, `jest`; mobile: `flutter analyze`, `flutter test`.

## Phase overview

| Branch | Name | Contents |
|---|---|---|
| `mobile-redesign-p0` | Item approval + UPC lookup (backend) | Migration `0020`, item status/visibility, `itemByUpc`, `submitItem`, `approveItem`/`rejectItem`, `setItemNutrients` |
| `mobile-redesign-p1` | Grocery↔inventory sync (backend) | `toggleGroceryItemChecked` stock sync in one tx, `incrementUserItem` |
| `mobile-redesign-p2` | Pending-items admin page (web) | `/items/pending` approve/reject UI |
| `mobile-redesign-p3` | Mobile auth + shell + Dashboard + grocery (Flutter) | Google sign-in, auth gate, dashboard, grocery lists, pantry |
| `mobile-redesign-p4` | Scan Item flow (Flutter) | Barcode scan, found/not-found, create item, nutrition entry |
| `mobile-redesign-p5` | Mobile CI + docs | `flutter analyze`/`test` job, READMEs, docs updates |

## Decisions locked in (from user)

1. **Check-off sync**: checking a grocery item adds `quantityNeeded` to `inventory.user_item`; unchecking removes the same amount (floor 0). Only for items with `item_id` set (skip `manual_item_name`/ingredient-only rows).
2. **Nutrition**: Phase 1 = manual nutrient entry on user-created items, structured so a future OCR label-scan pipeline can feed the same mutation (no throwaway code).
3. **Approval UI**: web admin page (like `/users`), not mobile.
4. **Stack**: keep Flutter; target **Android + iOS** from the start (permissions/config for both; dev/test on Android).
5. **Scan quantity**: prompt for quantity (default 1) after a match — Add increments, Remove decrements and deletes the row at 0.
6. **Barcodes**: 12 digits → `upc12`; 13 digits → left-pad `0` → `upc14`; 14 digits → `upc14`; anything else → not found.
7. **Scope**: auth gate, Dashboard, grocery, scan flow, pending-item nutrition entry only. Wine/recipe/meal-plan stub screens stay placeholders.
8. **Pending visibility**: creator-only everywhere — pending items visible to/usable by the creator in lists, scan results, grocery/recipe pickers; invisible to other users until approved. Admin-created items via existing `createItem` stay auto-approved (unchanged admin path).

---

## mobile-redesign-p0 — Backend: item approval workflow + UPC lookup

### Migration `0020_item_approval.up.sql` / `.down.sql`
```sql
ALTER TABLE inventory.item
  ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'approved'
    CHECK (status IN ('pending','approved','rejected')),
  ADD COLUMN submitted_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
  ADD COLUMN approved_by_user_id BIGINT REFERENCES identity.users(user_id),
  ADD COLUMN approved_at TIMESTAMPTZ;
CREATE INDEX idx_item_status ON inventory.item (status);
```
Existing rows default to `approved`. `.down.sql` drops index + columns.

### sqlc queries (`internal/inventory/queries.sql`, regen `internal/inventory/sqlc/`)
- `GetItemByUpc` :one — `WHERE (upc12 = $code OR upc14 = $code) AND (status = 'approved' OR submitted_by_user_id = $userID)`.
- `ListItems` / `CountItems` — gained `userID` param; `WHERE status='approved' OR submitted_by_user_id = $userID`.
- `ListPendingItems` / `CountPendingItems` — `WHERE status='pending'` (admin).
- `SetItemStatus` :exec — sets `status`, `approved_by_user_id`, `approved_at` (only when approving).
- `CreateItem` gains `status`, `submitted_by_user_id` params.
- `DeleteFoodNutrientsByItem` :exec — for the `setItemNutrients` replace-all path.

### Service (`internal/inventory/service.go`)
- `Item` struct: `Status`, `SubmittedByUserID *int64`, `ApprovedByUserID *int64`, `ApprovedAt *time.Time`.
- `CreateItem` unchanged semantics (always approved); `SubmitItem(ctx, arg, userID, by)` creates `status='pending'`.
- `GetItemByUpc(ctx, code, userID)`, `ListItems(ctx, userID, limit, offset)`, `CountItems(ctx, userID)`, `ListPendingItems`, `CountPendingItems`, `SetItemStatus(ctx, itemID, status, adminUserID, by)`.
- `SetItemNutrients(ctx, itemID, entries, by)` — delete + insert in one transaction.

### BFF (`internal/bff/`)
- `schema.graphqls`:
  - `Item` gains `status: String!`, `submittedByMe: Boolean!`.
  - Query: `itemByUpc(code: String!): Item`, `pendingItems(page, pageSize): ItemPage!` (admin).
  - Mutations: `submitItem(input: CreateItemInput!): Item!` (any authed user → pending), `approveItem(id)`/`rejectItem(id)` (admin), `setItemNutrients(itemId, entries)` (admin or creator-of-pending). `ItemNutrientEntryInput { nutrientId, amount }`.
- `resolver_inventory.go`:
  - `ItemByUpc`: digits-only validation; len 12 → `upc12`; 13 → `"0"+code` → `upc14`; 14 → `upc14`; else nil.
  - `Items`: user-scoped (pending items surface to their creator).
  - `Item(id)`: hidden when not visible to caller (`itemVisibleTo`).
  - `UpdateItem`, `AddFoodNutrient`, `RemoveFoodNutrient`, `AddFoodFlavor`, `RemoveFoodFlavor`: `requireAdmin` → `canModifyItem` (admin or pending-submitter).
  - Unique violations (name+brand, UPC) → `BAD_USER_INPUT`, not 500.
- `services.go`: extended `InventoryService`; `internal/bff/mock/` + `internal/inventory/sqlc/mock/` regenerated.

## Progress log

- **2026-09-08** — `mobile-redesign-p0` implemented, verified (build/vet/gofmt/`go test -short` green; golangci-lint only flags a vendored file inside `clients/web/node_modules`), merged via **PR #70**. Local `main` synced to `5ac33c0`. Next: create branch `mobile-redesign-p1` and implement the section below. Note: testcontainers integration tests were skipped under `-short` — run `go test ./internal/inventory -run Integration` (Docker) if DB-level verification is wanted.
- **2026-09-08** — `mobile-redesign-p1` implemented. `toggleGroceryItemChecked` now syncs catalog-item grocery check-offs to `inventory.user_item` in one `pgx` transaction, and the new `incrementUserItem(itemId, delta)` mutation adds/removes pantry stock with automatic delete at zero. Verified `go build ./...`, `go vet ./...`, `gofmt`, `go test -short ./...`, and `golangci-lint run ./...` (only the same vendored `clients/web/node_modules` file is flagged).

## mobile-redesign-p1 — Backend: grocery check-off ↔ inventory sync + scan quantity mutation

- `resolver_grocery.go` `ToggleGroceryItemChecked`: wrap toggle + inventory adjustment in **one pgx transaction** via `dbtx.InTx` on the shared pool, using `WithTx` instances of `grocery.Service` and `userprefs.Service`.
  - On check (`is_checked` false→true) with `item_id != nil`: `user_item.current_qty += quantity_needed` (create row if absent; preserve min_qty/expires/notes/is_favorite).
  - On uncheck: `current_qty -= quantity_needed`, floor 0 (keep the row at 0 — do not delete, to preserve prefs).
  - `manual_item_name`/ingredient-only rows: toggle only.
- New mutation `incrementUserItem(itemId: ID!, delta: Float!): UserItem` in `resolver_userprefs.go`: server-side read-modify-write in tx (`delta > 0` adds; `< 0` subtracts; result ≤ 0 → delete row and return null). Used by the mobile scan Add/Remove. Reuses `userprefs.UpsertUserItem`/`DeleteUserItem`.
- **Tests**: resolver + integration tests covering check→increment, uncheck→decrement, manual-item skip, qty floor at 0, delete-on-remove.

## mobile-redesign-p2 — Web: pending-items admin page

- `clients/web/lib/api.ts` + `types.ts`: `getPendingItems(page,pageSize)`, `approveItem(id)`, `rejectItem(id)`, `getItemByUpc(code)` (for future use), `Item.status`/`submittedByMe`.
- New page `clients/web/app/items/pending/page.tsx` — admin-gated (`useMe` role check, same pattern as `app/users/page.tsx`): table of pending items (name, brand, category, UPC, submitter, nutrients) with Approve / Reject buttons + confirm dialog.
- Nav: add "Pending Items" entry in `AdminLayout` gated on admin role; badge-style count optional.
- Jest tests for the page + api functions (`__tests__/`).

## mobile-redesign-p3 — Mobile: auth + app shell + Dashboard + grocery

- `pubspec.yaml`: add `google_sign_in`, `mobile_scanner`, `provider` (minimal footprint); keep `graphql_flutter`, `flutter_secure_storage`.
- **Auth** (`lib/auth/`):
  - `AuthService` (ChangeNotifier): `google_sign_in` sign-in with `serverClientId` = the **web OAuth client ID** so the returned ID token's `aud` matches `LENA_AUTH_AUDIENCES`; persist token under `id_token` in secure storage; expiry check via JWT `exp` decode; `signOut`.
  - `graphql_config.dart`: rebuild client with `AuthLink` reading the token + `ErrorLink`/`Link.exception` handling that signs out on 401/UNAUTHENTICATED. API base URL via `--dart-define=LENA_API_URL` (default `http://10.0.2.2:8080/graphql` for Android emulator; document LAN/IP override for physical devices and iOS simulator `localhost`).
  - `main.dart`: `AuthGate` — token present & unexpired → `MainScreen`, else `LoginScreen` ("Sign in with Google" button).
- **Platform config**: Android `minSdk`/camera permission for `mobile_scanner`; iOS `Info.plist` `NSCameraUsageDescription`, `GIDClientID`, reversed-client-ID URL scheme (documented in `clients/mobile/README.md`).
- **Dashboard** (`lib/screens/dashboard_screen.dart`): mirror web — today's meal slots from `mealPlans`/`mealPlan` (week-start date range + `dayOfWeek`), plus `recommendedRecipes` list. Bottom-nav shell: Dashboard | Grocery | Scan (center action) | Pantry.
- **Grocery** (`lib/screens/grocery_lists_screen.dart`, `grocery_list_screen.dart`): list of `groceryLists`, detail view with checkboxes → `toggleGroceryItemChecked` (inventory sync is server-side); pull-to-refresh; `addGroceryItem` manual entry; `generateGroceryList` from a chosen meal plan.
- **Pantry** (`lib/screens/pantry_screen.dart`): `userItems` list w/ quantities (needed for scan-remove context).
- Keep `items_screen`, `recipes_screen`, etc. as-is/placeholder tabs out of the nav.
- **Tests**: `flutter_test` widget tests for auth gate, dashboard render with mocked `GraphQLClient`, grocery toggle.

## mobile-redesign-p4 — Mobile: Scan Item flow

- `lib/screens/scan_screen.dart`: `mobile_scanner` camera view; debounce; normalize result (strip non-digits).
- Result flow (bottom sheet / pushed screen):
  - `itemByUpc(code)` → **found**: item card (name/brand/category/status badge if `pending`), quantity field (default 1), "Add to Inventory" → `incrementUserItem(+qty)`; "Remove from Inventory" → `incrementUserItem(-qty)` (confirm when result would delete the row).
  - **Not found**: `CreateItemScreen` — fields: name (required), brand dropdown limited to existing brands (`brands`/`frequentBrands`, optional — `createBrand` is admin-only so no free-text brand), category dropdown (`categories`, required), unit dropdown (`units`, required), UPC prefilled into `upc12`/`upc14` per length rule → `submitItem` → success message "submitted for approval".
  - After create: offer "Add nutrition info" → `NutritionEntryScreen`: list of `nutrientTypes` with amount fields → `setItemNutrients(itemId, entries)`. Same mutation the future OCR pipeline will call with extracted values — no throwaway code.
- My-submissions visibility: pending items show "Pending approval" badge via `status`/`submittedByMe`.
- **Tests**: widget tests for scan result states (found/not-found/create/nutrition) with mocked link; unit tests for the UPC length-normalization helper.

## mobile-redesign-p5 — Mobile CI + docs (small)

- `test.yml`: add `mobile` job — `subosito/flutter-action`, `flutter pub get`, `flutter analyze`, `flutter test` (no device needed). Add `mobile-redesign-*` to the push branch filter if the workflow should run on these branches.
- Update `clients/mobile/README.md` (Google client-id config for Android `serverClientId` + iOS `GIDClientID`, dart-define API URL, emulator notes), `docs/mobil-front-end.md` (mark superseded for LENA2 or rewrite), `docs/graphql-schema.md` (new queries/mutations), `docs/postgres-data-model.md` (item approval columns), and `AGENTS.md` (note the named-phase branch convention).

## Out of scope / follow-ups

- OCR nutrition-label capture (phase B goal — `setItemNutrients` is the seam it plugs into).
- Recipe/meal-plan/wine mobile screens; offline support; push notifications.
- External product DB (OpenFoodFacts) lookup.
- Rejected-submission cleanup job; user notification on approval.

## Risks

- **Visibility filter breadth**: `ListItems`/`CountItems` signature change ripples through web `api.getItems` callers and picker resolvers — audit all `ListItems` call sites; recipe/grocery pickers batch-load via `GetItemsByIDs` (unfiltered — acceptable, since FK references are created by the owner, but note for review).
- **Google `serverClientId` on iOS** must be the *iOS* client ID for `GIDClientID` while `serverClientId` stays the web client ID — a common misconfiguration; README must be explicit.
- **Cross-service tx** in `ToggleGroceryItemChecked` needs a shared-pool `WithTx` path — verify `dbtx`/`pgxpool` tx helper exists (used by `InTx` in domain services).
- **`UNIQUE(name, brand_id)`** collision for user-submitted duplicates → friendly error, not 500.
