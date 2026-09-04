# LENA — Migration & Seeding Runbook

## 1. Philosophy

This rewrite is a **fresh start**. No data is migrated from the existing SQL Server database. However, reference data (categories, nutrient types, wine countries) can be seeded to make the app usable immediately.

## 2. Tooling

- **Migrations**: `golang-migrate/migrate`
- **Query code generation**: `sqlc`
- **Seed data**: plain `.sql` files in `migrations/seed/*.sql`

## 3. Directory Layout

```
migrations/
  0001_identity.up.sql
  0001_identity.down.sql
  0002_inventory.up.sql
  0002_inventory.down.sql
  0003_wine.up.sql
  0003_wine.down.sql
  0004_recipe.up.sql
  0004_recipe.down.sql
  0005_userprefs.up.sql
  0005_userprefs.down.sql
  0006_mealplan.up.sql
  0006_mealplan.down.sql
  0007_grocery.up.sql
  0007_grocery.down.sql
  seed/
    0001_inventory_ref.up.sql
    0002_wine_ref.up.sql
```

## 4. Migration Order

1. `identity` — users table.
2. `inventory` — catalog tables.
3. `wine` — wine catalog.
4. `recipe` — recipe catalog.
5. `userprefs` — per-user tables.
6. `mealplan` — meal planning.
7. `grocery` — grocery lists.

This ordering ensures foreign key dependencies are satisfied.

## 5. Running Migrations

### Local

```bash
migrate -path ./migrations -database "postgres://lena_app:${LENA_DB_PASSWORD}@localhost:5432/lena?sslmode=disable" up
```

### Docker

A one-shot `db-migrate` service runs `golang-migrate` before the API starts:

```yaml
  db-migrate:
    image: migrate/migrate
    command: ["-path", "/migrations", "-database", "postgres://lena_app:${LENA_DB_PASSWORD}@db:5432/lena?sslmode=disable", "up"]
    volumes:
      - ./migrations:/migrations:ro
    depends_on:
      db:
        condition: service_healthy
```

## 6. Seed Data

Seed scripts insert default reference data only. They are **not** `golang-migrate` versioned files; they are applied idempotently via `INSERT ... ON CONFLICT DO NOTHING`.

```sql
-- migrations/seed/0001_inventory_ref.up.sql
INSERT INTO inventory.category (name, created_by) VALUES
  ('Produce', 'seed'),
  ('Dairy', 'seed'),
  ('Meat', 'seed')
ON CONFLICT (name) DO NOTHING;

INSERT INTO inventory.nutrient_type (name, unit, created_at) VALUES
  ('Protein', 'g', now()),
  ('Carbohydrates', 'g', now())
ON CONFLICT (name) DO NOTHING;
```

## 7. sqlc

`sqlc` is configured per module. Example `sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/inventory/queries.sql"
    schema: "migrations/0002_inventory.up.sql"
    gen:
      go:
        package: "inventory"
        out: "internal/inventory/sqlc"
        sql_package: "pgx/v5"
```

Run:

```bash
sqlc generate
```

## 8. Test Data

For integration tests, `testcontainers-go` spins up a Postgres container and applies the same migration path.

```go
func TestMain(m *testing.M) {
    // start container, run migrations, run tests
}
```

## 9. Resetting

To start over:

```bash
migrate -path ./migrations -database "$DATABASE" down -all
docker compose down -v   # if using Docker volumes
```

## 10. No Data Migration

- Existing SQL Server data is intentionally not migrated.
- If migration is needed later, a separate one-time ETL tool can be written after this rewrite is complete.