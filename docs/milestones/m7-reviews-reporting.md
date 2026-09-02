# M7 — Version-specific Reviews and content reporting

## Delivered

- Version-specific Reviews with three checked 1–5 dimensions: taste, instruction clarity, and difficulty accuracy.
- Exact-version completed-Cook-Session eligibility, verified-account enforcement, author self-Review prevention, and one Review per user/version.
- Idempotent Review creation, owner editing, optional processed Review photos, and transactionally recomputed per-version aggregates.
- Current Recipe responses use only their current published version's aggregate; historical versions retain independent Reviews and scores.
- Idempotent polymorphic reports for accessible Recipes and Reviews with a fixed reason taxonomy and one report per reporter/target.
- Transactional auto-hide on the third distinct verified reporter, with reports after hiding excluded from the threshold.
- Staff report queues and required-reason decisions to restore, keep hidden, or remove content.
- Transactional in-app author notifications for new Reviews and moderation outcomes.
- Append-only moderation audits containing actor, action, target, reason, and before/after state.

## API routes

- `POST|GET /api/v1/recipe-versions/{id}/reviews`
- `GET|PATCH /api/v1/reviews/{id}`
- `POST /api/v1/reports`
- `GET /api/v1/staff/reports`
- `POST /api/v1/staff/reports/{id}/decision`

Review creation and reporting require an `Idempotency-Key`. Review submission and content reporting require a verified account. Hidden or removed content follows the same non-disclosure boundary as inaccessible parent Recipes.

## Verification

- Full Go suite and OpenAPI contract validation.
- Migration `000014` lifecycle from the v6 fixture to latest, latest rollback/reapply, full rollback, and empty schema to latest.
- Real-PostgreSQL integration coverage for unverified accounts, author Reviews, wrong-version eligibility, duplicate and idempotent creation, editing, version-isolated aggregates, duplicate reports, the three-reporter auto-hide threshold, post-hide reports, restoration, report resolution, and immutable audit history.

## X thread draft

1/ I’m building Cooked because I don’t know how to cook—and a five-star score rarely tells me whether food tasted good, the instructions made sense, or the recipe’s claimed difficulty was honest.

This milestone makes feedback specific to what someone actually cooked. 🧵

2/ A Review is tied to one exact, immutable recipe version. To write one, I must have completed Cook Mode for that version. I can’t review the author’s newer instructions based on an older cook, and authors can’t review their own work.

3/ Reviews separate taste, instruction clarity, and difficulty accuracy into three 1–5 scores. Difficulty accuracy means “did the stated difficulty match my experience?”—not simply “was this easy?”

4/ Each person gets one Review per version and can edit it later. Aggregate scores update transactionally, while old-version feedback stays with the old version instead of quietly changing the current recipe’s score.

5/ Reporting now covers Recipes and Reviews. One person can report a target once, and only reports from distinct verified accounts count toward the threshold.

The third qualifying report hides the content for staff review in the same database transaction.

6/ Moderators can restore content, keep it hidden, or remove it. Every decision needs a reason, notifies the affected author, and writes an append-only before/after audit record.

7/ I tested the uncomfortable edges too: incomplete cooks, the wrong recipe version, self-Reviews, duplicate retries, hidden content, duplicate reporters, unverified reporters, aggregate reconciliation, and restoration after review.

Cooked now has a feedback loop grounded in real cooking attempts—and a moderation trail designed to be explainable when something goes wrong.
