-- 0041_store_routing.down.sql
ALTER TABLE grocery.grocery_list DROP COLUMN IF EXISTS store_id;
DROP TABLE IF EXISTS grocery.item_route;
DROP TABLE IF EXISTS grocery.aisle_assignment;
DROP TABLE IF EXISTS grocery.store_aisle;
DROP TABLE IF EXISTS grocery.store;
