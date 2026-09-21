DROP INDEX IF EXISTS idx_brand_name_normalized;

ALTER TABLE inventory.brand
    DROP COLUMN IF EXISTS name_normalized,
    ADD CONSTRAINT brand_name_key UNIQUE (name);
