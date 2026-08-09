-- Extends v1 with structured cooking data, social graph, and richer discovery.

-- ---------------------------------------------------------------------------
-- Denormalized rating stats on recipes (for feed / sort)
-- ---------------------------------------------------------------------------
ALTER TABLE recipes
    ADD COLUMN IF NOT EXISTS avg_rating NUMERIC(3, 2) NOT NULL DEFAULT 0
        CHECK (avg_rating >= 0 AND avg_rating <= 5),
    ADD COLUMN IF NOT EXISTS rating_count INT NOT NULL DEFAULT 0
        CHECK (rating_count >= 0);

CREATE INDEX IF NOT EXISTS idx_recipes_public_rating
    ON recipes (avg_rating DESC, rating_count DESC)
    WHERE deleted_at IS NULL AND visibility = 'public';

-- ---------------------------------------------------------------------------
-- ingredients (global catalog)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ingredients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    default_unit VARCHAR(32)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ingredients_name_lower
    ON ingredients (lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ingredients_deleted_at ON ingredients (deleted_at);

-- ---------------------------------------------------------------------------
-- recipe_ingredients
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS recipe_ingredients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id UUID NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL REFERENCES ingredients (id) ON DELETE RESTRICT,
    quantity NUMERIC(12, 3),
    unit VARCHAR(32),
    note VARCHAR(255),
    position INT NOT NULL DEFAULT 0,
    CONSTRAINT uq_recipe_ingredient UNIQUE (recipe_id, ingredient_id)
);

CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_recipe_id
    ON recipe_ingredients (recipe_id, position);

CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_ingredient_id
    ON recipe_ingredients (ingredient_id);

-- ---------------------------------------------------------------------------
-- recipe_steps (ordered instructions)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS recipe_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    recipe_id UUID NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    position INT NOT NULL,
    body TEXT NOT NULL,
    duration_minutes INT CHECK (duration_minutes IS NULL OR duration_minutes >= 0),
    image_url VARCHAR(512)
);

-- Soft-delete friendly: position may be reused after a step is deleted.
CREATE UNIQUE INDEX IF NOT EXISTS uq_recipe_step_pos
    ON recipe_steps (recipe_id, position)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_recipe_steps_recipe_id
    ON recipe_steps (recipe_id, position)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_recipe_steps_deleted_at ON recipe_steps (deleted_at);

-- ---------------------------------------------------------------------------
-- tags + M:N joins (discovery)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(64) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL DEFAULT 'general'
        CHECK (kind IN ('cuisine', 'diet', 'occasion', 'general'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_slug
    ON tags (slug)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_name_lower
    ON tags (lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tags_kind ON tags (kind) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tags_deleted_at ON tags (deleted_at);

CREATE TABLE IF NOT EXISTS recipe_tags (
    recipe_id UUID NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_recipe_tags_tag_id ON recipe_tags (tag_id);

CREATE TABLE IF NOT EXISTS delicacy_tags (
    delicacy_id UUID NOT NULL REFERENCES delicacies (id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (delicacy_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_delicacy_tags_tag_id ON delicacy_tags (tag_id);

-- ---------------------------------------------------------------------------
-- favorites (user saves a recipe)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS favorites (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    recipe_id UUID NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, recipe_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_recipe_id ON favorites (recipe_id);
CREATE INDEX IF NOT EXISTS idx_favorites_user_created
    ON favorites (user_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- ratings (one per user per recipe)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ratings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    recipe_id UUID NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    score SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rating_user_recipe UNIQUE (user_id, recipe_id)
);

CREATE INDEX IF NOT EXISTS idx_ratings_recipe_id ON ratings (recipe_id);
CREATE INDEX IF NOT EXISTS idx_ratings_recipe_score ON ratings (recipe_id, score);

-- ---------------------------------------------------------------------------
-- comments (threaded)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    recipe_id UUID NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id UUID REFERENCES comments (id) ON DELETE CASCADE,
    body TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_comments_recipe_id
    ON comments (recipe_id, created_at)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments (user_id);
CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments (parent_id)
    WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_comments_deleted_at ON comments (deleted_at);

-- ---------------------------------------------------------------------------
-- follows (social graph)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS follows (
    follower_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    following_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT chk_follows_not_self CHECK (follower_id <> following_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_following_id ON follows (following_id);
CREATE INDEX IF NOT EXISTS idx_follows_follower_created
    ON follows (follower_id, created_at DESC);
