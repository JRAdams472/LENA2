-- 0043_to_taste_unit.down.sql
-- Removes the discretionary units added by 0043. Fails if any recipe item
-- still references them — delete those references first.

DELETE FROM inventory.unit WHERE name IN ('to taste', 'as needed');
