-- 0033_event_recipe_items.up.sql — per-slot ingredient snapshots +
-- servings scaling.
--
-- Same isolation model as event_recipe_step (0032): a slot owns a copy of
-- the recipe's items, so scaling or editing quantities for one event never
-- alters the shared recipe. base_servings freezes the recipe's servings at
-- link time — it is the denominator of the scaling factor, so a later
-- recipe edit can't silently rescale a laid-out plan:
--     scaled_quantity = quantity * (slot.servings / base_servings)

ALTER TABLE event.event_recipe
    ADD COLUMN base_servings INTEGER CHECK (base_servings > 0);

CREATE TABLE event.event_recipe_item (
    event_recipe_item_id BIGSERIAL PRIMARY KEY,
    event_recipe_id      BIGINT NOT NULL REFERENCES event.event_recipe(event_recipe_id) ON DELETE CASCADE,
    item_id              BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    ingredient_id        BIGINT REFERENCES inventory.ingredient(ingredient_id) ON DELETE SET NULL,
    quantity             NUMERIC(10,4) NOT NULL,
    unit_id              BIGINT NOT NULL REFERENCES inventory.unit(unit_id),
    section_name         TEXT,
    display_order        INTEGER NOT NULL DEFAULT 0,
    notes                VARCHAR(500),
    is_optional          BOOLEAN NOT NULL DEFAULT FALSE,
    created_by           VARCHAR(100) NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           VARCHAR(100),
    updated_at           TIMESTAMPTZ
);

CREATE INDEX idx_event_recipe_item_recipe ON event.event_recipe_item (event_recipe_id);

-- Backfill: freeze the recipe's servings as the scale denominator and copy
-- its items for every existing linked slot.
UPDATE event.event_recipe er
SET base_servings = r.servings
FROM recipe.recipe r
WHERE er.recipe_id = r.recipe_id;

INSERT INTO event.event_recipe_item
    (event_recipe_id, item_id, ingredient_id, quantity, unit_id,
     section_name, display_order, notes, is_optional, created_by, updated_by)
SELECT er.event_recipe_id,
       ri.item_id,
       ri.ingredient_id,
       ri.quantity,
       ri.unit_id,
       ri.section_name,
       ri.display_order,
       ri.notes,
       ri.is_optional,
       er.created_by,
       er.created_by
FROM event.event_recipe er
JOIN recipe.recipe_item ri ON ri.recipe_id = er.recipe_id
WHERE er.recipe_id IS NOT NULL;

-- lena_app grants — mirrors the 0031/0032 block.
GRANT SELECT, INSERT, UPDATE, DELETE ON event.event_recipe_item TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA event TO lena_app;
