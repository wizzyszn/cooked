DROP INDEX IF EXISTS idx_password_reset_tokens_user_active_code;

CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens (code);
