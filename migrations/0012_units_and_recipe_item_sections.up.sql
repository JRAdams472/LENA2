-- Shared unit lookup table. kind enables future unit conversion
-- (volume <-> volume, weight <-> weight); 'count' covers discrete units.
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

INSERT INTO inventory.unit (name, abbreviation, kind, created_by) VALUES
    ('teaspoon',   'tsp',   'volume', 'migration'),
    ('tablespoon', 'tbsp',  'volume', 'migration'),
    ('cup',        'c',     'volume', 'migration'),
    ('fluid ounce','fl oz', 'volume', 'migration'),
    ('milliliter', 'ml',    'volume', 'migration'),
    ('liter',      'l',     'volume', 'migration'),
    ('pint',       'pt',    'volume', 'migration'),
    ('quart',      'qt',    'volume', 'migration'),
    ('gallon',     'gal',   'volume', 'migration'),
    ('gram',       'g',     'weight', 'migration'),
    ('kilogram',   'kg',    'weight', 'migration'),
    ('ounce',      'oz',    'weight', 'migration'),
    ('pound',      'lb',    'weight', 'migration'),
    ('each',       'ea',    'count',  'migration'),
    ('pinch',      NULL,    'count',  'migration'),
    ('clove',      NULL,    'count',  'migration'),
    ('can',        NULL,    'count',  'migration'),
    ('package',    'pkg',   'count',  'migration'),
    ('bunch',      NULL,    'count',  'migration'),
    ('slice',      NULL,    'count',  'migration')
ON CONFLICT (name) DO NOTHING;

-- Convert free-form unit columns to unit_id foreign keys. Backfill matches
-- lower(trim(unit)) against the unit name or abbreviation; anything
-- unmatched falls back to 'each' so NOT NULL columns can be enforced.

ALTER TABLE inventory.item ADD COLUMN unit_id BIGINT;
UPDATE inventory.item i
SET unit_id = u.unit_id
FROM inventory.unit u
WHERE lower(trim(i.unit)) = u.name OR lower(trim(i.unit)) = u.abbreviation;
UPDATE inventory.item SET unit_id = (SELECT unit_id FROM inventory.unit WHERE name = 'each') WHERE unit_id IS NULL;
ALTER TABLE inventory.item
    ALTER COLUMN unit_id SET NOT NULL,
    ADD CONSTRAINT item_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES inventory.unit(unit_id),
    DROP COLUMN unit;

ALTER TABLE recipe.recipe_item ADD COLUMN unit_id BIGINT;
UPDATE recipe.recipe_item ri
SET unit_id = u.unit_id
FROM inventory.unit u
WHERE lower(trim(ri.unit)) = u.name OR lower(trim(ri.unit)) = u.abbreviation;
UPDATE recipe.recipe_item SET unit_id = (SELECT unit_id FROM inventory.unit WHERE name = 'each') WHERE unit_id IS NULL;
ALTER TABLE recipe.recipe_item
    ALTER COLUMN unit_id SET NOT NULL,
    ADD CONSTRAINT recipe_item_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES inventory.unit(unit_id),
    DROP COLUMN unit;

ALTER TABLE mealplan.meal_slot_item ADD COLUMN unit_id BIGINT;
UPDATE mealplan.meal_slot_item msi
SET unit_id = u.unit_id
FROM inventory.unit u
WHERE lower(trim(msi.unit)) = u.name OR lower(trim(msi.unit)) = u.abbreviation;
UPDATE mealplan.meal_slot_item SET unit_id = (SELECT unit_id FROM inventory.unit WHERE name = 'each') WHERE unit_id IS NULL;
ALTER TABLE mealplan.meal_slot_item
    ALTER COLUMN unit_id SET NOT NULL,
    ADD CONSTRAINT meal_slot_item_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES inventory.unit(unit_id),
    DROP COLUMN unit;

ALTER TABLE grocery.grocery_list_item ADD COLUMN unit_id BIGINT;
UPDATE grocery.grocery_list_item gli
SET unit_id = u.unit_id
FROM inventory.unit u
WHERE gli.unit_of_measure IS NOT NULL
  AND (lower(trim(gli.unit_of_measure)) = u.name OR lower(trim(gli.unit_of_measure)) = u.abbreviation);
ALTER TABLE grocery.grocery_list_item
    ADD CONSTRAINT grocery_list_item_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES inventory.unit(unit_id),
    DROP COLUMN unit_of_measure;

ALTER TABLE inventory.ingredient ADD COLUMN default_unit_id BIGINT;
UPDATE inventory.ingredient i
SET default_unit_id = u.unit_id
FROM inventory.unit u
WHERE i.default_unit IS NOT NULL
  AND (lower(trim(i.default_unit)) = u.name OR lower(trim(i.default_unit)) = u.abbreviation);
ALTER TABLE inventory.ingredient
    ADD CONSTRAINT ingredient_default_unit_id_fkey FOREIGN KEY (default_unit_id) REFERENCES inventory.unit(unit_id),
    DROP COLUMN default_unit;

-- Recipe item sections and ordering. A surrogate key replaces
-- (recipe_id, item_id) so the same item can appear in multiple sections
-- (e.g. "flour" in both "crust" and "filling").
ALTER TABLE recipe.recipe_item
    ADD COLUMN recipe_item_id BIGSERIAL,
    ALTER COLUMN recipe_item_id SET NOT NULL,
    ADD COLUMN section_name VARCHAR(200),
    ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0,
    DROP CONSTRAINT recipe_item_pkey,
    ADD PRIMARY KEY (recipe_item_id);
CREATE INDEX idx_recipe_item_recipe ON recipe.recipe_item (recipe_id, display_order);
