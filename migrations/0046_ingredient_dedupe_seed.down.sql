-- 0046_ingredient_dedupe_seed.down.sql
-- Drops the normalized-name index and removes the seeded rows. Rows that
-- picked up references (recipe_item/item links, overrides, usuals) must
-- be cleaned up first — the FK constraints will block the delete and
-- surface exactly which seeds are in use.

DROP INDEX idx_ingredient_name_norm;

DELETE FROM inventory.ingredient WHERE created_by = 'seed';
