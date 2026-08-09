-- Reverse cooking / social / discovery extensions (dependency order).

DROP TABLE IF EXISTS follows CASCADE;
DROP TABLE IF EXISTS comments CASCADE;
DROP TABLE IF EXISTS ratings CASCADE;
DROP TABLE IF EXISTS favorites CASCADE;
DROP TABLE IF EXISTS delicacy_tags CASCADE;
DROP TABLE IF EXISTS recipe_tags CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
DROP TABLE IF EXISTS recipe_steps CASCADE;
DROP TABLE IF EXISTS recipe_ingredients CASCADE;
DROP TABLE IF EXISTS ingredients CASCADE;

DROP INDEX IF EXISTS idx_recipes_public_rating;

ALTER TABLE recipes
    DROP COLUMN IF EXISTS avg_rating,
    DROP COLUMN IF EXISTS rating_count;
