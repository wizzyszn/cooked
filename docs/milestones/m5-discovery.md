# M5 — Favorites, search, browse, and initial discovery

## Delivered

- Idempotent save and unsave commands, plus a deterministic cursor-paginated saved list.
- Access filtering at write and read time so private, unlisted, deleted, hidden, draft, or otherwise inaccessible Recipes disclose no saved-content metadata.
- Public Dish search across canonical names and aliases with PostgreSQL trigram matching.
- Indexed current-version Recipe-title search with dietary, difficulty, maximum-total-time, category, and region filters.
- Category/region Dish browse, recently approved Dishes, and authenticated recommendations matching any of the user's dietary preferences.
- Migration `000012` with discovery, filter, join, and favorite-cursor indexes.
- An opt-in, disposable 50,000-Recipe load profile in `internal/discovery/load_test.go`.

Final trending remains intentionally owned by M8 because Cook and Review signals do not exist yet.

## API routes

- `GET /api/v1/search`
- `GET /api/v1/browse/dishes`
- `GET /api/v1/discovery/recent-dishes`
- `GET /api/v1/discovery/recommendations`
- `GET /api/v1/users/me/favorites`
- `PUT /api/v1/recipes/{id}/favorite`
- `DELETE /api/v1/recipes/{id}/favorite`

Collections default to 20 items, cap at 50, and use timestamp-plus-UUID cursors for stable ordering.

## Performance evidence

Run on 2 September 2026 with Go 1.26.7, 8 logical CPUs, PostgreSQL database size 65,281,047 bytes, a warm application/repository layer, 50 concurrent clients, and a disposable dataset of 50,000 public Recipes.

- Duration: 5 minutes
- Requests: 563,687
- Failures: 0
- Server-side p95: 57.324035 ms
- Target: below 300 ms p95
- Query mix: unfiltered title search; difficulty; category; region plus maximum total time
- Plan assertion: Recipe title search used `idx_recipe_versions_title_trgm` through a bitmap index scan

Command:

```bash
COOKED_RUN_M5_LOAD_TEST=1 go test -p 1 ./internal/discovery -run TestSearchLoadProfile -v
```

The test creates and removes an isolated PostgreSQL schema. It does not migrate or seed the normal application schema.

## X thread draft

1/ I’m building Cooked because I don’t know how to cook—and “just find a recipe” is more frustrating than it sounds when you don’t know what fits your time, diet, or skill level.

This milestone makes finding something I can actually cook feel practical. 🧵

2/ Cooked can now search real dishes (including alternate names and misspellings) and published recipes, then narrow the results by difficulty, total cooking time, dietary preference, category, and region.

So “easy vegetarian West African meal under an hour” becomes a useful query, not a guessing game.

3/ I also added personalized discovery. If I save dietary preferences, Cooked recommends public recipes matching any of them—and clearly treats those tags as helpful labels, not food-safety certification.

Recently approved dishes give me another low-pressure way to explore what to learn next.

4/ Recipes can now be saved and unsaved safely. The operation is idempotent, so retries cannot create duplicates, and if a saved recipe later becomes private, hidden, unlisted, or deleted, it disappears without leaking its title or other metadata.

5/ Under the hood, every discovery path reads only the current immutable published recipe version. Drafts and inaccessible content are excluded in the database query itself, and cursor pagination stays stable even when timestamps match.

6/ I tested it with 50,000 public recipes and 50 concurrent clients for five minutes: 563,687 searches, zero failures, and 57.32ms p95 server time—well under the 300ms target.

Still a long road, but Cooked is becoming the guide I wish I had when I first tried to learn my way around a kitchen.
