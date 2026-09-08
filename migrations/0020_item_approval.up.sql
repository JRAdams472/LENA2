-- Mobile redesign p0: user-submitted catalog items require admin approval.
-- status defaults to 'approved' so all existing catalog rows stay visible;
-- new user submissions set status='pending' and submitted_by_user_id so they
-- remain visible to (and editable by) their creator until an admin approves.
-- 'rejected' rows are hidden from everyone; approved_by/approved_at record
-- the moderation decision.

ALTER TABLE inventory.item
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'approved'
        CONSTRAINT item_status CHECK (status IN ('pending', 'approved', 'rejected')),
    ADD COLUMN submitted_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    ADD COLUMN approved_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    ADD COLUMN approved_at TIMESTAMPTZ;

CREATE INDEX idx_item_status ON inventory.item (status);
