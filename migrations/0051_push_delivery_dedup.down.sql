DROP INDEX household.idx_push_delivery_dedup_key;
ALTER TABLE household.push_delivery DROP COLUMN dedup_key;
