ALTER TABLE recipes ADD COLUMN moderation_status VARCHAR(20) NOT NULL DEFAULT 'visible' CHECK (moderation_status IN ('visible','hidden','removed'));
ALTER TABLE recipes ADD COLUMN current_published_version_id UUID;
CREATE TABLE recipe_versions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
 version_number INT NOT NULL CHECK(version_number>0), lifecycle VARCHAR(16) NOT NULL CHECK(lifecycle IN('draft','published')),
 title VARCHAR(255) NOT NULL, summary VARCHAR(512) NOT NULL DEFAULT '', base_servings INT CHECK(base_servings>0),
 prep_time_seconds INT CHECK(prep_time_seconds>=0), cook_time_seconds INT CHECK(cook_time_seconds>=0),
 difficulty VARCHAR(16) CHECK(difficulty IN('easy','medium','hard')), notes TEXT NOT NULL DEFAULT '',
 legacy_image_urls JSONB NOT NULL DEFAULT '[]', published_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK((lifecycle='published' AND published_at IS NOT NULL) OR lifecycle='draft')
);
CREATE UNIQUE INDEX uq_recipe_version_number ON recipe_versions(recipe_id,version_number);
CREATE UNIQUE INDEX uq_recipe_active_draft ON recipe_versions(recipe_id) WHERE lifecycle='draft';
ALTER TABLE recipes ADD CONSTRAINT fk_recipe_current_version FOREIGN KEY(current_published_version_id) REFERENCES recipe_versions(id) ON DELETE RESTRICT;

CREATE TABLE recipe_version_ingredients (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), recipe_version_id UUID NOT NULL REFERENCES recipe_versions(id) ON DELETE CASCADE,
 ingredient_id UUID REFERENCES ingredients(id) ON DELETE RESTRICT, name VARCHAR(255) NOT NULL, quantity NUMERIC(12,3),
 measurement_unit_id UUID REFERENCES measurement_units(id) ON DELETE RESTRICT, display_amount VARCHAR(100), substitution_note VARCHAR(255), position INT NOT NULL CHECK(position>=0), deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_rvi_position ON recipe_version_ingredients(recipe_version_id,position) WHERE deleted_at IS NULL;
CREATE TABLE recipe_version_steps (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), recipe_version_id UUID NOT NULL REFERENCES recipe_versions(id) ON DELETE CASCADE,
 position INT NOT NULL CHECK(position>=0), title VARCHAR(160) NOT NULL DEFAULT '', instruction TEXT NOT NULL,
 action VARCHAR(16) NOT NULL DEFAULT 'other' CHECK(action IN('sauté','boil','simmer','fry','bake','grill','fold','whisk','chop','marinate','rest','other')),
 duration_seconds INT CHECK(duration_seconds>=0), technique_tags TEXT[] NOT NULL DEFAULT '{}', deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_rvs_position ON recipe_version_steps(recipe_version_id,position) WHERE deleted_at IS NULL;
CREATE TABLE recipe_step_ingredients(step_id UUID NOT NULL REFERENCES recipe_version_steps(id) ON DELETE CASCADE, ingredient_entry_id UUID NOT NULL REFERENCES recipe_version_ingredients(id) ON DELETE CASCADE, PRIMARY KEY(step_id,ingredient_entry_id));
CREATE TABLE recipe_version_tags(recipe_version_id UUID NOT NULL REFERENCES recipe_versions(id) ON DELETE CASCADE, tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE RESTRICT, PRIMARY KEY(recipe_version_id,tag_id));
CREATE TABLE recipe_version_media(recipe_version_id UUID NOT NULL REFERENCES recipe_versions(id) ON DELETE CASCADE, media_asset_id UUID NOT NULL REFERENCES media_assets(id) ON DELETE RESTRICT, purpose VARCHAR(16) NOT NULL CHECK(purpose IN('cover','step')), step_id UUID REFERENCES recipe_version_steps(id) ON DELETE CASCADE, position INT NOT NULL DEFAULT 0, PRIMARY KEY(recipe_version_id,media_asset_id), CHECK((purpose='cover' AND step_id IS NULL) OR (purpose='step' AND step_id IS NOT NULL)));
CREATE TABLE recipe_publish_commands(recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE, idempotency_key VARCHAR(128) NOT NULL, published_version_id UUID REFERENCES recipe_versions(id) ON DELETE RESTRICT, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(recipe_id,idempotency_key));

INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title,summary,base_servings,prep_time_seconds,cook_time_seconds,difficulty,notes,legacy_image_urls,published_at,created_at,updated_at)
SELECT gen_random_uuid(),id,1,'published',title,coalesce(summary,''),servings,prep_time_minutes*60,cook_time_minutes*60,difficulty,coalesce(algo,''),coalesce(imgs,'[]'::jsonb),created_at,created_at,updated_at FROM recipes;
UPDATE recipes r SET current_published_version_id=v.id FROM recipe_versions v WHERE v.recipe_id=r.id AND v.version_number=1;
INSERT INTO recipe_version_ingredients(id,recipe_version_id,ingredient_id,name,quantity,display_amount,substitution_note,position)
SELECT ri.id,v.id,ri.ingredient_id,i.name,ri.quantity,ri.unit,ri.note,ri.position FROM recipe_ingredients ri JOIN ingredients i ON i.id=ri.ingredient_id JOIN recipe_versions v ON v.recipe_id=ri.recipe_id AND v.version_number=1;
INSERT INTO recipe_version_steps(id,recipe_version_id,position,instruction,duration_seconds)
SELECT rs.id,v.id,rs.position,rs.body,rs.duration_minutes*60 FROM recipe_steps rs JOIN recipe_versions v ON v.recipe_id=rs.recipe_id AND v.version_number=1 WHERE rs.deleted_at IS NULL;
INSERT INTO recipe_version_tags SELECT v.id,rt.tag_id FROM recipe_tags rt JOIN recipe_versions v ON v.recipe_id=rt.recipe_id AND v.version_number=1;

CREATE FUNCTION prevent_published_version_mutation() RETURNS TRIGGER AS $$ BEGIN IF OLD.lifecycle='published' THEN RAISE EXCEPTION 'published recipe versions are immutable'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_immutable_recipe_version BEFORE UPDATE OR DELETE ON recipe_versions FOR EACH ROW EXECUTE FUNCTION prevent_published_version_mutation();
CREATE FUNCTION prevent_published_child_mutation() RETURNS TRIGGER AS $$ DECLARE vid UUID; BEGIN vid:=coalesce(NEW.recipe_version_id,OLD.recipe_version_id);IF EXISTS(SELECT 1 FROM recipe_versions WHERE id=vid AND lifecycle='published') THEN RAISE EXCEPTION 'published recipe version content is immutable';END IF;RETURN coalesce(NEW,OLD);END;$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_immutable_rvi BEFORE INSERT OR UPDATE OR DELETE ON recipe_version_ingredients FOR EACH ROW EXECUTE FUNCTION prevent_published_child_mutation();
CREATE TRIGGER trg_immutable_rvs BEFORE INSERT OR UPDATE OR DELETE ON recipe_version_steps FOR EACH ROW EXECUTE FUNCTION prevent_published_child_mutation();
CREATE TRIGGER trg_immutable_rvt BEFORE INSERT OR UPDATE OR DELETE ON recipe_version_tags FOR EACH ROW EXECUTE FUNCTION prevent_published_child_mutation();
CREATE TRIGGER trg_immutable_rvm BEFORE INSERT OR UPDATE OR DELETE ON recipe_version_media FOR EACH ROW EXECUTE FUNCTION prevent_published_child_mutation();
