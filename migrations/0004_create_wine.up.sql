CREATE TABLE wine.country (
    country_id  BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    iso_code    VARCHAR(3) NOT NULL UNIQUE,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.region (
    region_id   BIGSERIAL PRIMARY KEY,
    country_id  BIGINT NOT NULL REFERENCES wine.country(country_id),
    name        VARCHAR(200) NOT NULL,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.type (
    type_id     BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL UNIQUE,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.vintage (
    vintage_id  BIGSERIAL PRIMARY KEY,
    year        INTEGER NOT NULL UNIQUE,
    description VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE wine.grape_variety (
    grape_variety_id BIGSERIAL PRIMARY KEY,
    name             VARCHAR(200) NOT NULL UNIQUE,
    description      VARCHAR(500),
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_by       VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       VARCHAR(100),
    updated_at       TIMESTAMPTZ
);

CREATE TABLE wine.flavor_profile (
    flavor_profile_id BIGSERIAL PRIMARY KEY,
    name              VARCHAR(200) NOT NULL UNIQUE,
    description       VARCHAR(500),
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        VARCHAR(100),
    updated_at        TIMESTAMPTZ
);

CREATE TABLE wine.bottle (
    bottle_id       BIGSERIAL PRIMARY KEY,
    type_id         BIGINT NOT NULL REFERENCES wine.type(type_id),
    country_id      BIGINT NOT NULL REFERENCES wine.country(country_id),
    region_id       BIGINT NOT NULL REFERENCES wine.region(region_id),
    vintage_year    INTEGER NOT NULL,
    vineyard        VARCHAR(200),
    abv             NUMERIC(5,2),
    acidity         SMALLINT,
    tannin_level    SMALLINT,
    body            SMALLINT,
    sweetness       SMALLINT,
    oak_integration BOOLEAN,
    bottle_size     VARCHAR(20) NOT NULL DEFAULT '750ml',
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

CREATE TABLE wine.bottle_flavor_profile (
    bottle_id         BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    flavor_profile_id BIGINT NOT NULL REFERENCES wine.flavor_profile(flavor_profile_id),
    intensity         SMALLINT NOT NULL CHECK (intensity BETWEEN 1 AND 5),
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (bottle_id, flavor_profile_id)
);

CREATE TABLE wine.bottle_grape_variety (
    bottle_id         BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    grape_variety_id  BIGINT NOT NULL REFERENCES wine.grape_variety(grape_variety_id),
    percentage        SMALLINT,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (bottle_id, grape_variety_id)
);
