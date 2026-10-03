-- 0045_item_ingredient_link.up.sql
-- Generic-ingredient layer, phase 1: branded catalog items gain a
-- catalog-level link to the unbranded ingredient abstraction; households
-- get an override to remap items for themselves, and a "usual brand"
-- record so ingredient-keyed grocery lines can credit pantry stock on
-- check-off. Resolution order everywhere is household override ->
-- item.ingredient_id.
--
-- recipe_item.item_id and event_recipe_item.item_id drop NOT NULL so
-- ingredient-first rows can land; a CHECK keeps every row anchored to at
-- least one reference. ingredient_id becomes required on recipe_item in
-- a later migration once the backfill is applied and verified.

ALTER TABLE inventory.item
    ADD COLUMN ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id);

CREATE INDEX idx_item_ingredient ON inventory.item (ingredient_id);

-- Household-level remap: "this product is a different ingredient for us."
CREATE TABLE userprefs.household_item_ingredient (
    household_id  BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    item_id       BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    ingredient_id BIGINT NOT NULL REFERENCES inventory.ingredient(ingredient_id) ON DELETE CASCADE,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    PRIMARY KEY (household_id, item_id)
);

-- "Usual brand": the item a household last picked for a given
-- ingredient, written by brand-picked check-offs so repeat purchases can
-- credit stock without prompting.
CREATE TABLE userprefs.household_ingredient_item (
    household_id  BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    ingredient_id BIGINT NOT NULL REFERENCES inventory.ingredient(ingredient_id) ON DELETE CASCADE,
    item_id       BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    last_used_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    PRIMARY KEY (household_id, ingredient_id)
);

ALTER TABLE recipe.recipe_item ALTER COLUMN item_id DROP NOT NULL;
ALTER TABLE recipe.recipe_item
    ADD CONSTRAINT recipe_item_one_ref
    CHECK (item_id IS NOT NULL OR ingredient_id IS NOT NULL);

ALTER TABLE event.event_recipe_item ALTER COLUMN item_id DROP NOT NULL;
ALTER TABLE event.event_recipe_item
    ADD CONSTRAINT event_recipe_item_one_ref
    CHECK (item_id IS NOT NULL OR ingredient_id IS NOT NULL);

-- meal_slot_item.item_id is already nullable; add the same anchor guard.
ALTER TABLE mealplan.meal_slot_item
    ADD CONSTRAINT meal_slot_item_one_ref
    CHECK (item_id IS NOT NULL OR ingredient_id IS NOT NULL);
