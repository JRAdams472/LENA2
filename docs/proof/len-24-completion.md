# LEN-24 — P5 allergy plan close-out: completion proof

**Ticket:** LEN-24 (LEN-16 P5)
**Scope:** docs/wiki/screenshots/audit re-walk — no feature code changed.

## Audit re-walk

Re-walked `audit/summary.md` + `audit/remediation/README.md` against the
allergy work (LEN-20..23). No regressions introduced; the implementation
follows the fixed patterns (`:execrows` + `ErrNotFound`,
`WHERE status='pending'` guards, `dbtx.InTx` for the accept flag-write +
status flip, `domainerr`, `@admin` gating). No new findings.

## Docs updated

- `README.md` — allergen flags, warning surfaces, suggestion queue in
  feature/inventory/admin/mobile/AI sections.
- `docs/graphql-schema.md` — allergen registry, member records
  (`myAllergies`/`setMyAllergy`), flag mutations, `allergyWarnings` fields,
  `allergenSuggestions` + suggest/accept/dismiss mutations (`@admin`),
  `AllergenFlag`/`MemberAllergen`/`AllergyWarning`/`AllergenSuggestion`
  types.
- `docs/postgres-data-model.md` — `inventory.allergen`,
  `ingredient_allergen`, `item_allergen`, `userprefs.user_allergen`,
  `allergen_suggestion` (verified against migrations 0047/0048).
- `clients/mobile/README.md` — allergy editor on Household tab + warning
  surfaces.
- `docs/wiki-screenshots.md` — new seed content + the SQL insert step for
  the suggestion queue.

## Wiki

- New page `Allergy-Tracking.md` — member records, warning surfaces,
  "no allergen information" semantics, registry/flag curation, AI review
  queue, mobile.
- Cross-links/sections added: `Home`, `_Sidebar`, `Ingredients`,
  `Inventory-and-Pantry`, `Recipes` (warning section + shot),
  `AI-Assistant` (suggestion-queue row), `Meal-Planning`, `Grocery-Lists`,
  `Food-Events`, `Households`, `Mobile-App`, `Mobile-Household`,
  `Mobile-Grocery-Lists`.
- Commits: `d11745a`, `961b205`.

## Screenshots (lena2shots stack)

New captures, all inspected — no spinners, real seeded data:

- `allergen-registry.png` — `/inventory/allergens` taxonomy table
- `allergen-suggestions.png` — `/inventory/allergen-suggestions` with a
  pending proposal row + Accept/Dismiss
- `profile-allergies.png` — profile allergy card, `milk → Allergy` selected
- `recipe-allergy-warning.png` — recipe detail alert + allergen chips
- `mobile-allergies.png` / `mobile-allergy-warning.png` — emulator shots of
  the Household allergy editor and the grocery-list warning badge
  (debug APK built with `LENA_DEBUG_ID_TOKEN` from the shots test issuer)

Plus the full canonical shot list re-run — all existing images refreshed.

`tools/wiki-shots/seed_demo.py` now seeds member allergy records +
item allergen flags; `capture.mjs` adds the four web shots above and seeds
a pending `allergen_suggestion` row via `docker exec` psql (no non-AI write
path exists — intentional).

## Verification

- `go build ./...` ✅
- `go test ./...` / `go vet` / `golangci-lint` — see CI
- Web: `tsc` / `eslint` / jest — see CI
- No feature code changed in this phase; docs + shot tooling only.
