-- 0055_access_request_notification.down.sql — remove the access-request
-- notification kind. Rows already delivered would violate the FK once the
-- registry row is gone, so delete them first.

DELETE FROM household.notifications
WHERE kind = 'access_requested';

DELETE FROM household.notification_type
WHERE kind = 'access_requested';
