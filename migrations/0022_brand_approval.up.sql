-- Brand approval workflow: any authenticated user may submit a new brand,
-- which starts pending and is visible only to its submitter until an
-- admin approves it. Mirrors 0020_item_approval.up.sql exactly.

ALTER TABLE inventory.brand
    ADD COLUMN created_by VARCHAR(100) NOT NULL DEFAULT 'seed',
    ADD COLUMN updated_by VARCHAR(100),
    ADD COLUMN updated_at TIMESTAMPTZ,
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'approved'
        CONSTRAINT brand_status CHECK (status IN ('pending', 'approved', 'rejected')),
    ADD COLUMN submitted_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    ADD COLUMN approved_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    ADD COLUMN approved_at TIMESTAMPTZ;

ALTER TABLE inventory.brand ALTER COLUMN created_by DROP DEFAULT;

CREATE INDEX idx_brand_status ON inventory.brand (status);
