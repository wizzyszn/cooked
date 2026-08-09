-- v1 schema: users, delicacies, recipes
-- Delicacy = shared dish identity; Recipe = one user's method for that dish.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    picture VARCHAR(512),
    is_verified BOOLEAN NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- ---------------------------------------------------------------------------
-- delicacies (catalog of dishes; optionally user-contributed)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS delicacies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    thumbnail_urls JSONB,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL
);

-- Case-insensitive unique name among live rows (soft-delete friendly).
CREATE UNIQUE INDEX IF NOT EXISTS idx_delicacies_name_lower
    ON delicacies (lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_delicacies_created_by ON delicacies (created_by);
CREATE INDEX IF NOT EXISTS idx_delicacies_deleted_at ON delicacies (deleted_at);

-- ---------------------------------------------------------------------------
-- recipes (owned by a user, always attached to a delicacy)
-- Multiple recipes per (user, delicacy) are allowed (variations).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    delicacy_id UUID NOT NULL REFERENCES delicacies (id) ON DELETE RESTRICT,
    title VARCHAR(255) NOT NULL,
    summary VARCHAR(512),
    algo TEXT NOT NULL,
    imgs JSONB,
    visibility VARCHAR(32) NOT NULL DEFAULT 'public'
        CHECK (visibility IN ('public', 'private', 'unlisted')),
    prep_time_minutes INT CHECK (prep_time_minutes IS NULL OR prep_time_minutes >= 0),
    cook_time_minutes INT CHECK (cook_time_minutes IS NULL OR cook_time_minutes >= 0),
    servings INT CHECK (servings IS NULL OR servings > 0),
    difficulty VARCHAR(32)
        CHECK (difficulty IS NULL OR difficulty IN ('easy', 'medium', 'hard'))
);

-- A user's cookbook
CREATE INDEX IF NOT EXISTS idx_recipes_user_id
    ON recipes (user_id)
    WHERE deleted_at IS NULL;

-- Recipes for a dish (primary discovery path)
CREATE INDEX IF NOT EXISTS idx_recipes_delicacy_id
    ON recipes (delicacy_id)
    WHERE deleted_at IS NULL;

-- Public browse / feed
CREATE INDEX IF NOT EXISTS idx_recipes_public_created_at
    ON recipes (created_at DESC)
    WHERE deleted_at IS NULL AND visibility = 'public';

CREATE INDEX IF NOT EXISTS idx_recipes_deleted_at ON recipes (deleted_at);
CREATE INDEX IF NOT EXISTS idx_recipes_visibility ON recipes (visibility)
    WHERE deleted_at IS NULL;
