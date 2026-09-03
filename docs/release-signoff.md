# M9 backend release sign-off

Date: 3 September 2026. Scope: backend-applicable FRD and NFR acceptance through migration 16.

## Automated evidence

- Migration lifecycle: v6 sanitized legacy fixture → latest, latest rollback/reapply, full rollback, and empty → latest.
- Integration journeys cover identity/profile/anonymization, RBAC, Dish moderation/merge, Recipe publication/version history, search/favorites, Cook completion/rewards/streaks, Reviews/report threshold/moderation, notification preferences/retries, and engagement projections.
- Concurrency tests cover publication and Cook completion idempotency; notification leases/provider keys and report/review command keys cover retry safety.
- The recorded M5 load profile ran 50 clients against 50,000 Recipes for five minutes: 563,687 requests, zero failures, 57.32 ms p95.
- Shared rate-limit and route-template observability tests are part of migration/unit CI.
- Local logical-backup rehearsal: the v13 development snapshot was dumped, checksum `c8afd8b373775d10102b0bc48332b55b73b75a8a5cbd92b759fc7e42a9ad294c` recorded, restored into an isolated schema, and its clean migration marker plus User, Recipe Version, and Cook Session tables verified. The isolated schema and plaintext artifact were removed automatically. The separate migration lifecycle proves the sanitized v6 fixture upgrades through v16.
- Critical-command soak: concurrent Recipe publication, concurrent Cook completion, Review idempotency/eligibility, and report-threshold/moderation integration tests each passed five consecutive isolated-schema runs.

## Manual/external verification record

- Backend security review: [security-review.md](security-review.md).
- Deployment, rollback, backup/restore, queue/provider, and retention procedures: [operations.md](operations.md).
- Environment contract: [environment.md](environment.md); API errors: [error-codes.md](error-codes.md).
- PWA-only WCAG 2.2 AA, browser compatibility, local timer notifications, and UI wording remain outside this backend repository and must be signed by the web release owner.
- Production backup restoration, TLS/reverse-proxy policy, provider credentials, alert delivery, and 99.5% availability measurement require deployment-environment evidence before public traffic.

## Decision

Backend release candidate accepted: no known critical/high backend security, integrity, performance, or migration defect. Public production cutover remains conditional on the external controls above and a recorded recoverable production-backup rehearsal. Product analytics explicitly reports whether 100 matured cohorts exist before evaluating the 25% retention target.
