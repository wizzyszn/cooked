CREATE TABLE recipe_version_review_aggregates (
    recipe_version_id UUID PRIMARY KEY REFERENCES recipe_versions(id) ON DELETE CASCADE,
    review_count INT NOT NULL DEFAULT 0 CHECK (review_count >= 0),
    average_taste NUMERIC(4,2) NOT NULL DEFAULT 0,
    average_clarity NUMERIC(4,2) NOT NULL DEFAULT 0,
    average_difficulty_accuracy NUMERIC(4,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE RESTRICT,
    recipe_version_id UUID NOT NULL REFERENCES recipe_versions(id) ON DELETE RESTRICT,
    taste SMALLINT NOT NULL CHECK (taste BETWEEN 1 AND 5),
    clarity SMALLINT NOT NULL CHECK (clarity BETWEEN 1 AND 5),
    difficulty_accuracy SMALLINT NOT NULL CHECK (difficulty_accuracy BETWEEN 1 AND 5),
    comment TEXT NOT NULL DEFAULT '',
    photo_media_id UUID REFERENCES media_assets(id) ON DELETE RESTRICT,
    moderation_status VARCHAR(16) NOT NULL DEFAULT 'visible' CHECK (moderation_status IN ('visible','hidden','removed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, recipe_version_id)
);
CREATE INDEX idx_reviews_version_visible ON reviews(recipe_version_id, created_at DESC, id DESC) WHERE moderation_status='visible';

CREATE TABLE review_create_commands (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(128) NOT NULL,
    review_id UUID NOT NULL REFERENCES reviews(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id,idempotency_key)
);

CREATE TABLE content_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('recipe','review')),
    target_id UUID NOT NULL,
    reason VARCHAR(32) NOT NULL CHECK (reason IN ('spam','harassment','hate','dangerous','copyright','misinformation','other')),
    details TEXT NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','dismissed','upheld')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    UNIQUE(reporter_id,target_type,target_id)
);
CREATE INDEX idx_content_reports_queue ON content_reports(state,target_type,created_at) WHERE state='pending';

CREATE TABLE content_report_commands (
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(128) NOT NULL,
    report_id UUID NOT NULL REFERENCES content_reports(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(reporter_id,idempotency_key)
);

CREATE FUNCTION prevent_audit_log_mutation() RETURNS TRIGGER AS $$
BEGIN RAISE EXCEPTION 'audit logs are append-only'; END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_immutable_audit_logs BEFORE UPDATE OR DELETE ON audit_logs FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_mutation();
