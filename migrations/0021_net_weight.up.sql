-- Add net weight and a metric/imperial flag to the item catalog.
-- UPC uniqueness is enforced if it was not already.

ALTER TABLE inventory.item
    ADD COLUMN net_weight NUMERIC(10,4),
    ADD COLUMN is_metric  BOOLEAN NOT NULL DEFAULT false;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'inventory.item'::regclass
          AND conname = 'item_upc12_key'
    ) THEN
        ALTER TABLE inventory.item ADD CONSTRAINT item_upc12_key UNIQUE (upc12);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'inventory.item'::regclass
          AND conname = 'item_upc14_key'
    ) THEN
        ALTER TABLE inventory.item ADD CONSTRAINT item_upc14_key UNIQUE (upc14);
    END IF;
END $$;
