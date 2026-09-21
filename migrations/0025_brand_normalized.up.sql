-- Race-free brand dedup: normalize the brand name and enforce a
-- partial unique index that ignores rejected rows so a previously
-- rejected brand can be resubmitted.
ALTER TABLE inventory.brand
    ADD COLUMN name_normalized VARCHAR(200) GENERATED ALWAYS AS (lower(regexp_replace(name, '[^a-zA-Z0-9]', '', 'g'))) STORED,
    DROP CONSTRAINT IF EXISTS brand_name_key;

CREATE UNIQUE INDEX idx_brand_name_normalized ON inventory.brand (name_normalized) WHERE status <> 'rejected';
