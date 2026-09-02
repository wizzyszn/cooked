# M4 — Recipe identity, immutable versions, and authoring

M4 gives Cooked a safe authoring model: a Recipe is the stable identity people save and share, while every publication is a complete, immutable snapshot. Authors can improve instructions without silently changing the version someone previously cooked.

## Product behavior

- Authenticated users create a Recipe with a private working draft; verification is required only when publishing.
- A snapshot includes title, summary, servings, preparation/cook seconds, difficulty, notes, ordered ingredients, ordered Steps, actions, technique tags, ingredient references, taxonomy tags, and processed media.
- Publishing is transactional and requires an `Idempotency-Key`. Concurrent retries resolve to the same published version.
- Editing after publication clones the current snapshot into one mutable draft. Publishing it makes the new immutable version current and preserves historical direct links.
- Public Recipes are guest-readable, unlisted Recipes are link-readable, and private/draft content is author/staff-only. Deleted or moderation-hidden Recipes disclose nothing.
- Historical responses include an `outdated` marker.
- Requested servings scale numeric quantities as a response projection; display-only amounts such as “to taste” remain unchanged and persisted content is never modified.
- Private Recipe media now follows the Recipe access policy; publishing rejects media with the wrong purpose, owner, processing state, or moderation state.

## Migration

Migration 11 retains all legacy columns for rollback safety while backfilling each legacy Recipe into published version 1. Ingredients, Steps, tags, notes, time values, servings, and legacy image URLs are copied and reconciled in the PostgreSQL migration test.

Database triggers reject changes to published versions and their Ingredient, Step, tag, and media rows. Partial unique indexes enforce one draft per Recipe, one version number per Recipe, and stable ordered positions.

## Routes

- `POST /api/v1/recipes`
- `GET /api/v1/recipes/:id`
- `GET /api/v1/recipe-versions/:id`
- `GET|PUT /api/v1/recipes/:id/draft`
- `POST /api/v1/recipes/:id/publish`
- `PATCH /api/v1/recipes/:id/visibility`
- `DELETE /api/v1/recipes/:id`

## Verification

The tests cover clean/v6 migration through v11, rollback/reapply, legacy backfill reconciliation, validation and serving projections, immutable database enforcement, draft cloning, historical reads, the visibility/deletion matrix, and concurrent idempotent publication.
