-- 0051_push_delivery_dedup.up.sql — LEN-17 P2: dedup for the push outbox.
-- Sweep reminders dedup their feed rows via notifications.dedup_key;
-- deliveries need the same protection or a repeat sweep would push the
-- same reminder again. NULL keys never conflict, so event-driven rows
-- are unaffected — same rule as notifications.
ALTER TABLE household.push_delivery
    ADD COLUMN dedup_key VARCHAR(200);

CREATE UNIQUE INDEX idx_push_delivery_dedup_key
    ON household.push_delivery (dedup_key);
