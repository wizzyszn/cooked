-- M5 discovery indexes. All recipe discovery reads use the stable Recipe row
-- joined to its current immutable published version.
CREATE INDEX idx_recipe_versions_title_trgm
    ON recipe_versions USING gin (lower(title) gin_trgm_ops)
    WHERE lifecycle = 'published';

CREATE INDEX idx_recipe_versions_discovery
    ON recipe_versions (published_at DESC, id DESC)
    WHERE lifecycle = 'published';

CREATE INDEX idx_recipes_public_current_version
    ON recipes (current_published_version_id)
    WHERE visibility = 'public' AND moderation_status = 'visible' AND deleted_at IS NULL;

CREATE INDEX idx_recipe_versions_filters
    ON recipe_versions (difficulty, (prep_time_seconds + cook_time_seconds), published_at DESC, id DESC)
    WHERE lifecycle = 'published'
      AND prep_time_seconds IS NOT NULL
      AND cook_time_seconds IS NOT NULL;

CREATE INDEX idx_delicacies_public_recent
    ON delicacies (published_at DESC, id DESC)
    WHERE status = 'published' AND deleted_at IS NULL;

CREATE INDEX idx_tags_kind_slug ON tags (kind, slug);
CREATE INDEX idx_recipe_version_tags_tag_version ON recipe_version_tags (tag_id, recipe_version_id);
CREATE INDEX idx_delicacy_regions_region_dish ON delicacy_regions (region_id, delicacy_id);

-- The legacy index omits the UUID tie-breaker required for deterministic cursors.
CREATE INDEX idx_favorites_user_cursor ON favorites (user_id, created_at DESC, recipe_id DESC);
