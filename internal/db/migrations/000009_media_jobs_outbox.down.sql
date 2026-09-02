DROP TABLE IF EXISTS notification_delivery_attempts;
DROP INDEX IF EXISTS idx_notifications_outbox;
ALTER TABLE notifications
    DROP COLUMN IF EXISTS sent_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS locked_by,
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS attempt_count;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_avatar_media_id_fkey;
DROP TABLE IF EXISTS media_variants;
DROP TABLE IF EXISTS media_assets;
