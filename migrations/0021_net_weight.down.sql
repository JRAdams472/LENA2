ALTER TABLE inventory.item
    DROP COLUMN IF EXISTS net_weight,
    DROP COLUMN IF EXISTS is_metric;
