-- Short OTPs must not be globally unique; only one unused (user_id, code) at a time.
DROP INDEX IF EXISTS idx_password_reset_tokens_token_hash;
DROP INDEX IF EXISTS idx_password_reset_tokens_user_code;

CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_tokens_user_active_code
    ON password_reset_tokens (user_id, code)
    WHERE used_at IS NULL AND deleted_at IS NULL;
