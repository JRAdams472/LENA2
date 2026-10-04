-- 0047_allergen.up.sql
-- Allergy tracking and risk detection, phase 1: a global allergen
-- taxonomy, canonical allergen links on generic ingredients, optional
-- product-level flags on branded items, and per-member allergy/dietary
-- records.
--
-- Resolution model (computed in the API, not here):
--   entity allergens = resolved ingredient's contains/may_contain rows
--                    UNION bound item's item_allergen rows
--   conflict         = entity allergens INTERSECT a member's user_allergen
-- Missing data degrades to "no allergen info" — never "known safe".

-- ---------- taxonomy ----------
CREATE TABLE inventory.allergen (
    allergen_id BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

-- Same normalized-unique guarantee as ingredient: whitespace-collapsed,
-- case-folded names collide at the DB level.
CREATE UNIQUE INDEX idx_allergen_name_norm
    ON inventory.allergen (lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))));

-- EU-14 / FDA-9 core set. Admin-managed from here on; names stay
-- lowercase to match the ingredient convention.
INSERT INTO inventory.allergen (name, description, created_by)
SELECT v.name, v.description, 'seed'
FROM (VALUES
    ('milk',                 'Milk and dairy products including lactose'),
    ('eggs',                 'Eggs and egg-derived ingredients'),
    ('fish',                 'Fish including anchovy, Worcestershire sauce, Caesar dressing'),
    ('crustacean shellfish', 'Shrimp, crab, lobster, crayfish and other crustaceans'),
    ('molluscs',             'Clams, mussels, oysters, scallops, squid and other molluscs'),
    ('tree nuts',            'Almonds, walnuts, cashews, pecans, hazelnuts, pistachios and other tree nuts'),
    ('peanuts',              'Peanuts and peanut-derived ingredients'),
    ('wheat',                'Wheat specifically, including wheat varieties and derivatives'),
    ('soy',                  'Soybeans and soy-derived ingredients'),
    ('sesame',               'Sesame seeds, sesame oil, tahini'),
    ('gluten',               'Cereals containing gluten: wheat, barley, rye, oats (non-certified), spelt'),
    ('celery',               'Celery, celeriac, celery salt and seed'),
    ('mustard',              'Mustard seed, prepared mustards, mustard powder'),
    ('sulfites',             'Sulphur dioxide and sulphites above labeling thresholds (dried fruit, wine, vinegar)'),
    ('lupin',                'Lupin flour and lupin-derived ingredients'),
    -- Dietary-restriction entries beyond the regulatory set — recorded
    -- with kind=dietary on member records, not medical allergies.
    ('corn',                 'Corn and corn-derived ingredients (cornmeal, cornstarch, masa, grits)'),
    ('gelatin',              'Gelatin and gelatin-derived ingredients (marshmallows, gummies, aspics)'),
    ('pork',                 'Pork and pork-derived ingredients — dietary restriction (kosher, halal, some vegetarians)'),
    ('beef',                 'Beef and beef-derived ingredients — dietary restriction')
) AS v(name, description);

-- ---------- canonical links ----------

-- Ingredient-level: the canonical allergen knowledge. kind distinguishes
-- an inherent component from a cross-contact risk.
CREATE TABLE inventory.ingredient_allergen (
    ingredient_id BIGINT NOT NULL REFERENCES inventory.ingredient(ingredient_id) ON DELETE CASCADE,
    allergen_id   BIGINT NOT NULL REFERENCES inventory.allergen(allergen_id) ON DELETE CASCADE,
    kind          VARCHAR(20) NOT NULL CHECK (kind IN ('contains', 'may_contain')),
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    PRIMARY KEY (ingredient_id, allergen_id)
);
CREATE INDEX idx_ingredient_allergen_allergen ON inventory.ingredient_allergen (allergen_id);

-- Item-level: product-specific additive flags — shared-facility
-- "may contain" or a formulation outlier on an otherwise clean
-- ingredient. Resolves as a union with the ingredient's rows, never a
-- replacement.
CREATE TABLE inventory.item_allergen (
    item_id     BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    allergen_id BIGINT NOT NULL REFERENCES inventory.allergen(allergen_id) ON DELETE CASCADE,
    kind        VARCHAR(20) NOT NULL CHECK (kind IN ('contains', 'may_contain')),
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (item_id, allergen_id)
);
CREATE INDEX idx_item_allergen_allergen ON inventory.item_allergen (allergen_id);

-- ---------- member records ----------

-- Per-member records: allergy is a hard warning, dietary a softer
-- advisory. Rows are personal — each member edits their own; warnings
-- name the member so the rest of the household sees the conflict label.
CREATE TABLE userprefs.user_allergen (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    allergen_id BIGINT NOT NULL REFERENCES inventory.allergen(allergen_id) ON DELETE CASCADE,
    kind        VARCHAR(20) NOT NULL CHECK (kind IN ('allergy', 'dietary')),
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, allergen_id)
);
CREATE INDEX idx_user_allergen_allergen ON userprefs.user_allergen (allergen_id);

-- ---------- grants ----------
-- Schema-level default privileges already cover new tables; granted
-- explicitly to match 0036's convention.
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.allergen TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.ingredient_allergen TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.item_allergen TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON userprefs.user_allergen TO lena_app;
GRANT USAGE ON SEQUENCE inventory.allergen_allergen_id_seq TO lena_app;
