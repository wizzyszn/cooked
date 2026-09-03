CREATE TABLE recipe_trend_scores (
    recipe_id UUID PRIMARY KEY REFERENCES recipes(id) ON DELETE CASCADE,
    unique_cooks INT NOT NULL DEFAULT 0,
    new_favorites INT NOT NULL DEFAULT 0,
    new_reviews INT NOT NULL DEFAULT 0,
    score INT NOT NULL DEFAULT 0,
    window_started_at TIMESTAMPTZ NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_recipe_trends_score ON recipe_trend_scores(score DESC,recipe_id DESC);

CREATE TABLE trend_projection_queue (
    recipe_id UUID PRIMARY KEY REFERENCES recipes(id) ON DELETE CASCADE,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE FUNCTION queue_recipe_trend_refresh() RETURNS TRIGGER AS $$
DECLARE rid UUID;
BEGIN
    rid := CASE WHEN TG_OP='DELETE' THEN OLD.recipe_id ELSE NEW.recipe_id END;
    INSERT INTO trend_projection_queue(recipe_id,queued_at) VALUES(rid,now())
    ON CONFLICT(recipe_id) DO UPDATE SET queued_at=excluded.queued_at;
    RETURN COALESCE(NEW,OLD);
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_favorite_trend_refresh AFTER INSERT OR DELETE ON favorites FOR EACH ROW EXECUTE FUNCTION queue_recipe_trend_refresh();
CREATE TRIGGER trg_cook_trend_refresh AFTER INSERT OR UPDATE OF status,completed_at ON cook_sessions FOR EACH ROW EXECUTE FUNCTION queue_recipe_trend_refresh();
CREATE TRIGGER trg_review_trend_refresh AFTER INSERT OR UPDATE OF moderation_status OR DELETE ON reviews FOR EACH ROW EXECUTE FUNCTION queue_recipe_trend_refresh();
INSERT INTO trend_projection_queue(recipe_id) SELECT id FROM recipes ON CONFLICT DO NOTHING;

CREATE TABLE notification_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category VARCHAR(16) NOT NULL CHECK(category IN ('activity','streak')),
    channel VARCHAR(16) NOT NULL CHECK(channel IN ('in_app','email')),
    enabled BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id,category,channel)
);

ALTER TABLE notifications
    ADD COLUMN category VARCHAR(16) NOT NULL DEFAULT 'transactional' CHECK(category IN ('transactional','activity','streak')),
    ADD COLUMN read_at TIMESTAMPTZ,
    ADD COLUMN idempotency_key VARCHAR(255);
CREATE UNIQUE INDEX uq_notifications_intent ON notifications(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_notifications_in_app_unread ON notifications(user_id,created_at DESC,id DESC) WHERE channel='in_app' AND read_at IS NULL AND deleted_at IS NULL;
