# Production operations runbook

## Deploy and migrate

1. Confirm a recoverable encrypted backup and record its object checksum, PostgreSQL version, migration version, and restore rehearsal ID.
2. Drain writes, keep the previous binaries available, and run `go run ./cmd/migrate version`.
3. Run `go run ./cmd/migrate up`; stop immediately on a dirty migration.
4. Run reconciliation/integrity tests, start worker then API, verify `/health/ready` reports migration 16, and inspect `/metrics`.
5. Restore traffic gradually while watching HTTP errors/latency, DB pool saturation, notification/media queue age, retries, and provider failures.

Migration 16 is additive. Legacy Recipe/Rating columns remain intentionally unused because the required one-release observation period and production backup evidence do not yet authorize removal. A rollback may use the previous binary only while its required schema remains present. Never force a dirty version without a reviewed database recovery plan.

## Backup and restore rehearsal

Use the same PostgreSQL major version as production:

```bash
pg_dump --format=custom --no-owner --no-acl "$DATABASE_URL" --file cooked.dump
sha256sum cooked.dump > cooked.dump.sha256
createdb cooked_restore_test
pg_restore --exit-on-error --no-owner --no-acl --dbname "$RESTORE_DATABASE_URL" cooked.dump
COOKED_TEST_DATABASE_URL="$RESTORE_DATABASE_URL" go test ./internal/db -run TestMigrationLifecycle -v
```

Verify row counts for users, published Recipes/versions, completed Cook Sessions, Reviews, XP/audit ledgers, notifications, and media metadata. Verify aggregate/trend reconciliation, sample private-resource denial, and readiness. Destroy the isolated restore database after recording results. A successful schema-only migration test is not a substitute for this data restore.

## Queue and provider incidents

- Rising queue age with no retries: confirm the worker unit, DB connectivity, leases, and clock synchronization.
- The worker prunes `rate_limit_buckets` whose indexed `expires_at` value has passed on every cycle; request correctness does not depend on pruning.
- Email provider failure: leave intent queued, validate Brevo credentials/status, and restart the worker. Stable notification IDs preserve provider idempotency.
- Object provider failure: uploads stay private/quarantined. Restore S3 access before retrying; never make the bucket public.
- Poison jobs stop after five attempts. Inspect `last_error`, correct the cause, then reschedule only the exact reviewed rows.
- Alert on sustained 5xx, p95 latency above 300 ms for search, DB pool exhaustion, queue-age growth, provider failures, media failures, and notification suppression anomalies.

## Data retention and anonymization

Account deletion deactivates access, revokes sessions, erases profile identifiers, and reattributes retained public contributions to the Deleted user identity. Cook Sessions, immutable Recipe versions, Reviews, reward ledgers, and moderation audits remain only for integrity and no longer expose the former public identity. Analytics user links are severed. Expired OAuth flows, login/reset credentials, rate-limit buckets, abandoned uploads, and operational logs are periodically purged under the production retention schedule. Backups expire through storage lifecycle rules and are not selectively rewritten.
