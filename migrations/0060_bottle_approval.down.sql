DROP INDEX idx_bottle_status;

ALTER TABLE wine.bottle
    DROP CONSTRAINT bottle_status,
    DROP COLUMN status,
    DROP COLUMN submitted_by_user_id,
    DROP COLUMN approved_by_user_id,
    DROP COLUMN approved_at;
