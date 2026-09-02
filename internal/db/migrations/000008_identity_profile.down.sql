DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS oauth_login_codes;
DROP TABLE IF EXISTS oauth_authorization_flows;
DROP TABLE IF EXISTS oauth_identities;
DROP TABLE IF EXISTS user_dietary_preferences;
DROP TABLE IF EXISTS dietary_tags;
ALTER TABLE users
    DROP COLUMN IF EXISTS longest_streak,
    DROP COLUMN IF EXISTS current_streak,
    DROP COLUMN IF EXISTS xp_total,
    DROP COLUMN IF EXISTS deactivated_at,
    DROP COLUMN IF EXISTS anonymized_at,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS avatar_media_id,
    DROP COLUMN IF EXISTS bio;
