ALTER TABLE users
    ADD COLUMN IF NOT EXISTS bio VARCHAR(1024),
    ADD COLUMN IF NOT EXISTS avatar_media_id UUID,
    ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    ADD COLUMN IF NOT EXISTS anonymized_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS xp_total BIGINT NOT NULL DEFAULT 0 CHECK (xp_total >= 0),
    ADD COLUMN IF NOT EXISTS current_streak INT NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
    ADD COLUMN IF NOT EXISTS longest_streak INT NOT NULL DEFAULT 0 CHECK (longest_streak >= 0);

CREATE TABLE dietary_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    slug VARCHAR(64) NOT NULL UNIQUE,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dietary_tags_slug_not_none CHECK (slug <> 'none')
);

INSERT INTO dietary_tags (name, slug) VALUES
    ('Vegetarian', 'vegetarian'), ('Vegan', 'vegan'),
    ('Halal', 'halal'), ('Pescatarian', 'pescatarian')
ON CONFLICT (slug) DO NOTHING;

CREATE TABLE user_dietary_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dietary_tag_id UUID NOT NULL REFERENCES dietary_tags(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, dietary_tag_id)
);
CREATE INDEX idx_user_dietary_preferences_tag ON user_dietary_preferences(dietary_tag_id, user_id);

CREATE TABLE oauth_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_subject),
    UNIQUE (user_id, provider)
);

CREATE TABLE oauth_authorization_flows (
    state_hash CHAR(64) PRIMARY KEY,
    code_verifier VARCHAR(128) NOT NULL,
    nonce_hash CHAR(64) NOT NULL,
    return_url VARCHAR(1024) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oauth_authorization_flows_expiry ON oauth_authorization_flows(expires_at);

CREATE TABLE oauth_login_codes (
    code_hash CHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oauth_login_codes_expiry ON oauth_login_codes(expires_at);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(128) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id UUID,
    reason TEXT,
    before_json JSONB,
    after_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_id, created_at DESC);
