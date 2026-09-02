CREATE TABLE categories (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name VARCHAR(100) NOT NULL, slug VARCHAR(100) NOT NULL, retired_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX uq_categories_name ON categories (lower(name)); CREATE UNIQUE INDEX uq_categories_slug ON categories (lower(slug));
CREATE TABLE regions (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name VARCHAR(100) NOT NULL, slug VARCHAR(100) NOT NULL, retired_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX uq_regions_name ON regions (lower(name)); CREATE UNIQUE INDEX uq_regions_slug ON regions (lower(slug));
CREATE TABLE measurement_units (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name VARCHAR(100) NOT NULL, symbol VARCHAR(32) NOT NULL, retired_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX uq_measurement_units_name ON measurement_units (lower(name)); CREATE UNIQUE INDEX uq_measurement_units_symbol ON measurement_units (lower(symbol));
ALTER TABLE delicacies ADD COLUMN category_id UUID REFERENCES categories(id) ON DELETE RESTRICT, ADD COLUMN cover_media_id UUID REFERENCES media_assets(id) ON DELETE SET NULL, ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','published','rejected','withdrawn','retired')), ADD COLUMN country_codes TEXT[] NOT NULL DEFAULT '{}', ADD COLUMN origin_notes TEXT, ADD COLUMN submitted_at TIMESTAMPTZ, ADD COLUMN published_at TIMESTAMPTZ, ADD COLUMN moderated_at TIMESTAMPTZ, ADD COLUMN moderated_by UUID REFERENCES users(id) ON DELETE SET NULL, ADD COLUMN moderation_reason TEXT;
UPDATE delicacies SET status='published', submitted_at=created_at, published_at=created_at WHERE deleted_at IS NULL;
CREATE INDEX idx_delicacies_public ON delicacies (published_at DESC, id DESC) WHERE deleted_at IS NULL AND status='published';
CREATE INDEX idx_delicacies_pending ON delicacies (submitted_at, id) WHERE deleted_at IS NULL AND status='pending';
CREATE INDEX idx_delicacies_name_trgm ON delicacies USING gin (lower(name) gin_trgm_ops) WHERE deleted_at IS NULL AND status IN ('pending','published');
CREATE TABLE delicacy_aliases (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), delicacy_id UUID NOT NULL REFERENCES delicacies(id) ON DELETE CASCADE, name VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX uq_live_delicacy_alias ON delicacy_aliases (lower(name)); CREATE INDEX idx_delicacy_aliases_dish ON delicacy_aliases (delicacy_id); CREATE INDEX idx_delicacy_aliases_trgm ON delicacy_aliases USING gin (lower(name) gin_trgm_ops);
CREATE FUNCTION prevent_dish_name_collision() RETURNS TRIGGER AS $$
BEGIN
  IF TG_TABLE_NAME = 'delicacies' AND EXISTS (SELECT 1 FROM delicacy_aliases WHERE lower(name)=lower(NEW.name)) THEN RAISE EXCEPTION 'dish name conflicts with alias' USING ERRCODE='unique_violation'; END IF;
  IF TG_TABLE_NAME = 'delicacy_aliases' AND EXISTS (SELECT 1 FROM delicacies WHERE deleted_at IS NULL AND status IN ('pending','published') AND lower(name)=lower(NEW.name)) THEN RAISE EXCEPTION 'alias conflicts with dish name' USING ERRCODE='unique_violation'; END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_dish_name_collision BEFORE INSERT OR UPDATE OF name ON delicacies FOR EACH ROW EXECUTE FUNCTION prevent_dish_name_collision();
CREATE TRIGGER trg_dish_alias_collision BEFORE INSERT OR UPDATE OF name ON delicacy_aliases FOR EACH ROW EXECUTE FUNCTION prevent_dish_name_collision();
CREATE TABLE delicacy_regions (delicacy_id UUID NOT NULL REFERENCES delicacies(id) ON DELETE CASCADE, region_id UUID NOT NULL REFERENCES regions(id) ON DELETE RESTRICT, PRIMARY KEY (delicacy_id, region_id));
CREATE INDEX idx_delicacy_regions_region ON delicacy_regions(region_id, delicacy_id);
CREATE TABLE delicacy_redirects (source_id UUID PRIMARY KEY REFERENCES delicacies(id) ON DELETE RESTRICT, target_id UUID NOT NULL REFERENCES delicacies(id) ON DELETE RESTRICT, created_by UUID REFERENCES users(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), CHECK (source_id <> target_id));
