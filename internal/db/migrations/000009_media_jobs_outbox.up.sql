CREATE TABLE media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    purpose VARCHAR(32) NOT NULL CHECK (purpose IN ('profile_avatar', 'dish_cover', 'recipe_cover', 'step_image', 'review_photo', 'cook_session_photo')),
    object_key VARCHAR(512) NOT NULL UNIQUE,
    original_filename VARCHAR(255),
    declared_mime_type VARCHAR(64) NOT NULL,
    decoded_mime_type VARCHAR(64),
    byte_size BIGINT CHECK (byte_size IS NULL OR byte_size >= 0),
    width INT CHECK (width IS NULL OR width > 0),
    height INT CHECK (height IS NULL OR height > 0),
    processing_status VARCHAR(24) NOT NULL DEFAULT 'awaiting_upload' CHECK (processing_status IN ('awaiting_upload', 'uploaded', 'processing', 'ready', 'retry', 'failed', 'deleted')),
    moderation_status VARCHAR(24) NOT NULL DEFAULT 'pending' CHECK (moderation_status IN ('pending', 'approved', 'rejected')),
    access_scope VARCHAR(16) NOT NULL CHECK (access_scope IN ('public', 'private')),
    checksum_sha256 CHAR(64),
    uploaded_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(128),
    last_error TEXT
);

CREATE INDEX idx_media_assets_owner ON media_assets(owner_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_assets_jobs ON media_assets(next_attempt_at, created_at)
WHERE deleted_at IS NULL AND processing_status IN ('uploaded', 'retry');
CREATE INDEX idx_media_assets_orphans ON media_assets(expires_at)
WHERE deleted_at IS NULL AND processing_status = 'awaiting_upload';

CREATE TABLE media_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id UUID NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    name VARCHAR(32) NOT NULL,
    object_key VARCHAR(512) NOT NULL UNIQUE,
    mime_type VARCHAR(64) NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    width INT NOT NULL CHECK (width > 0),
    height INT NOT NULL CHECK (height > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(media_asset_id, name)
);

ALTER TABLE users
    ADD CONSTRAINT users_avatar_media_id_fkey
    FOREIGN KEY (avatar_media_id) REFERENCES media_assets(id) ON DELETE SET NULL;

ALTER TABLE notifications
    ADD COLUMN attempt_count INT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN locked_at TIMESTAMPTZ,
    ADD COLUMN locked_by VARCHAR(128),
    ADD COLUMN last_error TEXT,
    ADD COLUMN sent_at TIMESTAMPTZ;

CREATE INDEX idx_notifications_outbox ON notifications(next_attempt_at, created_at)
WHERE deleted_at IS NULL AND status IN ('pending', 'failed');

CREATE TABLE notification_delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL CHECK (attempt_number > 0),
    provider_key VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(16) NOT NULL CHECK (status IN ('started', 'sent', 'failed', 'suppressed')),
    external_ref VARCHAR(255),
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE(notification_id, attempt_number)
);

CREATE INDEX idx_notification_attempts_notification ON notification_delivery_attempts(notification_id, started_at DESC);
