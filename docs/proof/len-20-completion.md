# LEN-20 — Allergy P1: schema + data (proof of completion)

Phase 1 of the LEN-16 allergy plan: allergen taxonomy, member records,
ingredient/item allergen links, and the reviewed curation seed.

## What landed

### Migration `0047_allergen`
- `inventory.allergen` — global registry, normalized-unique name index
  (same expression as `idx_ingredient_name_norm`), `is_active`, audit
  columns. Seeded with 19 entries: the EU-14/FDA-9 regulatory set plus
  four dietary-restriction entries (`corn`, `gelatin`, `pork`, `beef`)
  for member records with `kind='dietary'`.
- `inventory.ingredient_allergen(ingredient_id, allergen_id, kind)` —
  canonical allergen knowledge; `kind ∈ {contains, may_contain}`.
- `inventory.item_allergen(item_id, allergen_id, kind)` — product-level
  additive flags (shared-facility "may contain", formulation outliers).
- `userprefs.user_allergen(user_id, allergen_id, kind)` — per-member
  records; `kind ∈ {allergy, dietary}`; PK `(user_id, allergen_id)`.
- Explicit `GRANT`s to `lena_app` (belt-and-suspenders over the
  schema-level default privileges, matching 0036's convention).
- Verified: `migrate up` → `down 1` → `up` all clean on the dev DB.

### `cmd/allergencurate`
Mirrors `cmd/ingredientcurate`: `propose` batches active ingredients
through the configured `llm.Provider` into a reviewable JSON artifact;
`apply` resolves allergen names against the registry and upserts
`ingredient_allergen` idempotently (`ON CONFLICT … DO UPDATE`, no-op on
unchanged kind). Unknown allergen names fail loudly — the registry is
admin-managed. `-dry-run` supported.

### Curated seed
Two propose→review→apply passes against the dev DB (qwen2.5:7b-instruct):

- Pass 1: 219 proposed → **183 applied** after review.
- Pass 2 (auto-selected the 231 still-unflagged): 76 proposed →
  **34 applied** after review.
- Result: **216 of 413 active ingredients flagged, 326
  `ingredient_allergen` rows**, 18 of 19 allergens in use (`lupin`
  seeded for admin use, currently unused).

Review corrections mattered — the 7B model produced material noise
(meat flagged with milk, `pecans → peanuts`, `oyster sauce →
crustacean`, `corn → wheat`, invented allergens like "tobacco"). The
review pass fixed wrong flags, dropped ~80 unsupportable mappings into
`unresolved` with reasons, remapped invalid names (`whey → milk`,
`shellfish → crustacean shellfish`), and systematically added `gluten`
alongside every `wheat` flag since a member records `gluten` for celiac
while wheat always implies it.

Reviewed artifacts committed for audit:
`docs/allergen-curation.json`, `docs/allergen-curation-pass2.json`.

The ~197 unflagged ingredients are mostly produce, plain proteins, and
single spices with genuinely no allergen — an honest "no allergen info"
state, which the P2/P3 surfaces must never render as "known safe".

## Verification

- `go build ./...` — clean
- `go vet ./cmd/allergencurate` — clean
- `go test ./cmd/...` — all packages pass, including 3 new
  `allergencurate` test functions (batch parsing, flag normalization,
  artifact schema round-trip)
- Migration `up`/`down`/`up` cycle clean; registry verified at 19 rows
- `apply` run twice (dry-run then real) — deterministic, idempotent
  (`ON CONFLICT … IS DISTINCT FROM` skips rewrites)

## Deferred to later phases

- sqlc queries + resolvers consuming these tables — P2
- Item-level flags (`item_allergen`) and member records — schema ships
  here, mutations ship in P2
- AI recipe-text flagging — P4
