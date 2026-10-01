-- 0041_store_routing.up.sql
-- Grocery store routing: household-defined stores with ordered aisles,
-- item->aisle assignments, and the learned/manual walk-order record that
-- survives grocery_list_item regeneration (generated rows are deleted
-- wholesale, so route state must live off-list).
--
-- Future sharing affordance: store.external_ref is a canonical
-- chain+location key; aisle_assignment and item_route are keyed per store
-- so data can later be merged across households that shop the same store.

CREATE TABLE grocery.store (
    store_id     BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    name         VARCHAR(200) NOT NULL,
    external_ref VARCHAR(200),
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ,
    UNIQUE (household_id, name)
);

CREATE TABLE grocery.store_aisle (
    aisle_id   BIGSERIAL PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES grocery.store(store_id) ON DELETE CASCADE,
    name       VARCHAR(200) NOT NULL,
    position   INTEGER NOT NULL,
    created_by VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by VARCHAR(100),
    updated_at TIMESTAMPTZ,
    UNIQUE (store_id, name)
);

CREATE INDEX idx_store_aisle_store ON grocery.store_aisle (store_id, position);

-- item_id / ingredient_id / manual_name: exactly one non-null. Manual
-- names are stored normalized (lower + trim) so the same phrase routes
-- identically across lists.
CREATE TABLE grocery.aisle_assignment (
    aisle_assignment_id BIGSERIAL PRIMARY KEY,
    store_id            BIGINT NOT NULL REFERENCES grocery.store(store_id) ON DELETE CASCADE,
    aisle_id            BIGINT NOT NULL REFERENCES grocery.store_aisle(aisle_id) ON DELETE CASCADE,
    item_id             BIGINT REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    ingredient_id       BIGINT REFERENCES inventory.ingredient(ingredient_id) ON DELETE CASCADE,
    manual_name         VARCHAR(200),
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ,
    CHECK (num_nonnulls(item_id, ingredient_id, manual_name) = 1)
);

CREATE UNIQUE INDEX idx_aisle_assignment_item
    ON grocery.aisle_assignment (store_id, item_id) WHERE item_id IS NOT NULL;
CREATE UNIQUE INDEX idx_aisle_assignment_ingredient
    ON grocery.aisle_assignment (store_id, ingredient_id) WHERE ingredient_id IS NOT NULL;
CREATE UNIQUE INDEX idx_aisle_assignment_manual
    ON grocery.aisle_assignment (store_id, manual_name) WHERE manual_name IS NOT NULL;

-- One route record per (household, store, item identity). store_id = 0 is
-- the generic household route (observed while the list had no store);
-- store-specific reads fall back to it. learned_sum/learned_count carry
-- the running mean of normalized check positions; manual_rank is the
-- explicit user arrangement and wins the sort while present.
CREATE TABLE grocery.item_route (
    item_route_id BIGSERIAL PRIMARY KEY,
    household_id  BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    store_id      BIGINT NOT NULL DEFAULT 0,
    item_id       BIGINT REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id) ON DELETE CASCADE,
    manual_name   VARCHAR(200),
    learned_sum   DOUBLE PRECISION NOT NULL DEFAULT 0,
    learned_count INTEGER NOT NULL DEFAULT 0,
    manual_rank   DOUBLE PRECISION,
    manual_at     TIMESTAMPTZ,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    CHECK (num_nonnulls(item_id, ingredient_id, manual_name) = 1)
);

CREATE UNIQUE INDEX idx_item_route_item
    ON grocery.item_route (household_id, store_id, item_id) WHERE item_id IS NOT NULL;
CREATE UNIQUE INDEX idx_item_route_ingredient
    ON grocery.item_route (household_id, store_id, ingredient_id) WHERE ingredient_id IS NOT NULL;
CREATE UNIQUE INDEX idx_item_route_manual
    ON grocery.item_route (household_id, store_id, manual_name) WHERE manual_name IS NOT NULL;
CREATE INDEX idx_item_route_lookup ON grocery.item_route (household_id, store_id);

-- Which store a list is shopped at; governs aisle grouping and which
-- route stats the check-offs on this list feed.
ALTER TABLE grocery.grocery_list
    ADD COLUMN store_id BIGINT REFERENCES grocery.store(store_id) ON DELETE SET NULL;
