-- 0055_access_request_notification.up.sql — access-request notification
-- kind (LEN-127).
--
-- Registers 'access_requested' in the notification_type registry so the
-- notification gate and feed accept it. Category 'admin' groups it with
-- site-administration events in notification preferences; it defaults to
-- enabled like every other kind.

INSERT INTO household.notification_type (kind, category, label, description) VALUES
    ('access_requested', 'admin', 'Access request', 'A new member requested access and is awaiting admin approval');
