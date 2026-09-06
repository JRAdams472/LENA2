-- Reverse recipe_item section/ordering changes. Restoring the composite
-- (recipe_id, item_id) primary key will fail if duplicate
-- (recipe_id, item_id) rows were created while sections existed.
DROP INDEX idx_recipe_item_recipe;
ALTER TABLE recipe.recipe_item
    DROP CONSTRAINT recipe_item_pkey,
    DROP COLUMN recipe_item_id,
    DROP COLUMN section_name,
    DROP COLUMN display_order,
    ADD PRIMARY KEY (recipe_id, item_id);

ALTER TABLE inventory.ingredient ADD COLUMN default_unit VARCHAR(20);
UPDATE inventory.ingredient i
SET default_unit = u.name
FROM inventory.unit u
WHERE i.default_unit_id = u.unit_id;
ALTER TABLE inventory.ingredient
    DROP CONSTRAINT ingredient_default_unit_id_fkey,
    DROP COLUMN default_unit_id;

ALTER TABLE grocery.grocery_list_item ADD COLUMN unit_of_measure VARCHAR(20);
UPDATE grocery.grocery_list_item gli
SET unit_of_measure = u.name
FROM inventory.unit u
WHERE gli.unit_id = u.unit_id;
ALTER TABLE grocery.grocery_list_item
    DROP CONSTRAINT grocery_list_item_unit_id_fkey,
    DROP COLUMN unit_id;

ALTER TABLE mealplan.meal_slot_item ADD COLUMN unit VARCHAR(20);
UPDATE mealplan.meal_slot_item msi
SET unit = u.name
FROM inventory.unit u
WHERE msi.unit_id = u.unit_id;
ALTER TABLE mealplan.meal_slot_item
    DROP CONSTRAINT meal_slot_item_unit_id_fkey,
    ALTER COLUMN unit SET NOT NULL,
    DROP COLUMN unit_id;

ALTER TABLE recipe.recipe_item ADD COLUMN unit VARCHAR(20);
UPDATE recipe.recipe_item ri
SET unit = u.name
FROM inventory.unit u
WHERE ri.unit_id = u.unit_id;
ALTER TABLE recipe.recipe_item
    DROP CONSTRAINT recipe_item_unit_id_fkey,
    ALTER COLUMN unit SET NOT NULL,
    DROP COLUMN unit_id;

ALTER TABLE inventory.item ADD COLUMN unit VARCHAR(20);
UPDATE inventory.item i
SET unit = u.name
FROM inventory.unit u
WHERE i.unit_id = u.unit_id;
ALTER TABLE inventory.item
    DROP CONSTRAINT item_unit_id_fkey,
    ALTER COLUMN unit SET NOT NULL,
    DROP COLUMN unit_id;

DROP TABLE inventory.unit;
