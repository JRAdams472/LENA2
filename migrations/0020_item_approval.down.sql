DROP INDEX idx_item_status;

ALTER TABLE inventory.item
    DROP CONSTRAINT item_status,
    DROP COLUMN status,
    DROP COLUMN submitted_by_user_id,
    DROP COLUMN approved_by_user_id,
    DROP COLUMN approved_at;
