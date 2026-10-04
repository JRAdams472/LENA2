-- 0047_allergen.down.sql
-- Reverses 0047. Member records and link rows are dropped with the
-- tables; the seeded allergen registry goes with inventory.allergen.

DROP TABLE userprefs.user_allergen;
DROP TABLE inventory.item_allergen;
DROP TABLE inventory.ingredient_allergen;
DROP TABLE inventory.allergen;
