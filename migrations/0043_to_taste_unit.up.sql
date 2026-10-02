-- 0043_to_taste_unit.up.sql
-- 'to taste' is a real unit so unquantified seasonings ("salt to taste",
-- "pepper as needed") resolve through the review flow instead of forcing a
-- meaningless count. kind='count' like the other discrete units.

INSERT INTO inventory.unit (name, abbreviation, kind, created_by) VALUES
    ('to taste',   NULL,   'count', 'migration'),
    ('as needed',  NULL,   'count', 'migration')
ON CONFLICT (name) DO NOTHING;
