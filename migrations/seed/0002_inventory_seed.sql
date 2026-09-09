-- Inventory item seed: loads the Grocery UPC CSV into the catalog.
-- Uses psql \copy, so this file must be run with psql (Docker db-seed does).

CREATE TEMP TABLE _upc_import (
    grp_id text,
    upc14  text,
    upc12  text,
    brand  text,
    name   text
);

\copy _upc_import (grp_id, upc14, upc12, brand, name) FROM '/seed/Grocery_UPC_Database.csv' WITH (FORMAT csv, HEADER);

INSERT INTO inventory.brand (name)
SELECT DISTINCT NULLIF(trim(brand), '')
FROM _upc_import
WHERE NULLIF(trim(brand), '') IS NOT NULL
ON CONFLICT (name) DO NOTHING;

INSERT INTO inventory.item (
    name,
    brand_id,
    upc12,
    upc14,
    category_id,
    unit_id,
    status,
    created_by
)
SELECT
    LEFT(i.name, 200),
    COALESCE(
        (SELECT b.brand_id FROM inventory.brand b WHERE b.name = NULLIF(trim(i.brand), '')),
        (SELECT b.brand_id FROM inventory.brand b WHERE b.name = 'Generic')
    ),
    NULLIF(trim(i.upc12), ''),
    NULLIF(trim(i.upc14), ''),
    (
        SELECT c.category_id
        FROM inventory.category c
        WHERE c.name =
            CASE
                WHEN i.brand ILIKE '%produce%' THEN 'Produce'
                WHEN i.brand ILIKE '%dairy%' OR i.brand ILIKE '%milk%' OR i.name ILIKE '%milk%' OR i.name ILIKE '%cheese%' THEN 'Dairy'
                WHEN i.brand ILIKE '%meat%' OR i.brand ILIKE '%beef%' OR i.brand ILIKE '%chicken%' THEN 'Meat'
                WHEN i.brand ILIKE '%bakery%' OR i.brand ILIKE '%bread%' OR i.brand ILIKE '%bagel%' THEN 'Bakery'
                ELSE 'Pantry'
            END
    ),
    (SELECT u.unit_id FROM inventory.unit u WHERE u.name = 'each'),
    'approved',
    'seed'
FROM _upc_import i
ON CONFLICT DO NOTHING;

DROP TABLE _upc_import;
