# LENA — PostgreSQL Data Model

## 1. Schema Layout

Each domain lives in its own PostgreSQL schema to keep boundaries explicit:

```sql
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS wine;
CREATE SCHEMA IF NOT EXISTS recipe;
CREATE SCHEMA IF NOT EXISTS mealplan;
CREATE SCHEMA IF NOT EXISTS grocery;
```

## 2. SQL Server → PostgreSQL Type Mapping

| SQL Server | PostgreSQL |
|---|---|
| `INT IDENTITY(1,1)` | `BIGSERIAL` |
| `DATETIME2` | `TIMESTAMPTZ` |
| `DECIMAL(10,2)` | `NUMERIC(10,2)` |
| `DECIMAL(5,2)` | `NUMERIC(5,2)` |
| `TINYINT` | `SMALLINT` (with `CHECK 0..255` where needed) |
| `BIT` | `BOOLEAN` |
| `NVARCHAR(n)` | `VARCHAR(n)` or `TEXT` |

## 3. Identity

```sql
CREATE TABLE identity.users (
    user_id             BIGSERIAL PRIMARY KEY,
    provider            VARCHAR(50) NOT NULL DEFAULT 'google',
    external_subject    VARCHAR(255) NOT NULL,
    email               VARCHAR(320) NOT NULL,
    display_name        VARCHAR(200),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at       TIMESTAMPTZ,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ,
    UNIQUE (provider, external_subject)
);
CREATE INDEX idx_users_email ON identity.users (email);
```

## 4. Inventory (catalog + per-user)

```sql
CREATE TABLE inventory.category (
    category_id   BIGSERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL UNIQUE,
    description   VARCHAR(500),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ
);

CREATE TABLE inventory.brand (
    brand_id   BIGSERIAL PRIMARY KEY,
    name       VARCHAR(200) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory.item (
    item_id     BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    brand_id    BIGINT REFERENCES inventory.brand(brand_id),
    upc12       VARCHAR(12),
    upc14       VARCHAR(14),
    category_id BIGINT NOT NULL REFERENCES inventory.category(category_id),
    unit_id     BIGINT NOT NULL REFERENCES inventory.unit(unit_id),
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    UNIQUE (name, brand_id),
    UNIQUE (upc12),
    UNIQUE (upc14)
);

CREATE TABLE inventory.user_item (
    user_item_id BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    item_id      BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    current_qty  NUMERIC(10,2) NOT NULL DEFAULT 0,
    min_qty      NUMERIC(10,2),
    purchase_at  TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    notes        VARCHAR(500),
    is_favorite  BOOLEAN NOT NULL DEFAULT FALSE,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ,
    UNIQUE (user_id, item_id)
);
CREATE INDEX idx_user_item_item_id ON inventory.user_item (item_id);
```

### 4.1 Inventory reference tables

```sql
CREATE TABLE inventory.flavor_profile (
    flavor_id   BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE inventory.food_flavor (
    food_id       BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    flavor_id     BIGINT NOT NULL REFERENCES inventory.flavor_profile(flavor_id),
    intensity     SMALLINT NOT NULL CHECK (intensity BETWEEN 1 AND 5),
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (food_id, flavor_id)
);

CREATE TABLE inventory.nutrient_type (
    nutrient_id   BIGSERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL UNIQUE,
    unit          VARCHAR(50),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory.food_nutrient (
    food_id      BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    nutrient_id  BIGINT NOT NULL REFERENCES inventory.nutrient_type(nutrient_id),
    amount       NUMERIC(10,4) NOT NULL,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (food_id, nutrient_id)
);

-- Brand-agnostic generic ingredient (e.g. "all-purpose flour"). Branded
-- items exist for barcode scanning; recipes, meal slots and grocery lists
-- can reference an ingredient instead. Scaffolding only — nothing
-- populates this data yet.
CREATE TABLE inventory.ingredient (
    ingredient_id   BIGSERIAL PRIMARY KEY,
    name            VARCHAR(200) NOT NULL UNIQUE,
    category_id     BIGINT REFERENCES inventory.category(category_id),
    default_unit_id BIGINT REFERENCES inventory.unit(unit_id),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

-- Canonical unit of measure shared by every domain. kind enables future
-- unit conversion (volume <-> volume, weight <-> weight); 'count' covers
-- discrete units. Seeds cover common cooking units; unknown inputs are
-- rejected at the BFF layer rather than stored.
CREATE TABLE inventory.unit (
    unit_id      BIGSERIAL PRIMARY KEY,
    name         VARCHAR(50) NOT NULL UNIQUE,
    abbreviation VARCHAR(20),
    kind         VARCHAR(20) NOT NULL DEFAULT 'count' CHECK (kind IN ('volume', 'weight', 'count')),
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ
);
```

## 5. Wine (catalog + per-user)

```sql
CREATE TABLE wine.country (
    country_id  BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    iso_code    VARCHAR(3) NOT NULL UNIQUE,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.region (
    region_id   BIGSERIAL PRIMARY KEY,
    country_id  BIGINT NOT NULL REFERENCES wine.country(country_id),
    name        VARCHAR(200) NOT NULL,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.type (
    type_id     BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL UNIQUE,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.vintage (
    vintage_id  BIGSERIAL PRIMARY KEY,
    year        INTEGER NOT NULL UNIQUE,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.grape_variety (
    grape_variety_id BIGSERIAL PRIMARY KEY,
    name             VARCHAR(200) NOT NULL UNIQUE,
    description      VARCHAR(500),
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_by       VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       VARCHAR(100),
    updated_at       TIMESTAMPTZ
);

CREATE TABLE wine.flavor_profile (
    flavor_profile_id BIGSERIAL PRIMARY KEY,
    name              VARCHAR(200) NOT NULL UNIQUE,
    description       VARCHAR(500),
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        VARCHAR(100),
    updated_at        TIMESTAMPTZ
);

CREATE TABLE wine.bottle (
    bottle_id       BIGSERIAL PRIMARY KEY,
    type_id         BIGINT NOT NULL REFERENCES wine.type(type_id),
    country_id      BIGINT NOT NULL REFERENCES wine.country(country_id),
    region_id       BIGINT NOT NULL REFERENCES wine.region(region_id),
    vintage_year    INTEGER NOT NULL,
    vineyard        VARCHAR(200),
    abv             NUMERIC(5,2),
    acidity         SMALLINT,
    tannin_level    SMALLINT,
    body            SMALLINT,
    sweetness       SMALLINT,
    oak_integration BOOLEAN,
    bottle_size     VARCHAR(20) NOT NULL DEFAULT '750ml',
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

CREATE TABLE wine.user_bottle (
    user_bottle_id BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    bottle_id      BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    bottle_number  INTEGER,
    quantity       INTEGER NOT NULL DEFAULT 1,
    purchase_at    TIMESTAMPTZ,
    purchase_price NUMERIC(10,2),
    storage_temp   NUMERIC(5,1),
    location       VARCHAR(100),
    notes          VARCHAR(500),
    is_favorite    BOOLEAN NOT NULL DEFAULT FALSE,
    created_by     VARCHAR(100) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     VARCHAR(100),
    updated_at     TIMESTAMPTZ
);
```

## 6. Recipe

```sql
CREATE TABLE recipe.recipe (
    recipe_id         BIGSERIAL PRIMARY KEY,
    name              VARCHAR(200) NOT NULL UNIQUE,
    description       VARCHAR(500),
    servings          INTEGER,
    prep_time_minutes INTEGER,
    cook_time_minutes INTEGER,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        VARCHAR(100),
    updated_at        TIMESTAMPTZ
);

CREATE TABLE recipe.recipe_item (
    recipe_item_id BIGSERIAL PRIMARY KEY,
    recipe_id      BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    item_id        BIGINT NOT NULL REFERENCES inventory.item(item_id),
    ingredient_id  BIGINT REFERENCES inventory.ingredient(ingredient_id),
    quantity       NUMERIC(10,4) NOT NULL,
    unit_id        BIGINT NOT NULL REFERENCES inventory.unit(unit_id),
    section_name   VARCHAR(200),
    display_order  INTEGER NOT NULL DEFAULT 0,
    notes          VARCHAR(500),
    is_optional    BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX idx_recipe_item_recipe ON recipe.recipe_item (recipe_id, display_order);

CREATE TABLE recipe.recipe_step (
    step_id       BIGSERIAL PRIMARY KEY,
    recipe_id     BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    step_number   INTEGER NOT NULL,
    instruction   VARCHAR(2000) NOT NULL,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    UNIQUE (recipe_id, step_number)
);

CREATE TABLE recipe.user_recipe_preference (
    user_id      BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    recipe_id    BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    is_favorite  BOOLEAN NOT NULL DEFAULT FALSE,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ,
    PRIMARY KEY (user_id, recipe_id)
);
```

## 7. Meal Plan

```sql
CREATE TABLE mealplan.meal_plan (
    meal_plan_id        BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    name                VARCHAR(200) NOT NULL,
    week_start_date     DATE NOT NULL,
    week_start_day_of_week SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ
);
CREATE INDEX idx_meal_plan_user_week ON mealplan.meal_plan (user_id, week_start_date);

CREATE TABLE mealplan.meal_slot (
    slot_id       BIGSERIAL PRIMARY KEY,
    meal_plan_id  BIGINT NOT NULL REFERENCES mealplan.meal_plan(meal_plan_id) ON DELETE CASCADE,
    day_of_week   SMALLINT NOT NULL,
    meal_type     VARCHAR(50) NOT NULL,
    recipe_id     BIGINT REFERENCES recipe.recipe(recipe_id),
    servings      INTEGER,
    replacement_note VARCHAR(500),
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    UNIQUE (meal_plan_id, day_of_week, meal_type)
);

CREATE TABLE mealplan.meal_slot_item (
    slot_item_id  BIGSERIAL PRIMARY KEY,
    slot_id       BIGINT NOT NULL REFERENCES mealplan.meal_slot(slot_id) ON DELETE CASCADE,
    item_id       BIGINT REFERENCES inventory.item(item_id),
    ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id),
    quantity      NUMERIC(10,4) NOT NULL,
    unit_id       BIGINT NOT NULL REFERENCES inventory.unit(unit_id),
    is_from_recipe BOOLEAN NOT NULL DEFAULT FALSE,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ
);
```

## 8. Grocery

```sql
CREATE TABLE grocery.grocery_list (
    grocery_list_id BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    meal_plan_id    BIGINT REFERENCES mealplan.meal_plan(meal_plan_id),
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

CREATE TABLE grocery.grocery_list_item (
    grocery_list_item_id BIGSERIAL PRIMARY KEY,
    grocery_list_id      BIGINT NOT NULL REFERENCES grocery.grocery_list(grocery_list_id) ON DELETE CASCADE,
    item_id              BIGINT REFERENCES inventory.item(item_id),
    ingredient_id        BIGINT REFERENCES inventory.ingredient(ingredient_id),
    manual_item_name     VARCHAR(200),
    quantity_needed      NUMERIC(10,4) NOT NULL,
    unit_id              BIGINT REFERENCES inventory.unit(unit_id),
    source               VARCHAR(50) NOT NULL,
    is_checked           BOOLEAN NOT NULL DEFAULT FALSE,
    created_by           VARCHAR(100) NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           VARCHAR(100),
    updated_at           TIMESTAMPTZ
);
```

## 9. Data-Isolation Notes

- `identity.users` is the only table referenced by foreign keys from other schemas for scoping.
- All catalog tables are global and may be mutated by any authenticated user initially.
- Per-user tables `ON DELETE CASCADE` from `users` so deleting a user removes their data.
- No cross-schema joins are allowed in module code; the BFF assembles cross-domain data.