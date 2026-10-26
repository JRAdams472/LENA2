# LENA — Migration & Seeding Runbook

## 1. Philosophy

Migrations own the schema *and* the data the application cannot function
without (roles, units, constraint support rows, catalog repairs).
Optional reference data (pantry categories, nutrient types, wine
countries/regions/grapes) lives under `migrations/seed/` and is applied
by a separate, opt-in process — never by the migration chain itself.

No data is carried over from the legacy SQL Server database.

## 2. Tooling

- **Migrations**: `golang-migrate/migrate` (append-only `NNNN_name.up.sql`
  / `.down.sql` pairs — the `.up.sql` suffix is required)
- **Query code generation**: `sqlc`
- **Seed data**: plain `.sql` files in `migrations/seed/*.sql`, applied
  idempotently and tracked in `public.schema_seed`

## 3. Directory Layout

```
migrations/
  0001_users.up.sql                 # identity
  0001_users.down.sql
  …                                 # 0058+ today; append-only, never edit
  seed/                             # NOT golang-migrate files
    0001_reference_data.sql         # categories, nutrient types, wine refs
    0002_inventory_seed.sql         # catalog brands/items (uses seeded cats)
```

## 4. Critical Ordering Rule

**Migrations must not depend on seed data.** The seed files run under
`--profile seed`, an opt-in path that many deployments never execute. A
migration that joins seeded rows will silently succeed and apply *nothing*
— that is exactly what happened to `0046_ingredient_dedupe_seed.up.sql`,
which inner-joined `inventory.category` and recorded success while
inserting zero ingredients on unseeded databases.

The repair pattern (see `0059_ingredient_catalog_repair.up.sql`) is:

1. `INSERT` the reference rows the migration needs, with
   `ON CONFLICT … DO NOTHING` (units and categories are both safe to
   create — `inventory.unit` is already populated by migration 0012, and
   the five pantry categories are now created by 0059 itself).
2. Replay the payload through a uniqueness guard that matches the
   table's own index (e.g. the normalized-name `NOT EXISTS` in 0059, which
   mirrors `idx_ingredient_name_norm`), so reruns are idempotent and
   already-present rows are left untouched.

If a migration needs rows that only the seed provides, it must create
them first — a join alone is a latent zero-row bug.

## 5. Running Migrations

### Docker (normal path)

`docker-compose.yml` wires this automatically:

- `db-migrate` runs `golang-migrate up` as the **`POSTGRES_USER`
  superuser** — migrations create schemas, extensions, and roles, so the
  connection must be DDL-capable. Do not point it at `lena_app`.
- `db-grant` then sets the `lena_app` password from
  `LENA_DB_PASSWORD` via `psql -v` quoting (safe with special
  characters).
- `api` waits on both completing successfully.

### Local

```bash
migrate -path ./migrations \
  -database "postgres://<superuser>:${POSTGRES_PASSWORD}@localhost:5432/lena?sslmode=disable" up
# then set the app role password
psql "$ADMIN_DSN" -v pw="$LENA_DB_PASSWORD" \
  -c "ALTER ROLE lena_app WITH PASSWORD :'pw'"
```

## 6. Seed Data

Seed scripts insert reference data only. They are not `golang-migrate`
versioned files; `db-seed` applies each file once, recording the filename
in `public.schema_seed` so repeated `--profile seed up` runs are no-ops:

```bash
docker compose --profile seed up db-seed
```

Because seeds are optional and unordered relative to app traffic,
application code and later migrations must tolerate a catalog that is
empty or partially populated. `0002_inventory_seed.sql` assumes
`0001_reference_data.sql` ran first (it joins categories by name) — keep
the `0001` → `0002` ordering if you add seed files.

## 7. sqlc

`sqlc` is configured in `sqlc.yaml`; generated code lands in
`internal/*/sqlc/`. After touching `migrations/` or
`internal/*/queries*.sql`:

```bash
sqlc generate
```

Commit the generated output — CI verifies it is in sync.

## 8. Test Data

Integration tests use `testcontainers-go` and apply the same migration
path — tests therefore run against an unseeded database unless they
insert their own fixtures. If a test needs reference data, insert it in
the test; do not assume the seed ran.

## 9. Resetting

```bash
docker compose down -v            # drops pg_data entirely
# or, on a local database:
migrate -path ./migrations -database "$DATABASE" down -all
```

`schema_seed` is not migrated — reset it with the data volume, or delete
rows manually to re-run a seed file.

## 10. Repairing an Already-Applied Bad Migration

When a migration was recorded successful but applied incorrectly (the
0046 case), never edit it in place — `golang-migrate` checksums and
already-run databases won't re-execute it anyway. Instead:

1. Add a new `NNNN_repair` migration that reaches the desired state
   idempotently (create prerequisites, guard on real uniqueness, leave
   existing rows alone).
2. Write its `.down.sql` conservatively — only remove rows that provably
   came from the migration (`created_by = 'seed'`) **and** are not
   referenced by application data (`NOT EXISTS` FK probes on
   `inventory.item`, `inventory.ingredient_nutrient`, etc.).
3. For a truly broken recorded version on a live DB, fix forward — or as
   a last resort `migrate -path ./migrations -database "$DSN" force <v>`
   to clear a `dirty` flag, then re-run `up`. `force` never re-applies a
   clean version.
