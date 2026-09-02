# M3 — Curated Dish taxonomy and moderation

M3 turns the legacy Delicacy record into Cooked's trusted Dish catalog. A verified user can suggest a Dish, but it stays private in `pending` until staff approves it. Staff may also create a published canonical Dish directly.

## Product behavior

- Every new Dish has one category, at least one region, aliases, optional ISO country codes/origin notes, and an optional processed cover-media reference.
- Exact normalized names and aliases cannot collide. PostgreSQL trigram ranking catches likely misspellings and requires `confirm_similar: true` after the contributor reviews suggestions.
- Submitters may edit or withdraw only their own pending entries.
- Moderators approve or reject with a reason. The state change, audit row, and submitter's in-app outcome notification commit together.
- Public list/detail requests only return published Dishes. Category and region slugs provide browse filters.
- Admin merge locks both Dishes, moves child Recipes, consolidates aliases and regions, retires the source, creates a permanent redirect, and writes one before/after audit record in a single transaction.
- Categories, regions, dietary tags, and measurement units are retired rather than deleted.

## Routes

- Public: `GET /api/v1/delicacies`, `GET /api/v1/delicacies/:id`, `GET /api/v1/delicacies/duplicate-suggestions`, `GET /api/v1/taxonomies`
- Verified contributor: `POST /api/v1/delicacies`, `PATCH /api/v1/delicacies/:id`, `POST /api/v1/delicacies/:id/withdraw`
- Moderator/Admin: `GET|POST /api/v1/staff/delicacies`, approve/reject routes, and taxonomy routes
- Admin: `POST /api/v1/admin/delicacies/:id/merge`

## Verification

- Full Go unit suite and static analysis.
- Migration v6 → v10, latest rollback/reapply, and empty schema → v10 against an isolated PostgreSQL schema.
- Service integration coverage for duplicate confirmation, visibility, browse, approval, rejection, withdrawal, merge rollback, recipe reassignment, redirects, notifications, and exactly-one audit records.
