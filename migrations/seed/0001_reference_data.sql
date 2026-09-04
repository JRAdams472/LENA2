-- Reference data seed script.
-- Safe to run multiple times; all inserts are idempotent.

-- Inventory catalog
INSERT INTO inventory.category (name, description, created_by) VALUES
  ('Produce', 'Fresh fruits and vegetables', 'seed'),
  ('Dairy', 'Milk, cheese, and other dairy products', 'seed'),
  ('Meat', 'Beef, poultry, pork, and other meats', 'seed'),
  ('Bakery', 'Bread, pastries, and baked goods', 'seed'),
  ('Pantry', 'Dry goods and shelf-stable staples', 'seed')
ON CONFLICT (name) DO NOTHING;

INSERT INTO inventory.brand (name) VALUES
  ('Generic'),
  ('Organic Valley'),
  ('Kroger'),
  ('Trader Joe\'s')
ON CONFLICT (name) DO NOTHING;

INSERT INTO inventory.flavor_profile (name, is_active, created_by) VALUES
  ('Sweet', true, 'seed'),
  ('Salty', true, 'seed'),
  ('Sour', true, 'seed'),
  ('Bitter', true, 'seed'),
  ('Umami', true, 'seed'),
  ('Spicy', true, 'seed')
ON CONFLICT (name) DO NOTHING;

INSERT INTO inventory.nutrient_type (name, unit) VALUES
  ('Calories', 'kcal'),
  ('Protein', 'g'),
  ('Carbohydrates', 'g'),
  ('Fat', 'g'),
  ('Fiber', 'g'),
  ('Sugar', 'g'),
  ('Sodium', 'mg')
ON CONFLICT (name) DO NOTHING;

-- Wine catalog
INSERT INTO wine.type (name, description, is_active, created_by) VALUES
  ('Red', 'Red wines', true, 'seed'),
  ('White', 'White wines', true, 'seed'),
  ('Rosé', 'Rosé wines', true, 'seed'),
  ('Sparkling', 'Sparkling wines', true, 'seed'),
  ('Dessert', 'Dessert wines', true, 'seed')
ON CONFLICT (name) DO NOTHING;

INSERT INTO wine.country (name, iso_code, description, is_active, created_by) VALUES
  ('France', 'FRA', 'French wines', true, 'seed'),
  ('Italy', 'ITA', 'Italian wines', true, 'seed'),
  ('United States', 'USA', 'US wines', true, 'seed'),
  ('Spain', 'ESP', 'Spanish wines', true, 'seed'),
  ('Australia', 'AUS', 'Australian wines', true, 'seed')
ON CONFLICT (iso_code) DO NOTHING;

INSERT INTO wine.grape_variety (name, description, is_active, created_by) VALUES
  ('Cabernet Sauvignon', 'Popular red grape', true, 'seed'),
  ('Merlot', 'Soft red grape', true, 'seed'),
  ('Pinot Noir', 'Light red grape', true, 'seed'),
  ('Chardonnay', 'Popular white grape', true, 'seed'),
  ('Sauvignon Blanc', 'Crisp white grape', true, 'seed'),
  ('Syrah', 'Spicy red grape', true, 'seed'),
  ('Riesling', 'Aromatic white grape', true, 'seed')
ON CONFLICT (name) DO NOTHING;

INSERT INTO wine.vintage (year, description, is_active, created_by) VALUES
  (2019, '2019 vintage', true, 'seed'),
  (2020, '2020 vintage', true, 'seed'),
  (2021, '2021 vintage', true, 'seed'),
  (2022, '2022 vintage', true, 'seed'),
  (2023, '2023 vintage', true, 'seed')
ON CONFLICT (year) DO NOTHING;

INSERT INTO wine.flavor_profile (name, description, is_active, created_by) VALUES
  ('Fruity', 'Fruit-forward profile', true, 'seed'),
  ('Oaky', 'Oak-driven profile', true, 'seed'),
  ('Earthy', 'Earthy profile', true, 'seed'),
  ('Floral', 'Floral profile', true, 'seed'),
  ('Spicy', 'Spicy profile', true, 'seed')
ON CONFLICT (name) DO NOTHING;

-- Regions: avoid duplicates with no unique constraint by checking first.
INSERT INTO wine.region (country_id, name, description, is_active, created_by)
  SELECT c.country_id, 'Bordeaux', 'Bordeaux region', true, 'seed'
  FROM wine.country c
  WHERE c.name = 'France'
    AND NOT EXISTS (SELECT 1 FROM wine.region r WHERE r.country_id = c.country_id AND r.name = 'Bordeaux');

INSERT INTO wine.region (country_id, name, description, is_active, created_by)
  SELECT c.country_id, 'Tuscany', 'Tuscany region', true, 'seed'
  FROM wine.country c
  WHERE c.name = 'Italy'
    AND NOT EXISTS (SELECT 1 FROM wine.region r WHERE r.country_id = c.country_id AND r.name = 'Tuscany');

INSERT INTO wine.region (country_id, name, description, is_active, created_by)
  SELECT c.country_id, 'Napa Valley', 'Napa Valley region', true, 'seed'
  FROM wine.country c
  WHERE c.name = 'United States'
    AND NOT EXISTS (SELECT 1 FROM wine.region r WHERE r.country_id = c.country_id AND r.name = 'Napa Valley');
