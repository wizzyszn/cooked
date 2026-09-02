ALTER TABLE users ADD COLUMN streak_last_qualifying_date DATE;

CREATE TABLE cook_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE RESTRICT,
    recipe_version_id UUID NOT NULL REFERENCES recipe_versions(id) ON DELETE RESTRICT,
    status VARCHAR(16) NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress','completed','abandoned')),
    photo_media_id UUID REFERENCES media_assets(id) ON DELETE RESTRICT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    abandoned_at TIMESTAMPTZ,
    completion_local_date DATE,
    completion_timezone VARCHAR(64),
    xp_awarded INT NOT NULL DEFAULT 0 CHECK (xp_awarded >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status='completed' AND completed_at IS NOT NULL AND completion_local_date IS NOT NULL AND completion_timezone IS NOT NULL) OR status<>'completed'),
    CHECK ((status='abandoned' AND abandoned_at IS NOT NULL) OR status<>'abandoned')
);
CREATE UNIQUE INDEX uq_cook_session_active ON cook_sessions(user_id,recipe_version_id) WHERE status='in_progress';
CREATE INDEX idx_cook_sessions_user_cursor ON cook_sessions(user_id,started_at DESC,id DESC);
CREATE INDEX idx_cook_sessions_completion ON cook_sessions(user_id,completion_local_date,completed_at) WHERE status='completed';
CREATE INDEX idx_cook_sessions_recipe_day ON cook_sessions(user_id,recipe_id,completion_local_date) WHERE status='completed';

CREATE TABLE cook_session_steps (
    cook_session_id UUID NOT NULL REFERENCES cook_sessions(id) ON DELETE CASCADE,
    recipe_step_id UUID NOT NULL REFERENCES recipe_version_steps(id) ON DELETE RESTRICT,
    visited_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(cook_session_id,recipe_step_id)
);

CREATE TABLE cook_timers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cook_session_id UUID NOT NULL REFERENCES cook_sessions(id) ON DELETE CASCADE,
    recipe_step_id UUID NOT NULL REFERENCES recipe_version_steps(id) ON DELETE RESTRICT,
    state VARCHAR(16) NOT NULL CHECK(state IN ('running','paused')),
    duration_seconds INT NOT NULL CHECK(duration_seconds > 0),
    remaining_seconds INT NOT NULL CHECK(remaining_seconds >= 0),
    target_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(cook_session_id,recipe_step_id),
    CHECK ((state='running' AND target_at IS NOT NULL) OR state='paused')
);

CREATE TABLE cook_completion_commands (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(128) NOT NULL,
    cook_session_id UUID NOT NULL REFERENCES cook_sessions(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id,idempotency_key)
);

CREATE TABLE xp_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    cook_session_id UUID NOT NULL REFERENCES cook_sessions(id) ON DELETE RESTRICT,
    local_date DATE NOT NULL,
    kind VARCHAR(24) NOT NULL CHECK(kind IN ('base','photo_bonus','first_dish_bonus','daily_session_cap','recipe_day_cap')),
    amount INT NOT NULL CHECK(amount >= 0),
    decision VARCHAR(32) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id,idempotency_key,kind)
);
CREATE INDEX idx_xp_ledger_user_date ON xp_ledger_entries(user_id,local_date,created_at);

CREATE TABLE streak_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    cook_session_id UUID NOT NULL REFERENCES cook_sessions(id) ON DELETE RESTRICT,
    local_date DATE NOT NULL,
    previous_streak INT NOT NULL CHECK(previous_streak >= 0),
    new_streak INT NOT NULL CHECK(new_streak >= 0),
    decision VARCHAR(24) NOT NULL CHECK(decision IN ('started','advanced','same_day')),
    idempotency_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id,idempotency_key)
);

CREATE TABLE analytics_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    anonymous_id UUID,
    event_name VARCHAR(64) NOT NULL,
    schema_version SMALLINT NOT NULL DEFAULT 1,
    source VARCHAR(16) NOT NULL CHECK(source IN ('client','server')),
    cook_session_id UUID REFERENCES cook_sessions(id) ON DELETE SET NULL,
    recipe_id UUID REFERENCES recipes(id) ON DELETE SET NULL,
    recipe_version_id UUID REFERENCES recipe_versions(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(128),
    properties JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK(user_id IS NOT NULL OR anonymous_id IS NOT NULL)
);
CREATE UNIQUE INDEX uq_analytics_server_event ON analytics_events(source,idempotency_key,event_name) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_analytics_event_time ON analytics_events(event_name,occurred_at);
CREATE INDEX idx_analytics_user_time ON analytics_events(user_id,occurred_at) WHERE user_id IS NOT NULL;

CREATE FUNCTION prevent_reward_ledger_mutation() RETURNS TRIGGER AS $$
BEGIN RAISE EXCEPTION 'reward ledgers are append-only'; END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_immutable_xp_ledger BEFORE UPDATE OR DELETE ON xp_ledger_entries FOR EACH ROW EXECUTE FUNCTION prevent_reward_ledger_mutation();
CREATE TRIGGER trg_immutable_streak_ledger BEFORE UPDATE OR DELETE ON streak_ledger_entries FOR EACH ROW EXECUTE FUNCTION prevent_reward_ledger_mutation();
