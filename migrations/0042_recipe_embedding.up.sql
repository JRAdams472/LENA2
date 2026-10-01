-- 0042_recipe_embedding.up.sql
-- Semantic recipe search: pgvector-backed embeddings on the recipe catalog.
-- `embedding` holds the recipe's vector (name + description + ingredient
-- names + categories); `embedding_model` records which model produced it so
-- a model change marks every row stale for the backfill sweep; `embedding_at`
-- is the last successful refresh.
--
-- The dimension is fixed at 768 (nomic-embed-text). A different embedding
-- model must produce 768-dim vectors or writes fail loudly and the row stays
-- stale for the sweep to log.
--
-- The extension lives in `public` so all schemas can reference `vector`.

CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;

ALTER TABLE recipe.recipe
    ADD COLUMN embedding       vector(768),
    ADD COLUMN embedding_model VARCHAR(100),
    ADD COLUMN embedding_at    TIMESTAMPTZ;

-- Partial index: NULL embeddings are excluded, keeping the index small and
-- the sweep's `embedding IS NULL` check cheap.
CREATE INDEX recipe_embedding_hnsw
    ON recipe.recipe USING hnsw (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;
