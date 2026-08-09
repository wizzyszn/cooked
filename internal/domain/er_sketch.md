# Entity-relationship sketch

## Status

| Layer | Status |
|-------|--------|
| **v1** — users, delicacies, recipes | Implemented (`000001_initial`) |
| **Full ER** — cooking data, social, discovery | Implemented (`000002_cooking_social`) |

---

## v1 core

```
┌──────────────────┐         ┌─────────────────────────┐         ┌──────────────────┐
│      users       │ 1     * │         recipes         │ *     1 │    delicacies    │
├──────────────────┤─────────┤─────────────────────────┤─────────┤──────────────────┤
│ id (PK)          │         │ id (PK)                 │         │ id (PK)          │
│ email (unique)   │         │ user_id (FK → users)    │         │ name (unique*)   │
│ name             │         │ delicacy_id (FK, NN)    │         │ description      │
│ picture          │         │ title, summary, algo    │         │ thumbnail_urls   │
│ is_verified      │         │ imgs, visibility, …     │         │ created_by (FK?) │
│ timestamps + soft│         │ avg_rating, rating_count│         │ timestamps + soft│
└────────┬─────────┘         │ timestamps + soft       │         └────────▲─────────┘
         │                   └─────────────────────────┘                  │
         │ 0..1            *                                              │
         └──────────────────── created_by ────────────────────────────────┘

* unique on lower(name) where not soft-deleted
```

### Cardinality (core)

| Relationship | Cardinality | Notes |
|--------------|-------------|-------|
| User → Recipe | 1 : N | Owner; cascade delete |
| Delicacy → Recipe | 1 : N | Required; restrict delete while recipes exist |
| User → Delicacy | 0..1 : N | Optional contributor (`created_by`) |
| User × Delicacy recipes | N allowed | Same user may publish variations |

---

## Full ER (implemented)

```
                              ┌─────────────┐
                              │    tags     │
                              │ name, slug, │
                              │ kind        │
                              └──────┬──────┘
                                     │ M:N
                     ┌───────────────┴───────────────┐
                     │                               │
              recipe_tags                     delicacy_tags
                     │                               │
┌────────┐     ┌─────▼──────┐                  ┌─────▼──────┐
│ users  │     │  recipes   │                  │ delicacies │
└───┬────┘     └─────┬──────┘                  └────────────┘
    │                │
    │         ┌──────┼──────────────┐
    │         │      │              │
    │    recipe_   recipe_
    │  ingredients  steps
    │         │
    │         ▼
    │   ┌──────────┐
    │   │ingredients│  (global catalog)
    │   └──────────┘
    │
    ├── favorites (user M:N recipe)
    ├── follows   (user M:N user)
    ├── ratings   (user → recipe: score + review)
    └── comments  (user → recipe: threaded text)
```

`recipes.imgs` stays JSONB (not normalized to `recipe_images`).

---

## Tables

### Structured cooking

#### `ingredients`

| Column | Notes |
|--------|-------|
| id, timestamps, soft delete | Base |
| name | unique `lower(name)` among live rows |
| default_unit | optional (g, ml, cup, …) |

#### `recipe_ingredients`

| Column | Notes |
|--------|-------|
| id | surrogate PK |
| recipe_id | FK cascade |
| ingredient_id | FK restrict |
| quantity | nullable (“to taste”) |
| unit, note, position | display / shopping list |
| UNIQUE (recipe_id, ingredient_id) | one line per ingredient per recipe |

#### `recipe_steps`

| Column | Notes |
|--------|-------|
| recipe_id + position | unique order within recipe |
| body | instruction text |
| duration_minutes, image_url | optional |
| soft delete | |

`recipes.algo` remains free-text fallback / search blob.

### Discovery

#### `tags`

| Column | Notes |
|--------|-------|
| name, slug | unique among live rows |
| kind | `cuisine` \| `diet` \| `occasion` \| `general` |

#### `recipe_tags` / `delicacy_tags`

Composite PKs; cascade on both sides.

### Social

#### `favorites`

`(user_id, recipe_id)` PK + `created_at`.

#### `ratings`

One row per `(user_id, recipe_id)`; `score` 1–5; optional `body`.  
App (or trigger) should keep `recipes.avg_rating` / `rating_count` in sync.

#### `comments`

Threaded via `parent_id` → `comments`; soft delete.

#### `follows`

`(follower_id, following_id)` PK; `CHECK (follower_id <> following_id)`.

---

## Discovery query paths

1. **By delicacy** — public recipes for a dish  
2. **By user / following feed** — public recipes from followed authors  
3. **By tag** — join `recipe_tags` (+ optional delicacy tags)  
4. **By ingredient** — join `recipe_ingredients` (“has chicken + tomato”)  
5. **By rating** — sort public recipes on `avg_rating`, `rating_count`  
6. **Saved** — `favorites` for the current user  
7. **Global feed** — public recipes by `created_at`

Always filter `visibility = 'public'` (or owner) and `deleted_at IS NULL`.

---

## Domain files

| File | Entity |
|------|--------|
| `user.go` | User |
| `delicacy.go` | Delicacy |
| `recipe.go` | Recipe (+ visibility / difficulty) |
| `ingredient.go` | Ingredient |
| `recipe_ingredient.go` | RecipeIngredient |
| `recipe_step.go` | RecipeStep |
| `tag.go` | Tag, RecipeTag, DelicacyTag |
| `favorite.go` | Favorite |
| `rating.go` | Rating |
| `comment.go` | Comment |
| `follow.go` | Follow |

---

## Invariants

- Every recipe belongs to exactly one delicacy and one author.  
- Discovery respects `visibility` and soft deletes.  
- Unique indexes that involve names/slugs are partial (`WHERE deleted_at IS NULL`).  
- Prefer join tables over JSON once the field must be queried (ingredients, tags).  
- Keep denormalized rating columns consistent when ratings are written.
