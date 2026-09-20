DO $$
DECLARE
    db_name text := current_database();
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lena_app') THEN
        CREATE ROLE lena_app WITH LOGIN;
    END IF;

    EXECUTE format('GRANT CONNECT ON DATABASE %I TO lena_app', db_name);
END
$$;

GRANT USAGE ON SCHEMA identity, inventory, wine, recipe, mealplan, grocery, analytics TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA identity, inventory, wine, recipe, mealplan, grocery, analytics TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA identity, inventory, wine, recipe, mealplan, grocery, analytics TO lena_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA identity, inventory, wine, recipe, mealplan, grocery, analytics
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO lena_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA identity, inventory, wine, recipe, mealplan, grocery, analytics
    GRANT USAGE ON SEQUENCES TO lena_app;
