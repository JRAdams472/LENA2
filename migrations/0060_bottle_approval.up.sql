-- Member-submitted catalog bottles require admin approval, mirroring the
-- inventory.item flow (0020). status defaults to 'approved' so all existing
-- catalog rows stay visible; new member submissions set status='pending' and
-- submitted_by_user_id so they remain visible to (and editable by) their
-- creator until an admin approves. 'rejected' rows are hidden from everyone;
-- approved_by/approved_at record the moderation decision.

ALTER TABLE wine.bottle
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'approved'
        CONSTRAINT bottle_status CHECK (status IN ('pending', 'approved', 'rejected')),
    ADD COLUMN submitted_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    ADD COLUMN approved_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    ADD COLUMN approved_at TIMESTAMPTZ;

CREATE INDEX idx_bottle_status ON wine.bottle (status);
