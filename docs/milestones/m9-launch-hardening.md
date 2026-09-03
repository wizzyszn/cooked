# M9 — Launch hardening, migration cutover, and sign-off

## Delivered

- Shared PostgreSQL rate limits for global network traffic and route-specific network/account abuse policies.
- Route-template HTTP latency/error counters plus DB pool, queue age, retry, provider failure, media processing, and notification suppression metrics at `/metrics`.
- Migration `000016` and additive cutover policy; legacy Recipe/Rating storage remains unused and retained until production backup and one-release observation gates allow removal.
- Reviewed OpenAPI contract, API error catalog, environment reference, deployment/rollback, backup/restore, provider incident, retention/anonymization, and security runbooks.
- Consolidated automated and external/manual acceptance evidence with explicit production-cutover conditions.

## Verification

- Full Go suite, static analysis, race tests, OpenAPI validation, and migration up/down/reapply lifecycle.
- Shared limiter enforcement and metrics route-template privacy tests.
- A real v13 logical backup was checksummed, restored into an isolated local schema, verified, and automatically removed; the sanitized v6 migration fixture upgrades through v16 independently.
- Existing 50-client/50,000-Recipe search profile remains the launch performance record: 57.32 ms p95 and zero failures across 563,687 requests.
- Critical publication, completion, Review/report, notification, migration, and anonymization integration journeys run against isolated PostgreSQL schemas.
- Concurrent publication/completion and Review/report critical-command suites passed five consecutive database-backed soak runs.

## Release posture

The backend release candidate has no known critical/high defect. Production traffic is conditional on a recoverable production-backup rehearsal, TLS/reverse-proxy and private-metrics controls, provider credentials/alerts, and separate web-owner accessibility and browser-compatibility sign-off.
