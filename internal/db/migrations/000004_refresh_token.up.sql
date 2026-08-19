-- Rotation / reuse-detection columns for refresh tokens.
-- family_id is added nullable, backfilled, then locked to NOT NULL so this
-- succeeds even when refresh_tokens already has rows.

ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS family_id UUID,
    ADD COLUMN IF NOT EXISTS parent_id UUID,
    ADD COLUMN IF NOT EXISTS replaced_by_id UUID,
    ADD COLUMN IF NOT EXISTS user_agent VARCHAR(512),
    ADD COLUMN IF NOT EXISTS ip VARCHAR(64);

UPDATE refresh_tokens
SET family_id = gen_random_uuid()
WHERE family_id IS NULL;

ALTER TABLE refresh_tokens
    ALTER COLUMN family_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens (family_id);
