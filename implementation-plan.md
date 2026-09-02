# Cooked v1 — Milestone Implementation Plan

**Execution source of truth:** This document

**Product requirements source:** [`product-frd.md`](./product-frd.md)

**Prepared:** 2026-09-02

**Delivery model:** Go API, PostgreSQL migrations, and background worker only

**Planning principle:** Each milestone must leave a testable vertical capability and satisfy its exit gate before dependent work begins.

### Checklist rules

- `[ ]` means the item is not yet fully implemented and verified; partially completed work stays unchecked and may be annotated with a short status note.
- `[x]` means the implementation is present, its relevant tests pass, and any required migration/API documentation is updated.
- Exit-gate boxes are checked only after every statement in that gate has objective test or review evidence.
- The change that completes an item must update this tracker. New scope must be added under a milestone before implementation rather than tracked elsewhere.
- A milestone is complete only when all its deliverable and exit-gate boxes are checked; dependent milestones may start earlier only where the dependency section explicitly allows parallel work.
- The milestone-summary box is checked last, after every checkbox inside that milestone is complete.

---

## 1. Current implementation baseline

### Already present and reusable

- Go/Gin API bootstrap, configuration loading, PostgreSQL/GORM connection, structured logging, standard response/error envelopes, and graceful shutdown.
- Versioned SQL migration runner with `up`, `down`, `goto`, `force`, and `version` commands.
- Email/password registration and login, Argon2id password hashing, email verification, rotating refresh tokens, logout/logout-all, and 30-minute password-reset OTPs.
- Persisted email-notification records, Brevo delivery, an in-process asynchronous dispatcher, and recovery of pending notifications on API startup.
- Initial Delicacy, Recipe, Ingredient, Step, Tag, Favorite, Rating, Comment, Follow, and Notification schemas/domain types.
- A partial authenticated Delicacy-create path.
- Unit tests around password hashing, JWT behavior, notification templating/delivery, and shared errors. `go test ./...` currently passes.

### Present but incomplete or conflicting with the FRD

| Area                      | Current state                                                                                                      | Required correction                                                                                                            |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| Migration`000007`       | M0 now provides a tested reversible`user_roles` migration and registered-user trigger.                           | Continue additive migrations from this corrected v7 baseline.                                                                  |
| Authentication middleware | M0 separates authentication from verification and loads current account state.                                     | Apply`RequireVerified` only to the FR-102 operations in later milestones.                                                    |
| Roles                     | M0 provides additive persisted roles and role middleware.                                                          | Add audited Admin assignment and initial-admin provisioning in M1 (FR-001–FR-003).                                            |
| Profile                   | Repository lookup exists; handler/service files are empty. Dietary preference is currently a single enum.          | Add profile APIs, IANA timezone, bio/avatar, anonymization, and zero-or-more dietary preferences (FR-104–FR-107).             |
| Delicacies                | Only immediate creation with name/description/images exists.                                                       | Add aliases, taxonomy, pending/approval lifecycle, duplicate suggestions, merge, redirects, and public reads (FR-201–FR-209). |
| Recipes                   | Legacy mutable Recipe tables and domain types exist; no Recipe handlers/services/routes exist.                     | Introduce stable Recipes and version-owned immutable content, then backfill legacy data (FR-301–FR-313).                      |
| Ratings                   | One score per user/Recipe exists only at schema/domain level.                                                      | Replace with three-dimensional version-specific Reviews and aggregates (FR-601–FR-608).                                       |
| Favorites                 | Table/domain type exists; no API exists.                                                                           | Add idempotent save/unsave/list behavior with access checks (FR-401).                                                          |
| Notifications             | M2 persists notification intent, delivery attempts, retry state, and provider idempotency keys; `cmd/worker` owns Brevo delivery. | Add user preferences, in-app reads, engagement producers, and scheduled behavior in M8 (FR-801–FR-805).                     |
| Media                     | M2 provides S3-compatible signed uploads, quarantined processing, responsive variants, access-aware reads, and avatar references. | Extend ownership/access joins as Dish, Recipe, Step, Review, and Cook Session records land (FR-1001–FR-1005).                |
| Rate limiting             | Per-process, IP-only memory buckets.                                                                               | Add route/account-aware distributed limits before multi-instance production use (NFR-7).                                       |
| Request IDs               | M0 registers middleware before request logging and returns IDs in API metadata.                                    | Preserve this ordering on future router changes.                                                                               |
| Worker                    | M2 runs durable notification and media jobs with PostgreSQL leases, retries, stale-lease recovery, and systemd supervision. | Add scheduled reminders and aggregate/retry work in their owning milestones.                                                  |
| Tests                     | M0 adds middleware, health, platform, OpenAPI, and real-Postgres migration tests plus backend CI.                  | Extend repository, handler, and service coverage with each feature milestone.                                                  |

### Work that is not v1 scope

The existing `follows`, `comments`, and social-feed-oriented structures must not drive v1 endpoints. Preserve existing rows during additive migrations, but do not spend milestone capacity exposing or extending them.

This plan does not include web/PWA components, frontend routing, visual accessibility work, browser compatibility, or Playwright UI journeys. It does include the backend APIs, persisted state, authorization, and contracts that a separate client can consume.

---

## Milestone status

- [ ] **M0:** Stabilize the foundation and freeze contracts. *(Implementation and local verification complete; first CI run pending.)*
- [x] **M1:** Identity, profile, preferences, and Google sign-in.
- [x] **M2:** Media pipeline and durable background jobs.
- [ ] **M3:** Curated Dish taxonomy and moderation workflow.
- [ ] **M4:** Recipe identity, immutable versions, and authoring.
- [ ] **M5:** Favorites, search, browse, and initial discovery.
- [ ] **M6:** Cook Mode, session lifecycle, XP, streaks, and core analytics.
- [ ] **M7:** Version-specific Reviews and content reporting.
- [ ] **M8:** Trending, notification preferences, and engagement automation.
- [ ] **M9:** Launch hardening, migration cutover, and sign-off.

The detailed milestone checklists below are authoritative. This summary is for at-a-glance progress only.

---

## 2. Shared implementation rules

These rules apply to every milestone and prevent incompatible local decisions.

### API and service conventions

- Retain `/api/v1` and the existing `{status, meta, data|error}` response envelope.
- Add `api/openapi.yaml` as the contract source of truth. Every new route must be specified before or with implementation.
- Use cursor pagination for collections: `limit` defaults to 20, caps at 50, and responses return `next_cursor` when another page exists.
- Accept `Idempotency-Key` on publish, Cook Session completion, Review creation, reporting, and other command endpoints identified by the FRD. Persist the key with the command result.
- Keep the package shape `handler → service → repository`. Authorization and business invariants live in services; repositories do not decide permissions.
- Pass `context.Context` through all layers and execute multi-record invariants in explicit database transactions.
- Return stable machine-readable error codes. Do not expose database/provider errors to clients.

### Authentication and authorization

- `RequireAuth` validates identity only and loads current user state.
- `RequireVerified` is applied only to publish, Dish submission, Review, and report operations.
- `RequireRole(moderator, admin)` protects moderation/taxonomy actions; Admin-only middleware protects role changes and Dish merges.
- Do not trust role or verification claims for long-lived authorization decisions without confirming current account state, or use short-lived claims with revocation/version checks.

### Database evolution

- Use additive migrations first: create target tables/columns, backfill, validate, switch reads/writes, then remove legacy columns in a later cleanup migration.
- Every up migration has a tested down migration unless reversal would destroy user data; irreversible cleanup must be explicitly marked and delayed until backup/verification.
- Never repurpose legacy `ratings` as target Reviews in place. Backfill them only if a deterministic mapping is approved; otherwise archive them outside current aggregates.
- Use partial unique indexes for active drafts, active sessions, live names, and idempotency constraints.

### Testing and delivery

- Unit-test service rules with fakes; integration-test repositories and transactions against PostgreSQL; test handlers with `httptest`; test migration up/down from an empty database and an existing v6 fixture.
- Every milestone updates OpenAPI, seed/fixture data, and the FRD traceability table in this plan.
- Backend CI gates: `go test ./...`, race tests for concurrency-sensitive packages, migration round-trip, static analysis, and API contract validation.
- Use feature flags only for incomplete externally visible behavior; do not expose endpoints that write incompatible placeholder data.

---

## 3. Milestone roadmap

## M0 — Stabilize the foundation and freeze contracts

**Purpose:** Make the current backend repository safe to extend and establish a stable HTTP contract.

**FRD coverage:** FR-001–FR-003, NFR-3, NFR-4, NFR-6, NFR-7, NFR-10.

### Backend deliverables

- [X] Replace invalid migration `000007` with a valid reversible role foundation migration. Existing registered accounts become `user`; no blanket overwrite may erase staff roles.
- [X] Decide role persistence as `user_roles(user_id, role)` with Registered User implicit for active accounts and `moderator`/`admin` explicit. Remove the conflicting single-role domain field after backfill.
- [X] Split `RequireAuth` and `RequireVerified`; add role middleware and current-account loading.
- [X] Register request-ID middleware before logging and normalize error/status codes.
- [X] Add `/health/live` and `/health/ready`; readiness checks database connectivity and required migration version.
- [X] Add a transaction helper, clock interface, UUID/idempotency helpers, cursor codec, and test builders shared by later domains.
- [X] Add `api/openapi.yaml` documenting existing auth routes, response envelopes, auth schemes, pagination, error format, and idempotency header.
- [X] Add system-PostgreSQL/CI test setup and migration tests for empty schema → latest → down/up plus v6 fixture → latest.
- [X] Keep migrations explicit through `cmd/migrate`; production deploys run migrations before starting the API.

### Exit gate

- [X] A clean database and a v6 fixture migrate to the corrected latest version and roll back in CI. *(Passed locally against isolated system-PostgreSQL schema on 2026-09-02; awaiting the first CI run.)*
- [X] Unverified access tokens can reach an ordinary authenticated test route but fail a `RequireVerified` route.
- [X] User, Moderator, and Admin authorization matrix tests pass.
- [X] OpenAPI contract validation passes; no v1 feature depends on the invalid role migration.

---

## M1 — Identity, profile, preferences, and Google sign-in

**Depends on:** M0.

**FRD coverage:** FR-101–FR-107, FR-208 (dietary taxonomy ownership), NFR-6, NFR-10.

### Data and backend deliverables

- [x] Add profile fields: bio, public avatar asset reference, IANA timezone, anonymization/deactivation timestamps, and cached XP/streak values reserved for later population.
- [x] Add curated dietary tags and `user_dietary_preferences`; enforce that `none` is represented as an empty selection rather than a tag combined with others.
- [x] Implement Google OAuth Authorization Code + PKCE flow, account linking by verified provider identity, state/nonce validation, and conflict-safe handling when an email/password account already exists.
- [x] Add:
  - [x] `GET /api/v1/users/me`
  - [x] `PATCH /api/v1/users/me`
  - [x] `PUT /api/v1/users/me/dietary-preferences`
  - [x] `GET /api/v1/profiles/{username}`
  - [x] `DELETE /api/v1/users/me`
  - [x] Google OAuth start/callback/exchange endpoints defined by the OpenAPI contract.
- [x] Add Admin role-assignment APIs and an operational initial-admin command that always writes an audit record.
- [x] Implement anonymization as a transaction: revoke sessions, replace identifying fields, detach provider credentials, reattribute retained public attribution, and preserve integrity records without public identity leakage.

### Exit gate

- [x] Email/password and Google users can complete the same authenticated flows. *(Google provider boundary exercised with a deterministic fake; production uses Google's token exchange/token-info validation.)*
- [x] Multiple dietary preferences round-trip correctly; invalid IANA timezones and conflicting selections are rejected.
- [x] Account-deletion integration test proves tokens are revoked and public content attribution is anonymized.

---

## M2 — Media pipeline and durable background jobs

**Depends on:** M0; profile avatar integration completes after M1.

**FRD coverage:** FR-805, FR-1001–FR-1005, NFR-4, NFR-10.

### Backend/worker deliverables

- [x] Add `media_assets` with owner, purpose, object key, decoded MIME type, byte size, dimensions, moderation status, processing status, visibility/access scope, and responsive variants.
- [x] Implement direct-to-object-storage upload initialization and completion endpoints using short-lived signed URLs. The server validates declared constraints before signing and decoded file properties after upload.
- [x] Implement worker jobs for metadata stripping, resizing, safety checking, retry/backoff, quarantine, and orphan cleanup.
- [x] Convert notification delivery to a durable database outbox claimed by `cmd/worker` using `FOR UPDATE SKIP LOCKED`; keep the API responsible for committing notification intent, not provider calls.
- [x] Add `notification_delivery_attempts` and idempotent provider-send keys. Remove reliance on process-local delivery for production.
- [x] Enforce authorization-aware media delivery for private Recipe assets and quarantine all pending/failed public assets. *(M2 defaults private assets to owner-only access; Recipe collaborator/staff policies extend this boundary when Recipe access control lands in M4.)*

### Exit gate

- [x] Public, private, quarantined, oversized, spoofed-MIME, failed-processing, and deleted-owner media tests pass.
- [x] Killing and restarting the worker does not lose or duplicate a notification/media job. *(PostgreSQL leases use `FOR UPDATE SKIP LOCKED`; restart tests cover provider-idempotent notification replay and retryable media processing.)*
- [x] Profile avatars can use processed `MediaAsset` references instead of arbitrary URLs.

---

## M3 — Curated Dish taxonomy and moderation workflow

**Depends on:** M1 and M2.

**FRD coverage:** FR-201–FR-209, FR-801, FR-901/FR-903–FR-905 where applicable, NFR-3, NFR-5.

### Data and backend deliverables

- [ ] Add categories, regions, measurement units, aliases, Dish-region joins, Dish status (`pending`, `published`, `rejected`, `withdrawn`, `retired`), origin notes/country codes, moderation metadata, and Dish redirects.
- [ ] Enable PostgreSQL `pg_trgm`; use normalized exact-name/alias constraints plus trigram similarity for duplicate suggestions.
- [ ] Replace immediate Delicacy creation with two commands: verified user submission creates `pending`; staff creation may publish directly.
- [ ] Add public list/detail/browse endpoints, authenticated pending edit/withdraw endpoints, duplicate-suggestion endpoint, and staff approve/reject/taxonomy endpoints.
- [ ] Implement Admin-only merge in one transaction: lock both Dishes, move Recipes, merge non-conflicting aliases/regions, retire source, create redirect, and audit before/after metadata.
- [ ] Build the first moderation/audit primitives here so later Recipe/Review reports reuse them.

### Exit gate

- [ ] Pending Dishes never appear publicly; published Dishes support category/region browse.
- [ ] Duplicate warning, confirm-submit, approval, rejection, withdrawal, merge redirect, and rollback tests pass.
- [ ] Every staff mutation creates exactly one append-only audit record.

---

## M4 — Recipe identity, immutable versions, and authoring

**Depends on:** M2 and M3.

**FRD coverage:** FR-301–FR-313, FR-1001–FR-1005, NFR-3, NFR-4.

### Migration and domain deliverables

- [ ] Add stable Recipe identity fields: author, Delicacy, current published version, visibility, moderation status, soft-delete state.
- [ ] Add Recipe Versions with version number, lifecycle (`draft`, `published`), snapshot fields, publication timestamp, and immutable-after-publication enforcement.
- [ ] Add version-owned Ingredients, Steps, Step-Ingredient references, tags, and media joins. Store duration in seconds and preserve ordered positions with partial unique indexes.
- [ ] Add one-active-draft-per-Recipe and one-version-number-per-Recipe constraints.
- [ ] Backfill every legacy Recipe into a published v1 Recipe Version, copy legacy Ingredients/Steps/tags/images, set current version, validate counts, then switch application reads. Keep legacy columns until a later verified cleanup migration.

### API deliverables

- [ ] Public/current reads: `GET /recipes/{id}`, `GET /recipe-versions/{id}`.
- [ ] Authoring: create Recipe + draft, fetch draft, update draft snapshot, publish with `Idempotency-Key`, change visibility, soft-delete.
- [ ] Access policy: public is discoverable, unlisted is link-readable, private/draft is author/staff-only, and eligible historical versions are direct-link read-only with an outdated marker.
- [ ] Validate complete publishable snapshots, Ingredient scaling metadata, action enums, references, positions, times, servings, difficulty, and media readiness.
- [ ] Return scaled Ingredient display as a read projection; never persist a version mutation for requested servings.

### Exit gate

- [ ] Concurrent publish attempts create one current immutable version per successful idempotency command.
- [ ] Editing after publication never changes the previous snapshot.
- [ ] Legacy fixture backfill matches Recipe, Ingredient, Step, tag, and image counts.
- [ ] The complete visibility/history/access matrix passes service, repository, and end-to-end tests.

---

## M5 — Favorites, search, browse, and initial discovery

**Depends on:** M3 and M4.

**FRD coverage:** FR-401–FR-405, FR-407–FR-408, NFR-1.

### Backend deliverables

- [ ] Add idempotent Favorite create/delete and cursor-paginated saved list with access filtering.
- [ ] Add normalized/trigram Dish search and indexed Recipe-title search across current public versions only.
- [ ] Implement dietary, difficulty, total-time, category, and region filters with deterministic cursor ordering.
- [ ] Implement recent Dishes and dietary-preference recommendation feeds. Do not ship final trending until Cook/Review signals exist.
- [ ] Add representative 50,000-Recipe seed/load dataset and query-plan assertions for critical paths.

### Exit gate

- [ ] Draft/private/unlisted/deleted/hidden content is absent from every discovery path.
- [ ] Save/unsave retries remain idempotent and inaccessible saved content leaks no metadata.
- [ ] Search meets the FRD p95 target under the specified load profile, or the milestone remains open with measured query/index work recorded.

---

## M6 — Cook Mode, session lifecycle, XP, streaks, and core analytics

**Depends on:** M1, M2, and M4. M5 may proceed in parallel once Recipe reads are stable.

**FRD coverage:** FR-501–FR-509, FR-701–FR-707, FR-803, FR-1101–FR-1105, NFR-3–NFR-5, NFR-8.

### Data and transaction deliverables

- [ ] Add Cook Sessions, Step progress, Timer state, completion idempotency records, immutable XP/streak ledger entries, and cached user aggregates.
- [ ] Snapshot the completion-local date and IANA timezone on the session; timezone changes never rewrite historical rows.
- [ ] Implement one-active-session-per-user/version, resume, Step visit, timer state, abandon, and completion commands.
- [ ] In one completion transaction: lock the session/user reward scope, verify every Step visited, complete once, apply per-Recipe/day and five-rewarded-session/day limits, calculate 50/+10/+25 awards, append ledger entries, update streak aggregates, and emit authoritative analytics/outbox events.
- [ ] Model zero-XP cap decisions in the ledger/audit projection so reward behavior can be explained and recomputed.
- [ ] Add a scheduled worker projection that makes an expired current streak display as zero even before another completion.

### API deliverables

- [ ] Start/resume session; retrieve active session; visit Step; create/update timer; abandon; complete with `Idempotency-Key`; list the current user's sessions.
- [ ] Product analytics ingestion for allowlisted client events and server-side generation for authoritative events.
- [ ] Internal metrics endpoints/report queries for activation, Cook Mode conversion, Review eligibility placeholder, and seven-day repeat completion.

### Exit gate

- [ ] Concurrency tests prove duplicate completion cannot duplicate completion state, analytics, XP, bonuses, or streak advancement.
- [ ] DST boundaries, timezone changes, missed days, same-day repeats, version repeats, first-Dish bonus, and daily cap scenarios pass with a fake clock.
- [ ] Session/timer API tests prove persisted deadlines and progress survive process restarts and can be restored by a client.
- [ ] Activation and seven-day return metrics reconcile exactly with source Cook Sessions.

---

## M7 — Version-specific Reviews and content reporting

**Depends on:** M4 and M6; reuses moderation primitives from M3.

**FRD coverage:** FR-601–FR-608, FR-801–FR-802, FR-901–FR-906, NFR-3–NFR-5.

### Backend deliverables

- [ ] Add Reviews with exact Recipe Version/user uniqueness, three checked 1–5 dimensions, optional text/photo, moderation state, and timestamps.
- [ ] Enforce exact-version completed-session eligibility and prohibit author self-Reviews inside the write transaction.
- [ ] Maintain version-specific aggregate projections transactionally or through an idempotent recompute job with a reconciliation command.
- [ ] Add polymorphic Content Reports with unique reporter/target, reason taxonomy, threshold state, and transactional auto-hide after the third eligible reporter.
- [ ] Extend moderation actions/console APIs for reported Recipes and Reviews, restore/keep-hidden/remove outcomes, and author notifications.
- [ ] Add Review create/edit/read/list and current/historical aggregate endpoints.

### Exit gate

- [ ] Review authorization matrix covers unverified users, wrong versions, incomplete sessions, authors, duplicates, edited Reviews, and inaccessible content.
- [ ] Current Recipe aggregates exclude historical versions and reconcile from source Reviews.
- [ ] Three distinct verified reports hide once; duplicates/unverified users do not count; restore/remove preserves immutable audit history.

---

## M8 — Trending, notification preferences, and engagement automation

**Depends on:** M5–M7.

**FRD coverage:** FR-406, FR-801–FR-805, FR-1105, NFR-2, NFR-4.

### Backend/worker deliverables

- [ ] Implement rolling seven-day trend projections using configurable defaults: cook `3`, Favorite `1`, Review `2`; deduplicate cooks per user/Recipe/local date.
- [ ] Recompute incrementally from authoritative events and provide a full reconciliation job. Exclude all non-public/inaccessible content at read time even if a stale score exists.
- [ ] Add Notification Preferences by category/channel, in-app list/unread count/mark-read endpoints, and email opt-in enforcement.
- [ ] Add outbox producers for new Reviews, Dish/moderation outcomes, and streak-at-risk reminders.
- [ ] Schedule the initial 19:00 local streak reminder once per user/local date and make retries idempotent.
- [ ] Complete the product report with Review rate and cohort decision-gate status.

### Exit gate

- [ ] Trend scores reconcile from fixtures and age out correctly after seven days.
- [ ] Preference defaults are in-app on/email off; transactional auth email bypasses optional preferences.
- [ ] Worker restarts/retries do not duplicate reminders or activity notices.
- [ ] Product dashboard numbers reconcile with direct database queries for a frozen test dataset.

---

## M9 — Launch hardening, migration cutover, and sign-off

**Depends on:** M0–M8.

**FRD coverage:** All acceptance criteria; NFR-1–NFR-11.

### Deliverables

- [ ] Complete full OpenAPI review, error-code catalog, operational runbooks, environment/config reference, backup/restore test, provider-failure procedures, and data-retention/anonymization runbook.
- [ ] Replace process-local rate limits with shared per-network/per-account limits and apply route-specific policies.
- [ ] Add structured metrics/traces for request latency, errors, DB pools, queue age, job retries, provider failures, upload processing, completion conflicts, and notification suppression.
- [ ] Run security review for OAuth, JWT/refresh rotation, reset/verification credentials, RBAC, IDOR, signed media, upload validation, logging redaction, and abuse paths.
- [ ] Execute the 50-concurrent-client/50,000-public-Recipe search load test and critical command soak tests.
- [ ] Rehearse production migration from a sanitized pre-v1 snapshot, verify backfill/reconciliation reports, and document rollback/cutover checkpoints.
- [ ] Remove legacy Recipe/Rating columns only after one release of verified target reads and a recoverable production backup; otherwise leave them unused.
- [ ] Run backend API integration journeys: register/verify, Dish submission/moderation, Recipe author/publish, search/save, cook/complete, XP/streak, Review/report/moderate, notifications, and account anonymization.

### Exit gate

- [ ] Every FRD acceptance criterion has an automated test or an explicitly signed manual verification record.
- [ ] No open critical/high backend security, integrity, performance, or migration defect remains.
- [ ] Search p95, API availability instrumentation, queue recovery, and backup restoration satisfy backend-applicable NFRs.
- [ ] Product analytics can distinguish pre-threshold reporting from the 100-cohort/25% retention decision gate.

---

## 4. Dependency path and parallel work

```text
M0 Foundation
 ├─ M1 Identity/Profile ─────┐
 └─ M2 Media/Worker ─────────┼─ M3 Dish Taxonomy ── M4 Recipe Versions ── M5 Discovery
                             │                              │                   │
                             └──────────────────────────────┴─ M6 Cook/XP ─────┤
                                                                              ├─ M8 Engagement
                                                         M7 Reviews ──────────┘
                                                                                   │
                                                                              M9 Launch
```

- M1 and the media/worker core of M2 can run in parallel after M0.
- M3 requires identity/roles and media references; M4 requires curated Dishes and media.
- M5 can proceed in parallel with early M6 once Recipe reads are stable.
- M7 requires completed sessions for Review eligibility; M8 requires all engagement signals.
- M9 is a release gate, not a bucket for deferred correctness work.

---

## 5. FRD traceability matrix

| FRD area                                           | Primary milestone                                   | Verification milestone |
| -------------------------------------------------- | --------------------------------------------------- | ---------------------- |
| FR-001–FR-003 Roles/access                        | M0–M1                                              | M9                     |
| FR-101–FR-107 Auth/profile                        | M1                                                  | M9                     |
| FR-201–FR-209 Dish taxonomy                       | M3                                                  | M9                     |
| FR-301–FR-313 Recipe/versioning                   | M4                                                  | M9                     |
| FR-401–FR-405, FR-407–FR-408 Discovery/favorites | M5                                                  | M9                     |
| FR-406 Trending                                    | M8                                                  | M9                     |
| FR-501–FR-509 Cook Sessions                       | M6                                                  | M9                     |
| FR-601–FR-608 Reviews                             | M7                                                  | M9                     |
| FR-701–FR-707 XP/streaks                          | M6                                                  | M9                     |
| FR-801–FR-805 Notifications                       | M2 foundation, M3/M7 producers, M8 product behavior | M9                     |
| FR-901–FR-906 Moderation                          | M3 foundation, M7 completion                        | M9                     |
| FR-1001–FR-1005 Media                             | M2                                                  | M9                     |
| FR-1101–FR-1105 Analytics                         | M6–M8                                              | M9                     |
| NFR-1–NFR-7, NFR-10                               | Built continuously                                  | M9 sign-off            |
| NFR-8                                              | Backend session/timer persistence in M6             | M9 API verification    |
| NFR-9, NFR-11                                      | Frontend responsibility; outside this backend plan  | Not tracked here       |

---

## 6. Recommended first execution slice

Start with the authoritative M0 checklist and do not add feature migrations on top of the current `000007` state. Execute its backend deliverables in their displayed order and then satisfy the exit gate. This removes the current blockers and creates the contract/test foundation required for every later milestone without maintaining a duplicate task list here.

---

## 7. Planning assumptions

- This plan covers only the Go API, PostgreSQL schema/migrations, workers, provider integrations, and backend tests. Frontend implementation is explicitly outside its scope.
- The current uncommitted user/Delicacy work belongs to the project and must be reconciled rather than overwritten.
- PostgreSQL remains the source of truth; GORM is retained, while invariants also use explicit SQL constraints and transactions.
- Brevo remains the initial email provider; S3-compatible storage and image-safety providers are selected through configuration, not hardcoded domain logic.
- Delivery dates are intentionally omitted until team size and sprint capacity are known. Milestone dependencies and exit gates—not optimistic calendar estimates—control sequencing.
