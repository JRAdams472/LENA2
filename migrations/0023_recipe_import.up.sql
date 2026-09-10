CREATE TABLE recipe.recipe_import (
    recipe_import_id     BIGSERIAL PRIMARY KEY,
    submitted_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    source_filename      VARCHAR(500) NOT NULL,
    source_path          TEXT NOT NULL,
    source_hash          VARCHAR(64),
    ocr_text             TEXT,
    ocr_json             JSONB,
    draft_json           JSONB,
    review_json          JSONB,
    profanity_flag       BOOLEAN NOT NULL DEFAULT FALSE,
    profanity_reason     TEXT,
    status               VARCHAR(20) NOT NULL DEFAULT 'pending'
        CONSTRAINT recipe_import_status CHECK (
            status IN (
                'pending','processing','ocred','drafted','reviewing',
                'ready','persisted','rejected','failed','profanity'
            )
        ),
    recipe_id            BIGINT REFERENCES recipe.recipe(recipe_id) ON DELETE SET NULL,
    error_message        TEXT,
    created_by           VARCHAR(100) NOT NULL,
    updated_by           VARCHAR(100),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ,
    approved_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    approved_at          TIMESTAMPTZ
);

CREATE INDEX idx_recipe_import_status         ON recipe.recipe_import (status);
CREATE INDEX idx_recipe_import_submitted_by  ON recipe.recipe_import (submitted_by_user_id);
CREATE INDEX idx_recipe_import_recipe_id     ON recipe.recipe_import (recipe_id);
