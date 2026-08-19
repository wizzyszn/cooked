DROP INDEX IF EXISTS idx_refresh_tokens_family_id;

ALTER TABLE refresh_tokens
    DROP COLUMN IF EXISTS ip,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS replaced_by_id,
    DROP COLUMN IF EXISTS parent_id,
    DROP COLUMN IF EXISTS family_id;
