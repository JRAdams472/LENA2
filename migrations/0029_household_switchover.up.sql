-- 0029_household_switchover.up.sql — move shared operational data from
-- per-user to per-household keys. Lands with the code that reads/writes
-- household_id; user_id columns and is_favorite are dropped here.
--
-- Backfill relies on the 0028 guarantee that every existing user has a
-- household (household_id = user_id for pre-feature users). Rows whose
-- user has no household — only possible for users created between the two
-- migrations — fall back to a fresh single-person household.

-- Stragglers: users created between the 0028 backfill and this migration
-- have no household. Give each a fresh single-person household and link it.
DO $$
DECLARE
    u      RECORD;
    new_id BIGINT;
BEGIN
    FOR u IN SELECT user_id, email FROM identity.users WHERE household_id IS NULL LOOP
        INSERT INTO household.households (created_by)
        VALUES (left(u.email, 100))
        RETURNING household_id INTO new_id;
        UPDATE identity.users SET household_id = new_id WHERE user_id = u.user_id;
    END LOOP;
END $$;

-- ---------- pantry: userprefs.user_item -> userprefs.household_item ----------

ALTER TABLE userprefs.user_item
    ADD COLUMN household_id BIGINT REFERENCES household.households(household_id);

UPDATE userprefs.user_item ui
SET household_id = u.household_id
FROM identity.users u
WHERE ui.user_id = u.user_id;

-- Extract per-user favorites before the column is dropped.
CREATE TABLE userprefs.user_item_favorite (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    item_id     BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, item_id)
);

INSERT INTO userprefs.user_item_favorite (user_id, item_id, is_favorite, created_by, created_at)
SELECT user_id, item_id, is_favorite, created_by, created_at
FROM userprefs.user_item
WHERE is_favorite;

ALTER TABLE userprefs.user_item
    DROP COLUMN is_favorite,
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL,
    ADD CONSTRAINT household_item_household_item UNIQUE (household_id, item_id);

ALTER TABLE userprefs.user_item RENAME TO household_item;
ALTER TABLE userprefs.household_item RENAME COLUMN user_item_id TO household_item_id;
ALTER TABLE userprefs.household_item RENAME CONSTRAINT user_item_pkey TO household_item_pkey;
ALTER SEQUENCE userprefs.user_item_user_item_id_seq RENAME TO household_item_household_item_id_seq;
ALTER INDEX userprefs.idx_user_item_item_id RENAME TO idx_household_item_item_id;

-- ---------- cellar: userprefs.user_bottle -> userprefs.household_bottle ----------

ALTER TABLE userprefs.user_bottle
    ADD COLUMN household_id BIGINT REFERENCES household.households(household_id);

UPDATE userprefs.user_bottle ub
SET household_id = u.household_id
FROM identity.users u
WHERE ub.user_id = u.user_id;

CREATE TABLE userprefs.user_bottle_favorite (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    bottle_id   BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, bottle_id)
);

INSERT INTO userprefs.user_bottle_favorite (user_id, bottle_id, is_favorite, created_by, created_at)
SELECT user_id, bottle_id, is_favorite, created_by, created_at
FROM userprefs.user_bottle
WHERE is_favorite;

ALTER TABLE userprefs.user_bottle
    DROP COLUMN is_favorite,
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL,
    ADD CONSTRAINT household_bottle_household_bottle UNIQUE (household_id, bottle_id);

ALTER TABLE userprefs.user_bottle RENAME TO household_bottle;
ALTER TABLE userprefs.household_bottle RENAME COLUMN user_bottle_id TO household_bottle_id;
ALTER TABLE userprefs.household_bottle RENAME CONSTRAINT user_bottle_pkey TO household_bottle_pkey;
ALTER SEQUENCE userprefs.user_bottle_user_bottle_id_seq RENAME TO household_bottle_household_bottle_id_seq;

-- ---------- meal plans ----------

ALTER TABLE mealplan.meal_plan
    ADD COLUMN household_id BIGINT REFERENCES household.households(household_id);

UPDATE mealplan.meal_plan mp
SET household_id = u.household_id
FROM identity.users u
WHERE mp.user_id = u.user_id;

-- Dropping user_id also drops idx_meal_plan_user_week (it covers the
-- column); recreate the same index against household_id.
ALTER TABLE mealplan.meal_plan
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL;

CREATE INDEX idx_meal_plan_household_week ON mealplan.meal_plan (household_id, week_start_date);

-- ---------- grocery lists ----------

ALTER TABLE grocery.grocery_list
    ADD COLUMN household_id BIGINT REFERENCES household.households(household_id);

UPDATE grocery.grocery_list gl
SET household_id = u.household_id
FROM identity.users u
WHERE gl.user_id = u.user_id;

ALTER TABLE grocery.grocery_list
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL;
