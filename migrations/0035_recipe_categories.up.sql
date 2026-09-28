-- 0035_recipe_categories.up.sql — faceted recipe taxonomy.
--
-- Categories are grouped into types (Course, Cuisine, ...). A group flagged
-- exclusive allows a recipe to hold at most one of its categories
-- (enforced in the service layer — Postgres can't express it declaratively
-- without a trigger). Assignments live in recipe_category and cascade with
-- either side.

CREATE TABLE recipe.category_group (
    category_group_id BIGSERIAL    PRIMARY KEY,
    name              VARCHAR(50)  NOT NULL UNIQUE,
    exclusive         BOOLEAN      NOT NULL DEFAULT TRUE,
    display_order     INTEGER      NOT NULL DEFAULT 0,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_by        VARCHAR(100),
    updated_at        TIMESTAMPTZ
);

CREATE TABLE recipe.category (
    category_id       BIGSERIAL    PRIMARY KEY,
    category_group_id BIGINT       NOT NULL REFERENCES recipe.category_group(category_group_id) ON DELETE RESTRICT,
    name              VARCHAR(50)  NOT NULL,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_by        VARCHAR(100),
    updated_at        TIMESTAMPTZ,
    UNIQUE (category_group_id, name)
);

CREATE TABLE recipe.recipe_category (
    recipe_id   BIGINT      NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    category_id BIGINT      NOT NULL REFERENCES recipe.category(category_id) ON DELETE CASCADE,
    assigned_by VARCHAR(100) NOT NULL,
    assigned_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (recipe_id, category_id)
);

CREATE INDEX idx_recipe_category_category ON recipe.recipe_category (category_id);

-- lena_app grants — mirrors 0031–0034.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA recipe TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA recipe TO lena_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA recipe
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO lena_app;

-- Seed taxonomy. Exclusivity is per group: Dish Type and Dietary allow
-- multiple picks (a single-pan soup; low-calorie + gluten-free later).
INSERT INTO recipe.category_group (name, exclusive, display_order, created_by) VALUES
    ('Course',          TRUE,  1, 'seed'),
    ('Dish Type',       FALSE, 2, 'seed'),
    ('Dietary',         FALSE, 3, 'seed'),
    ('Difficulty',      TRUE,  4, 'seed'),
    ('Main Ingredient', TRUE,  5, 'seed'),
    ('Cuisine',         TRUE,  6, 'seed');

INSERT INTO recipe.category (category_group_id, name, created_by)
SELECT g.category_group_id, v.name, 'seed'
FROM recipe.category_group g
JOIN (VALUES
    ('Course',          'Breakfast'),
    ('Course',          'Lunch'),
    ('Course',          'Dinner'),
    ('Course',          'Dessert'),
    ('Course',          'Snack'),
    ('Course',          'Side'),
    ('Dish Type',       'Cocktail'),
    ('Dish Type',       'Soup'),
    ('Dish Type',       'Bread'),
    ('Dish Type',       'Casserole'),
    ('Dish Type',       'Single Pan'),
    ('Dietary',         'Low Calorie'),
    ('Difficulty',      'Easy'),
    ('Difficulty',      'Medium'),
    ('Difficulty',      'Skilled'),
    ('Main Ingredient', 'Chicken'),
    ('Main Ingredient', 'Beef'),
    ('Main Ingredient', 'Fish'),
    ('Main Ingredient', 'Pork'),
    ('Main Ingredient', 'Vegetarian'),
    ('Cuisine',         'Italian'),
    ('Cuisine',         'Spanish'),
    ('Cuisine',         'Mexican'),
    ('Cuisine',         'Southwestern US'),
    ('Cuisine',         'French'),
    ('Cuisine',         'American'),
    ('Cuisine',         'Asian')
) AS v(group_name, name) ON v.group_name = g.name;
