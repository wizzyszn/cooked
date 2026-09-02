# Cooked — Functional Requirements Document (v1)

**Document owner:** Olubayo Wisdom (Reslve)

**Status:** Draft for stakeholder and engineering sign-off

**Last updated:** 2026-09-02

**Product surface:** Mobile-first responsive web application / PWA

---

## 1. Purpose

Cooked is a recipe curation, discovery, and guided-cooking platform. The v1 release exists to validate one core retention loop:

> Discover or save a Recipe → enter Cook Mode → complete the cook → optionally review it → return and cook again.

An **activated user** is a registered user who completes their first Cook Session. The primary v1 success criterion is that at least **25% of activated users complete another qualifying Cook Session within seven days**. This criterion is evaluated once at least 100 users have completed their seven-day activation window; the metric shall still be reported before that threshold.

### 1.1 In scope

- Email/password and Google authentication, profiles, preferences, and staff roles.
- A curated Dish taxonomy with moderated user submissions.
- Recipe drafts, immutable published versions, structured ingredients and Steps.
- Search, browse, dietary filtering, favorites, and activity-based trending.
- Resumable Cook Mode, user-started timers, Cook Sessions, XP, and daily streaks.
- Version-specific structured Reviews.
- Image uploads for Dishes, Recipes, Steps, Reviews, and Cook Sessions.
- Email and in-app notifications.
- Reporting, moderation, audit history, and an internal staff console.
- First-party product analytics needed to measure the v1 retention hypothesis.

### 1.2 Explicitly deferred

- Recipe forking.
- Badges, leaderboards, and streak-freeze tokens.
- Social follows, feeds, and live cook-alongs.
- Pantry-aware recommendations, skill trees, and ML recommendations.
- Push notifications and native mobile applications.
- Offline Cook Mode, video Steps, monetization, and localization.

Existing database structures for deferred features do not make those features part of v1.

---

## 2. Terminology

| Term | Definition |
| --- | --- |
| Dish | The user-facing name for a canonical food identity, such as “Jollof Rice.” |
| Delicacy | The internal/API name for the Dish entity, retained for backend compatibility. |
| Recipe | A stable authored identity belonging to one Dish and one author. |
| Recipe Version | A complete snapshot of a Recipe's cookable content. A published version is immutable. |
| Step | An ordered, structured unit of instructions within a Recipe Version. |
| Cook Session | A version-bound record of a user's cooking attempt. |
| Qualifying completion | An idempotently completed Cook Session that satisfies the Step-visit requirement and is eligible for streak processing. |
| Review | A user's version-specific assessment after completing that exact Recipe Version. |
| Technique | Metadata describing a cooking action; it does not affect a v1 skill score. |

---

## 3. Roles and Access

| Role | Capabilities |
| --- | --- |
| Guest | Browse and search public Dishes and public Recipes; view unlisted Recipes when given a link. |
| Registered User | Manage their profile and preferences, save Recipes, create drafts, use Cook Mode, and earn XP/streaks. Verified users may publish, review, report content, and submit Dishes. |
| Moderator | Perform Registered User actions; review pending Dishes and reports; hide, restore, or remove content; curate taxonomy. |
| Admin | Perform Moderator actions; merge Dishes, manage roles and taxonomies, manage users, and inspect all audit records. |

- **FR-001:** Roles shall be persisted and enforced server-side on every protected operation.
- **FR-002:** Only an Admin may grant or revoke Moderator or Admin roles. The initial Admin shall be provisioned through an audited operational process.
- **FR-003:** Public profile and content endpoints shall never expose email address, timezone, notification settings, dietary preferences, authentication data, or moderation-only data.

---

## 4. Functional Requirements

### 4.1 Authentication and profile

- **FR-101:** The system shall support registration and login by email/password and Google OAuth.
- **FR-102:** Email/password accounts shall require email verification before the user can publish a Recipe, submit a Dish, submit a Review, or report content. Browsing, saving, drafting, and Cook Mode remain available before verification.
- **FR-103:** Password reset shall use a single-use emailed token or code that expires after 30 minutes. Successful use shall invalidate all other outstanding reset credentials for the account.
- **FR-104:** A profile shall contain display name, username, avatar, optional bio, joined date, authored public Recipe count, cumulative XP, current streak, and longest streak.
- **FR-105:** Public profiles shall display the fields in FR-104 except authentication data and private settings. Email address, IANA timezone, dietary preferences, and notification preferences shall remain private.
- **FR-106:** A user may select zero or more curated dietary preference tags. Selecting `none` shall clear and remain mutually exclusive with all dietary tags. Preferences affect filtering and recommendations only and are not a food-safety guarantee.
- **FR-107:** Account deletion shall deactivate access, erase or irreversibly anonymize personal profile data, and reattribute retained public Recipes and Reviews to a system “Deleted user” identity. Cook Sessions and immutable audit/XP records shall be retained only as required for integrity and shall no longer expose the former identity.

### 4.2 Dish taxonomy

- **FR-201:** A Delicacy shall contain a canonical name, aliases, description, cover image, one curated category, one or more curated regions, optional ISO country codes, and optional free-text origin notes.
- **FR-202:** Admins and Moderators may create and publish Dishes directly. A verified Registered User may submit a Dish in `pending` status; it shall not be publicly discoverable before approval.
- **FR-203:** A submitter may edit or withdraw their pending Dish. After approval, the canonical Dish is staff-managed; approval does not give the submitter permanent taxonomy-edit rights.
- **FR-204:** Before submission, the system shall perform case-insensitive and typo-tolerant matching against live Dish names and aliases, show likely duplicates, and require explicit confirmation before a similar submission can proceed.
- **FR-205:** Moderators may approve or reject pending Dishes with a required reason. The submitter shall receive an in-app notification and, if enabled, an email notification of the outcome.
- **FR-206:** Each public Dish page shall show its public Recipes and support sorting by current-version rating, recency, and qualifying cook count.
- **FR-207:** Admins may merge duplicate Dishes. A merge shall transactionally reassign child Recipes, preserve redirects from the retired Dish identifier, consolidate non-conflicting aliases, and write an audit record.
- **FR-208:** Admins shall manage category, region, dietary, and measurement-unit taxonomies through the internal console. Referenced taxonomy values shall be retired rather than destructively deleted.
- **FR-209:** v1 shall not maintain a canonical Dish-level ingredient list; ingredients belong to Recipe Versions.

### 4.3 Recipe authoring and versioning

- **FR-301:** A Recipe shall belong to exactly one Delicacy and one author. The Recipe is a stable identity and points to at most one current published Recipe Version and at most one working draft.
- **FR-302:** A Recipe draft shall be mutable and visible only to its author and authorized staff. Publishing shall atomically create/promote a complete immutable Recipe Version without exposing a partially updated Recipe.
- **FR-303:** A published Recipe shall use one visibility value:
  - `public`: available to guests, indexed by Cooked search, and shown in discovery;
  - `unlisted`: available to anyone with its link but excluded from search, browse, and trending;
  - `private`: available only to its author and authorized staff.
- **FR-304:** Changing visibility shall update Recipe access metadata and shall not mutate the contents of a published Recipe Version. `draft`, `public`, `unlisted`, and `private` form the four user-visible Recipe states.
- **FR-305:** A Recipe Version shall snapshot title, summary, base servings, prep time, cook time, difficulty (`easy`, `medium`, or `hard`), cover images, dietary and general tags, ordered ingredients, ordered Steps, and optional notes/variations.
- **FR-306:** An Ingredient entry shall contain a name or ingredient-catalog reference, optional numeric quantity, optional curated unit, optional display amount text, position, and optional substitution note. Display text supports non-scalable values such as “to taste.”
- **FR-307:** Serving scaling shall multiply numeric quantities by `requested_servings / base_servings` and use unit-aware display rounding. Display-only amounts shall remain unchanged and be labelled non-scalable.
- **FR-308:** A Step shall contain a stable version-local identifier, position, short title, instruction content, action, optional duration in seconds, zero or more technique tags, zero or more Ingredient-entry references, and zero or more images.
- **FR-309:** The initial Step action values shall be `sauté`, `boil`, `simmer`, `fry`, `bake`, `grill`, `fold`, `whisk`, `chop`, `marinate`, `rest`, and `other`. Technique tags remain curated metadata and do not produce skill scores in v1.
- **FR-310:** Editing a published Recipe shall create or update a separate draft based on the current version. Publishing the draft shall create a new immutable version and atomically make it current; the previously published version shall remain unchanged.
- **FR-311:** A previously public or unlisted version shall remain readable through its direct version URL with an “outdated version” label unless the Recipe is deleted or the version is hidden by moderation. Private-version history remains available only to the author and authorized staff.
- **FR-312:** Soft-deleting a Recipe shall remove it and all versions from ordinary access and discovery while retaining the records required by Cook Session, Review, moderation, and XP integrity.
- **FR-313:** A verified author may publish without pre-approval. Published Recipes remain subject to reporting and retrospective moderation.

### 4.4 Favorites, search, and discovery

- **FR-401:** An authenticated user shall be able to idempotently save or unsave an accessible Recipe and view a paginated collection of saved Recipes. A saved Recipe that later becomes inaccessible shall not disclose its content.
- **FR-402:** Search shall cover public Dish names/aliases and current public Recipe titles. Results shall be paginated and typo-tolerant for Dish names and aliases.
- **FR-403:** Recipe results shall be filterable by dietary tag, difficulty, maximum total time, category, and region. Total time is `prep_time + cook_time`; a Recipe lacking either value shall not match a maximum-time filter.
- **FR-404:** Browse shall group public Dishes by curated category and region and allow the user to move between those groupings.
- **FR-405:** The homepage shall show recently approved Dishes, trending public Recipes, and—when authenticated—public Recipes matching any of the user's dietary preferences.
- **FR-406:** Trending shall use events from a rolling seven-day window. The initial score is `3 × unique qualifying cooks + 1 × new favorites + 2 × new Reviews`. A cook is deduplicated per user/Recipe/local date for this calculation. Weights and window length shall be server-configurable.
- **FR-407:** Draft, private, unlisted, soft-deleted, and moderation-hidden content shall never appear in search, browse, preference recommendations, or trending.
- **FR-408:** Dietary tags are author-selected from a curated list and shall be presented as informational labels, not allergen or religious-compliance certification.

### 4.5 Cook Mode and Cook Sessions

- **FR-501:** Entering Cook Mode shall create or resume one `in_progress` Cook Session bound to the exact Recipe Version being viewed. Repeated entry shall not create duplicate active sessions for the same user and version.
- **FR-502:** A Cook Session shall have `in_progress`, `completed`, or `abandoned` status, with start, last-activity, completion, and abandonment timestamps as applicable.
- **FR-503:** Cook Mode shall present the bound version's Steps sequentially, record which Steps have been visited, persist progress, and allow the user to resume on another request or after refreshing the PWA.
- **FR-504:** A timed Step shall display its suggested duration but shall not start automatically. The user may explicitly start, pause, resume, or reset its countdown.
- **FR-505:** Timer state shall be based on persisted target timestamps rather than an assumed continuously running browser process. When browser permission and platform support are available, the PWA shall issue a local notification when a backgrounded timer ends; otherwise it shall restore the correct remaining/elapsed state when reopened.
- **FR-506:** The user may mark a session complete only after visiting every Step and explicitly selecting “I finished cooking.” Finishing a timer or merely reaching the last Step shall not complete the session.
- **FR-507:** Completion shall be transactional and idempotent. Concurrent or repeated completion requests shall produce one completion, one streak evaluation, and no duplicate XP ledger entries.
- **FR-508:** Completion shall prompt for an optional dish photo that may be attached as part of the completion request. Neither a photo nor waiting for every timer is required for completion.
- **FR-509:** A user may explicitly abandon an in-progress session. An abandoned session shall not qualify for XP, streaks, reviews, or trending.

### 4.6 Reviews

- **FR-601:** A verified user may review a Recipe Version only after completing a Cook Session for that exact version.
- **FR-602:** An author shall not review their own Recipe. Authors may complete their own Recipes and receive otherwise-valid XP.
- **FR-603:** A Review shall contain a taste rating, instruction-clarity rating, and difficulty-accuracy rating, each from 1 to 5, plus an optional comment and optional photo.
- **FR-604:** For difficulty accuracy, `1` means the stated difficulty was very inaccurate and `5` means it matched the user's experience very accurately. It does not independently mean easy or hard.
- **FR-605:** A user may create at most one Review per Recipe Version. The Review may be edited but shall remain bound to its original user and version.
- **FR-606:** Current Recipe list/detail views shall show arithmetic mean taste, clarity, and difficulty-accuracy scores plus Review count for the current published version only.
- **FR-607:** Historical-version pages shall show Reviews and aggregates for that version. Reviews of older versions shall not be silently combined into the current version's aggregates.
- **FR-608:** Review creation/editing and aggregate updates shall be transactionally consistent or recoverably recomputed from Review records.

### 4.7 XP and streaks

- **FR-701:** A qualifying completion shall award a configurable base of 50 XP, a configurable 10 XP photo bonus, and a configurable one-time 25 XP bonus when the user first completes any Recipe Version belonging to that Dish.
- **FR-702:** Base and photo XP may be awarded at most once per user/Recipe/local date. Completing another version of the same Recipe on that date shall not bypass the limit.
- **FR-703:** At most five Cook Sessions per user/local date may award XP. Later valid completions shall still be recorded and may support Review eligibility but shall award zero XP.
- **FR-704:** The first qualifying completion on a local calendar date shall advance the daily streak. Additional completions on the same date shall not advance it again.
- **FR-705:** A streak shall be the number of consecutive local dates containing at least one qualifying completion. If the date following the last qualifying date passes without one, the current streak becomes zero; a later completion begins a new streak of one.
- **FR-706:** Streak dates shall use the user's stored IANA timezone. A timezone change affects only future session completions and shall not recalculate historical XP or streak dates.
- **FR-707:** XP grants, bonus decisions, cap decisions, and streak mutations shall be represented by immutable, idempotency-keyed ledger entries. Profile totals may be cached but must be reproducible from source records and the ledger.

### 4.8 Notifications

- **FR-801:** The system shall support in-app and email notifications for a new Review on an authored Recipe, Dish-submission and moderation outcomes, and streak-at-risk reminders.
- **FR-802:** In-app categories shall default to enabled. Optional activity and streak emails shall default to disabled and require explicit opt-in. Users may toggle each non-transactional category independently by channel.
- **FR-803:** Streak-at-risk evaluation shall use the user's IANA timezone and a server-configurable evening threshold, initially 19:00 local time. It shall notify only users with an active streak who have not completed a qualifying session that day.
- **FR-804:** Verification, password-reset, security, and other necessary transactional messages are not controlled by activity-notification preferences.
- **FR-805:** Notification creation and delivery attempts shall be recorded separately so retries do not create duplicate in-app notifications or duplicate provider sends.

### 4.9 Reporting, moderation, and administration

- **FR-901:** A verified user may report an accessible Recipe or Review by selecting a reason and optionally adding details. A user may report a target only once.
- **FR-902:** Reports from three distinct verified users shall automatically hide the target pending moderation. Duplicate reports, reports from unverified accounts, and reports made after the target is hidden shall not increase the threshold count.
- **FR-903:** A Moderator may dismiss reports and restore content, keep content hidden, or remove it. A reason is required for every decision, and affected authors shall be notified.
- **FR-904:** The internal web console shall provide queues and detail views for pending Dishes, reported/auto-hidden Recipes and Reviews, Dish merges, taxonomy management, role management, user lookup, and moderation history.
- **FR-905:** Every staff action shall record actor, action, target, timestamp, reason, and relevant before/after metadata in an append-only audit log.
- **FR-906:** Moderation removal shall override historical direct-link access. Restoring content shall restore access according to its prior visibility without altering immutable version content.

### 4.10 Media

- **FR-1001:** Dish covers, Recipe-Version covers, Step images, Review photos, and Cook Session photos shall be stored in S3-compatible object storage and served through a CDN.
- **FR-1002:** Each upload shall be at most 5 MB and shall be validated by decoded MIME type, dimensions, and supported format rather than filename alone.
- **FR-1003:** Images shall be stripped of unnecessary metadata, resized server-side into configured responsive variants, and served with appropriate cache and access controls.
- **FR-1004:** Uploads shall pass a third-party image-safety service or documented v1 heuristic before public display. Failed or pending moderation shall not expose the original publicly.
- **FR-1005:** Private Recipe media shall use authorization-aware or expiring access URLs; possession of a permanent public object URL shall not bypass Recipe visibility.

### 4.11 Product analytics

- **FR-1101:** The system shall record a pseudonymous, versioned first-party analytics event schema covering search/browse impressions, Recipe views, favorites, Cook Mode entry, Step visits, timer use, session completion/abandonment, Review submission, and return completion.
- **FR-1102:** Server-authoritative events shall be used for activation, qualifying completion, XP, streak, Review, and retention calculations. Client events may describe funnel behavior but shall not grant rewards or permissions.
- **FR-1103:** Analytics shall support funnels and seven-day activation cohorts without placing email address, free-text Review content, or other unnecessary personal data in event properties.
- **FR-1104:** Analytics collection shall respect applicable consent choices and a documented retention/deletion policy. Account anonymization shall sever analytics records from the former public identity where technically required.
- **FR-1105:** A product report shall continuously display activation count, Cook Mode-to-completion conversion, Review rate after completion, and seven-day repeat-cook retention. The 25% retention decision gate applies after 100 completed activation cohorts.

---

## 5. Non-Functional Requirements

- **NFR-1 — Performance:** Search API responses shall be below 300 ms p95 server time during a five-minute steady-state test with 50 concurrent clients, a warm application, and a representative dataset containing at least 50,000 public Recipes. The test report shall state hardware, query mix, and database size.
- **NFR-2 — Availability:** The production API target is 99.5% monthly availability, excluding pre-announced maintenance. Authentication, Recipe access, Cook Session completion, and Review submission are included in the measured service boundary.
- **NFR-3 — Data integrity:** Publishing, session completion, Review eligibility, aggregate updates, XP grants, and Dish merges shall use transactions and database constraints appropriate to their invariants.
- **NFR-4 — Idempotency:** Publish, session-completion, favorite, Review, report, and notification-delivery operations shall tolerate client retries without unintended duplicates.
- **NFR-5 — Auditability:** XP ledger entries and moderation logs are append-only. Corrections shall use compensating entries rather than in-place mutation.
- **NFR-6 — Security:** Credentials, sessions, OAuth state, object access, and RBAC shall follow current OWASP guidance. Passwords shall use an accepted adaptive password hash; secrets and raw reset/refresh credentials shall not be stored in plaintext.
- **NFR-7 — Abuse resistance:** The server shall apply configurable per-account and per-network rate limits to authentication, Dish submission, Recipe publication, Cook completion, Reviews, reports, and media uploads. Limits shall not replace the XP-specific rules in FR-702 and FR-703.
- **NFR-8 — PWA reliability:** Cook Mode shall be mobile-first, recover state after refresh/backgrounding, and never rely on JavaScript remaining active while the device is locked. Browser/platform limitations shall be communicated when notifications are unavailable.
- **NFR-9 — Accessibility:** Public and authenticated user interfaces, including Cook Mode and the internal console, shall target WCAG 2.2 AA for keyboard use, focus, contrast, labels, reduced motion, and screen-reader semantics.
- **NFR-10 — Privacy:** Authorization shall be enforced when reading media and historical versions. Logs and analytics shall minimize personal data and shall never contain passwords, raw authentication tokens, or reset codes.
- **NFR-11 — Compatibility:** The PWA shall support the current and previous major versions of Chrome, Safari, Firefox, and Edge, subject to explicit graceful degradation for browser-notification capabilities.

---

## 6. Target Data Model

The following list defines entities and ownership, not a complete physical schema:

```text
User
Role / UserRole
UserDietaryPreference
NotificationPreference

Delicacy (Dish)
DelicacyAlias
Category
Region / DelicacyRegion
Tag / DelicacyTag / RecipeVersionTag

Recipe (stable identity, author, Delicacy, current version, visibility)
RecipeVersion (immutable after publication)
IngredientCatalogEntry
RecipeVersionIngredient
RecipeVersionStep
StepIngredientReference

Favorite
CookSession / CookSessionStepProgress / TimerState
Review

XPAndStreakLedgerEntry
Notification / NotificationDeliveryAttempt
MediaAsset

ContentReport
ModerationAction
AnalyticsEvent
```

Database constraints shall enforce, at minimum:

- one active draft per Recipe;
- unique version number per Recipe;
- one active Cook Session per user and Recipe Version;
- one Review per user and Recipe Version;
- no author Review of their own Recipe;
- one report per reporter and target;
- ordered Ingredient and Step positions within a Recipe Version;
- unique idempotency keys for completion, XP grants, and notification sends.

---

## 7. Existing Backend Alignment

This FRD is the product source of truth and describes the target state. Implementation planning shall account for these known differences in the current repository:

- Ingredients and Steps currently reference `recipes` directly; target-state records belong to `recipe_versions`.
- Existing `ratings` contain a single score and reference a Recipe; target Reviews contain three dimensions and reference a Recipe Version.
- Existing Recipe difficulty uses three labels and is retained; validation and product wording must be made consistent.
- Existing Recipe visibility values require normalization into the access behavior defined by FR-303 and FR-304, alongside a separate working draft.
- An in-progress expansion introduces a single stored role and a single dietary value. The target requires a Registered User default role, Admin-assigned RBAC, and zero-or-more dietary preferences, so the current model and migration still require normalization.
- Taxonomy records, versioning, Cook Sessions, reports, moderation, media, analytics, notification preferences/delivery attempts, and XP/streak ledger persistence require implementation or migration.
- Existing follows/social-feed structures are inactive and out of v1 scope; they must not influence v1 APIs or acceptance.
- Existing records shall be migrated with deterministic defaults, referential integrity checks, and reversible database migrations before production rollout.

---

## 8. Acceptance Criteria

### 8.1 Publishing and access

- Given an unverified author, when they attempt to publish a draft, the request is rejected without losing the draft.
- Given a published Recipe, when its author edits and republishes it, a new immutable version becomes current and the previous version's content remains unchanged.
- Public Recipes appear in discovery; unlisted Recipes work by link; private Recipes and drafts are author/staff-only.
- A prior public/unlisted version is available by direct link and marked outdated, unless deletion or moderation has overridden access.

### 8.2 Cook Sessions, XP, and streaks

- Entering Cook Mode twice for the same version resumes one active session.
- Completion is rejected until every Step has been visited and the user explicitly confirms completion.
- Retrying a successful completion produces no duplicate session, XP, streak, analytics, or notification effects.
- The first eligible Recipe completion on a local date awards 50 XP, plus applicable 10-point photo and 25-point first-Dish bonuses.
- A second completion of the same Recipe on that local date is recorded but awards no additional base/photo XP.
- After five XP-bearing sessions on one local date, further valid sessions award no XP.
- One or more qualifying completions on consecutive local dates advance the streak once per date; a missed date resets it before the next completion.
- Changing timezone does not rewrite historical session dates, streaks, or ledger entries.

### 8.3 Reviews and moderation

- A user without a completed session for the exact version cannot submit its Review.
- A Recipe author cannot Review any version of their own Recipe.
- A user can edit but cannot create a second Review for the same version; cooking a later version permits a separate Review.
- The current Recipe page excludes older-version Reviews from its aggregates.
- Three reports from distinct verified users hide a target once; duplicate/unverified reports do not advance the threshold.
- Every approval, rejection, hide, restore, removal, merge, taxonomy change, and role change has a required append-only audit record.

### 8.4 Discovery, media, and notifications

- Search tolerates common Dish-name misspellings and consistently applies access, dietary, time, difficulty, category, and region filters.
- Trending applies the configured seven-day weighted formula and deduplicates cooks per user/Recipe/local date.
- Unsupported, unsafe, oversized, or unauthorized image uploads are rejected or quarantined without public exposure.
- Activity email remains off until opted in; in-app activity notifications default on; transactional authentication messages remain deliverable.
- Backgrounded timers recover the correct state, and the UI explains when browser notifications are unavailable.

### 8.5 Product success and quality

- Activation and repeat-cook retention are derived from server-authoritative completed Cook Sessions.
- The product report shows the required funnel and cohort measures and clearly indicates whether 100 completed activation cohorts have been reached.
- Automated integration tests cover transactional publication, completion idempotency, XP limits, version-specific Review authorization, auto-hide thresholds, RBAC, and anonymized account deletion.
- Load, accessibility, compatibility, and security checks demonstrate the NFR targets before production sign-off.

---

## 9. Assumptions and Dependencies

- Product requirements take precedence over the current schema; migrations are expected.
- Published Recipe content is immutable, while its access visibility is mutable metadata.
- Cook completion and XP are self-attested after all Steps have been visited; photos and completed timers are not mandatory.
- Dietary labels are informational and must not be presented as allergen, medical, or religious certification.
- Email delivery, Google OAuth, object storage/CDN, and image-safety checking depend on configured external providers.
- Public contributions survive account deletion only in anonymized form so version, Review, Cook Session, and audit integrity can be preserved.
- v1 is web/PWA-first. Native-only behavior, guaranteed background execution, and push notifications are not acceptance requirements.

---

## 10. v2 Questions

- Pantry entry through manual input, barcode scanning, or receipt scanning.
- Technique taxonomy formalization and its relationship to a future skill tree.
- Whether social follows/feeds materially improve retention after the v1 cohort test.
- Recipe forking and attribution rules.
- Native applications, push notifications, offline Cook Mode, and video bandwidth/storage economics.
- Badges, regional leaderboards, and streak-freeze mechanics after basic XP/streak behavior is validated.
