-- Explicit nutrient basis for inventory.food_nutrient (audit A2-06).
-- Previously `amount` had no declared basis, so meal-plan aggregation
-- multiplied it by arbitrary recipe quantities. The basis is stored per
-- row as "amount per basis_quantity of basis_unit_id" and defaults to the
-- USDA-style convention of per 100 grams.

ALTER TABLE inventory.food_nutrient
    ADD COLUMN basis_quantity NUMERIC(10,4),
    ADD COLUMN basis_unit_id  BIGINT REFERENCES inventory.unit(unit_id);

UPDATE inventory.food_nutrient
SET basis_quantity = 100,
    basis_unit_id  = (SELECT unit_id FROM inventory.unit WHERE name = 'gram' LIMIT 1);

ALTER TABLE inventory.food_nutrient
    ALTER COLUMN basis_quantity SET NOT NULL,
    ALTER COLUMN basis_unit_id  SET NOT NULL;
