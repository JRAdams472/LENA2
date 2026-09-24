-- 0029_household_switchover.down.sql — BEST-EFFORT rollback.
--
-- The up migration aggregates per-user rows into household rows; mapping
-- household rows back to individual users is lossy. Household data is
-- reassigned to each member's user_id (the last member processed wins for
-- shared rows) and is_favorite is restored from the favorite tables.
-- household.invites and household.households are preserved by 0028's down
-- migration, not this one.

-- ---------- grocery lists ----------

ALTER TABLE grocery.grocery_list
    ADD COLUMN user_id BIGINT REFERENCES identity.users(user_id);

UPDATE grocery.grocery_list gl
SET user_id = u.user_id
FROM identity.users u
WHERE u.household_id = gl.household_id;

ALTER TABLE grocery.grocery_list
    ALTER COLUMN user_id SET NOT NULL,
    DROP COLUMN household_id;

-- ---------- meal plans ----------

ALTER TABLE mealplan.meal_plan
    ADD COLUMN user_id BIGINT REFERENCES identity.users(user_id);

UPDATE mealplan.meal_plan mp
SET user_id = u.user_id
FROM identity.users u
WHERE u.household_id = mp.household_id;

-- Dropping household_id also drops idx_meal_plan_household_week; recreate
-- the original user-scoped index.
ALTER TABLE mealplan.meal_plan
    ALTER COLUMN user_id SET NOT NULL,
    DROP COLUMN household_id;

CREATE INDEX idx_meal_plan_user_week ON mealplan.meal_plan (user_id, week_start_date);

-- ---------- cellar ----------

ALTER TABLE userprefs.household_bottle RENAME CONSTRAINT household_bottle_pkey TO user_bottle_pkey;
ALTER SEQUENCE userprefs.household_bottle_household_bottle_id_seq RENAME TO user_bottle_user_bottle_id_seq;
ALTER TABLE userprefs.household_bottle RENAME TO user_bottle;
ALTER TABLE userprefs.user_bottle RENAME COLUMN household_bottle_id TO user_bottle_id;

ALTER TABLE userprefs.user_bottle
    ADD COLUMN user_id     BIGINT REFERENCES identity.users(user_id),
    ADD COLUMN is_favorite BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE userprefs.user_bottle ub
SET user_id = u.user_id
FROM identity.users u
WHERE u.household_id = ub.household_id;

UPDATE userprefs.user_bottle ub
SET is_favorite = f.is_favorite
FROM userprefs.user_bottle_favorite f
WHERE f.user_id = ub.user_id AND f.bottle_id = ub.bottle_id;

ALTER TABLE userprefs.user_bottle
    ALTER COLUMN user_id SET NOT NULL,
    DROP CONSTRAINT household_bottle_household_bottle,
    ADD CONSTRAINT user_bottle_user_bottle_key UNIQUE (user_id, bottle_id),
    DROP COLUMN household_id;

DROP TABLE userprefs.user_bottle_favorite;

-- ---------- pantry ----------

ALTER INDEX userprefs.idx_household_item_item_id RENAME TO idx_user_item_item_id;
ALTER TABLE userprefs.household_item RENAME CONSTRAINT household_item_pkey TO user_item_pkey;
ALTER SEQUENCE userprefs.household_item_household_item_id_seq RENAME TO user_item_user_item_id_seq;
ALTER TABLE userprefs.household_item RENAME TO user_item;
ALTER TABLE userprefs.user_item RENAME COLUMN household_item_id TO user_item_id;

ALTER TABLE userprefs.user_item
    ADD COLUMN user_id     BIGINT REFERENCES identity.users(user_id),
    ADD COLUMN is_favorite BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE userprefs.user_item ui
SET user_id = u.user_id
FROM identity.users u
WHERE u.household_id = ui.household_id;

UPDATE userprefs.user_item ui
SET is_favorite = f.is_favorite
FROM userprefs.user_item_favorite f
WHERE f.user_id = ui.user_id AND f.item_id = ui.item_id;

ALTER TABLE userprefs.user_item
    ALTER COLUMN user_id SET NOT NULL,
    DROP CONSTRAINT household_item_household_item,
    ADD UNIQUE (user_id, item_id),
    DROP COLUMN household_id;

DROP TABLE userprefs.user_item_favorite;
