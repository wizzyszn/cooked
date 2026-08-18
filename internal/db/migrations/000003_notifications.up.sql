CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel VARCHAR(16) NOT NULL DEFAULT 'email' CHECK (channel IN ('in_app', 'email')),
    template VARCHAR(64) NOT NULL,
    payload_json JSONB,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('failed', 'pending', 'sent', 'suppressed')),
    external_ref VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications (user_id);

CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications (status)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_deleted_at ON notifications (deleted_at);
