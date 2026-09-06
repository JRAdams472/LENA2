-- Add a conversion factor to each unit so quantities can be converted
-- between measurement systems. quantity * to_base_factor yields the amount
-- in the kind's canonical base unit (milliliters for 'volume', grams for
-- 'weight'). Count units are not convertible and stay NULL.
ALTER TABLE inventory.unit
    ADD COLUMN to_base_factor NUMERIC(12,6);

UPDATE inventory.unit SET to_base_factor = 4.92892  WHERE name = 'teaspoon';
UPDATE inventory.unit SET to_base_factor = 14.7868  WHERE name = 'tablespoon';
UPDATE inventory.unit SET to_base_factor = 29.5735  WHERE name = 'fluid ounce';
UPDATE inventory.unit SET to_base_factor = 236.588  WHERE name = 'cup';
UPDATE inventory.unit SET to_base_factor = 473.176  WHERE name = 'pint';
UPDATE inventory.unit SET to_base_factor = 946.353  WHERE name = 'quart';
UPDATE inventory.unit SET to_base_factor = 3785.41  WHERE name = 'gallon';
UPDATE inventory.unit SET to_base_factor = 1        WHERE name = 'milliliter';
UPDATE inventory.unit SET to_base_factor = 1000     WHERE name = 'liter';

UPDATE inventory.unit SET to_base_factor = 1        WHERE name = 'gram';
UPDATE inventory.unit SET to_base_factor = 1000     WHERE name = 'kilogram';
UPDATE inventory.unit SET to_base_factor = 28.3495  WHERE name = 'ounce';
UPDATE inventory.unit SET to_base_factor = 453.592  WHERE name = 'pound';
