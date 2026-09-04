CREATE TABLE inventory.category (
    category_id   BIGSERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL UNIQUE,
    description   VARCHAR(500),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ
);

CREATE TABLE inventory.brand (
    brand_id   BIGSERIAL PRIMARY KEY,
    name       VARCHAR(200) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory.item (
    item_id     BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    brand_id    BIGINT REFERENCES inventory.brand(brand_id),
    upc12       VARCHAR(12),
    upc14       VARCHAR(14),
    category_id BIGINT NOT NULL REFERENCES inventory.category(category_id),
    unit        VARCHAR(20) NOT NULL,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    UNIQUE (name, brand_id),
    UNIQUE (upc12),
    UNIQUE (upc14)
);

CREATE TABLE inventory.flavor_profile (
    flavor_id   BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE inventory.food_flavor (
    food_id       BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    flavor_id     BIGINT NOT NULL REFERENCES inventory.flavor_profile(flavor_id),
    intensity     SMALLINT NOT NULL CHECK (intensity BETWEEN 1 AND 5),
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (food_id, flavor_id)
);

CREATE TABLE inventory.nutrient_type (
    nutrient_id   BIGSERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL UNIQUE,
    unit          VARCHAR(50),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory.food_nutrient (
    food_id      BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    nutrient_id  BIGINT NOT NULL REFERENCES inventory.nutrient_type(nutrient_id),
    amount       NUMERIC(10,4) NOT NULL,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (food_id, nutrient_id)
);
