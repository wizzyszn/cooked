# M8 — Trending, notification preferences, and engagement automation

## Delivered

- Configurable rolling seven-day Recipe trends using weights of 3 per unique qualifying cook, 1 per new Favorite, and 2 per new visible Review.
- Cook signals deduplicated by user, Recipe, and snapshotted local completion date, with an authoritative reconciliation worker that ages expired signals out.
- Public trending reads that reject private, unlisted, deleted, moderation-hidden, or otherwise inaccessible Recipes even if a stale projection remains.
- Activity and streak notification preferences per channel, with in-app enabled and email disabled by default.
- In-app notification list, unread count, and owner-only mark-read operations.
- Preference-aware, idempotent in-app and email notification intent for new Reviews, Dish outcomes, moderation outcomes, and streak-at-risk reminders.
- A timezone-aware reminder worker using the configurable 19:00 local default and one reminder intent per user/channel/local date.
- Product metrics for Review eligibility, Review rate, matured activation cohorts, seven-day retention, and the 100-cohort/25% decision gate.

## API routes

- `GET /api/v1/discovery/trending`
- `GET|PUT /api/v1/users/me/notification-preferences`
- `GET /api/v1/users/me/notifications`
- `POST /api/v1/users/me/notifications/{id}/read`
- `GET /api/v1/staff/metrics/product`

## Verification

- Full Go suite, static analysis, race checks, and OpenAPI validation.
- Migration `000015` upgrade, latest rollback/reapply, full rollback, and clean-schema lifecycle.
- Real-PostgreSQL fixtures verify trend weights, cook deduplication, seven-day expiry, public access filtering, preference defaults, email opt-in, reminder restart idempotency, inbox unread state, and product-report reconciliation.

## X thread draft

1/ I’m building Cooked because I don’t know how to cook—and discovering a recipe is easier when I can see what people are actually cooking now, not just what has accumulated clicks forever.

This milestone adds a time-aware engagement loop. 🧵

2/ Trending now uses the last seven days: 3 points for a unique qualifying cook, 1 for a new save, and 2 for a new Review. Repeating the same Recipe on the same local date does not inflate its cook score.

3/ The score is a projection, never an access-control shortcut. Private, unlisted, deleted, or moderation-hidden Recipes stay out of trending even if an old score exists.

4/ Notifications now have actual preferences. Activity and streak notices are on in-app by default, while email starts off and requires an explicit opt-in for each category.

Transactional account emails remain independent, so turning off activity cannot suppress verification or password-reset messages.

5/ The inbox exposes unread counts and mark-read state. New Reviews, Dish decisions, moderation outcomes, and streak reminders all persist idempotent intent, so a retry or worker restart cannot create a second notice.

6/ At 19:00 in each cook’s own timezone, Cooked can remind someone whose active streak is at risk and who has not completed a qualifying cook that day. The local date is part of the uniqueness key.

7/ The product report now closes the loop: Review rate after eligible completions, seven-day repeat cooking, and a clear decision gate that only activates after 100 matured cohorts and checks the 25% target.

Cooked now has a way to surface momentum, encourage a return without noisy email defaults, and measure whether the habit is genuinely forming.
