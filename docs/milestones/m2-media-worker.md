# M2 build notes — media pipeline and durable workers

## Product outcome

Cooked can now safely accept the photos that make cooking guidance useful: profile avatars today, and Dish, Recipe, Step, Review, and Cook Session images as those product areas land. Uploads do not become public merely because an object exists. They remain quarantined until a worker decodes, validates, normalizes, resizes, and approves them.

Email work is also durable. Registration, verification, and password-reset requests commit notification intent to PostgreSQL, while a separately supervised worker performs delivery. API latency and process restarts no longer determine whether a message survives.

## Implementation approach

- Added migration v9 for media assets, responsive variants, job leases, notification delivery attempts, retry metadata, and provider idempotency keys.
- Added short-lived S3-compatible signed upload and download URLs behind an object-store interface.
- Enforced a 5 MB limit, JPEG/PNG decoding, declared-versus-actual size and MIME checks, dimension/pixel limits, metadata stripping through re-encoding, and 256/1024-pixel variants.
- Used PostgreSQL `FOR UPDATE SKIP LOCKED` claims, stale-lease recovery, bounded exponential backoff, and five-attempt terminal failure handling.
- Kept pending, rejected, failed, private, and deleted-owner media from public delivery; private media defaults to owner-only access.
- Passed a stable notification ID to Brevo as its `idempotencyKey`, so replay after a crash does not create another provider send.
- Replaced the API's process-local email dispatcher with a persistence-only outbox writer and made `cmd/worker` a real independently supervised process.
- Added a hardened systemd worker unit and operating instructions; no Docker Compose dependency was introduced.

## Verification record

- `go test ./...`
- `go vet ./...`
- `go test -race ./internal/media ./internal/notify`
- Empty/v6 PostgreSQL migration lifecycle through v9, rollback/reapply, isolated lease contention, and deleted-owner cleanup.
- Unit coverage for public/private/quarantined access, oversize rejection, MIME spoofing, processing failure, responsive variants, avatar ownership/readiness, notification replay, and media worker restart.
