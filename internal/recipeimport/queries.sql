-- name: CreateRecipeImport :one
INSERT INTO recipe.recipe_import (
    submitted_by_user_id,
    source_filename,
    source_path,
    source_hash,
    status,
    created_by,
    updated_by,
    created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING *;

-- name: GetRecipeImport :one
SELECT *
FROM recipe.recipe_import
WHERE recipe_import_id = $1;

-- name: ListRecipeImports :many
SELECT *
FROM recipe.recipe_import
WHERE ($1::varchar = '' OR status = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRecipeImports :one
SELECT COUNT(*)
FROM recipe.recipe_import
WHERE ($1::varchar = '' OR status = $1);

-- name: UpdateRecipeImportOCR :exec
UPDATE recipe.recipe_import
SET ocr_text    = $2,
    ocr_json    = $3,
    status      = 'ocred',
    updated_at  = now()
WHERE recipe_import_id = $1;

-- name: UpdateRecipeImportDraft :exec
UPDATE recipe.recipe_import
SET draft_json   = $2,
    status       = 'drafted',
    updated_at   = now()
WHERE recipe_import_id = $1;

-- name: UpdateRecipeImportReview :exec
UPDATE recipe.recipe_import
SET review_json = $2,
    status      = $3,
    updated_at  = now(),
    updated_by  = COALESCE($4, updated_by)
WHERE recipe_import_id = $1;

-- name: MarkRecipeImportProfanity :exec
UPDATE recipe.recipe_import
SET profanity_flag = TRUE,
    profanity_reason = $2,
    status           = 'profanity',
    error_message    = NULL,
    updated_at       = now()
WHERE recipe_import_id = $1;

-- name: MarkRecipeImportFailed :exec
UPDATE recipe.recipe_import
SET status        = 'failed',
    error_message = $2,
    updated_at    = now()
WHERE recipe_import_id = $1;

-- name: SetRecipeImportPersisted :exec
UPDATE recipe.recipe_import
SET status              = 'persisted',
    recipe_id             = $2,
    approved_by_user_id   = $3,
    approved_at           = now(),
    updated_at            = now()
WHERE recipe_import_id = $1;

-- name: SetRecipeImportRejected :exec
UPDATE recipe.recipe_import
SET status       = 'rejected',
    updated_at   = now(),
    error_message = NULL
WHERE recipe_import_id = $1;

-- name: SetRecipeImportPending :exec
UPDATE recipe.recipe_import
SET status         = 'pending',
    error_message  = NULL,
    profanity_flag = FALSE,
    profanity_reason = NULL,
    updated_at     = now()
WHERE recipe_import_id = $1;
