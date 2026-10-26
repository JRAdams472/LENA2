-- 0059_ingredient_catalog_repair.down.sql
-- Removes the repaired seed catalog. Rows that picked up references
-- (recipe_item/item links, overrides, usuals) are protected by the FK
-- constraints — the delete fails and names them rather than dangling.
-- Categories survive the down when the real seed profile or other rows
-- still reference them.

DELETE FROM inventory.ingredient WHERE created_by = 'seed';

DELETE FROM inventory.category c
WHERE c.created_by = 'seed'
  AND c.name IN ('Produce', 'Dairy', 'Meat', 'Bakery', 'Pantry')
  AND NOT EXISTS (
      SELECT 1 FROM inventory.ingredient i WHERE i.category_id = c.category_id
  )
  AND NOT EXISTS (
      SELECT 1 FROM inventory.item it WHERE it.category_id = c.category_id
  );
