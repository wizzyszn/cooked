# Backend development and migrations

## Local PostgreSQL

Cooked uses the system PostgreSQL service rather than a project Docker container.

```bash
sudo systemctl start postgresql.service
pg_isready -h localhost -p 5432
```

Application connection values come from `.env`. Do not commit that file or print its secrets in test output.

## Migrations

Migrations are an explicit deployment step and are never run automatically by `cmd/api`:

```bash
go run ./cmd/migrate version
go run ./cmd/migrate up
go run ./cmd/api
```

Production deployment must stop before API startup if migration execution fails or reports a dirty schema.

## Migration integration test

The lifecycle test requires `COOKED_TEST_DATABASE_URL`. A database ending in `_test` is accepted automatically. To use a shared development database, set `COOKED_TEST_ALLOW_SHARED_DATABASE=1`; the test then creates a cryptographically unique `cooked_migration_<uuid>` schema and drops only that schema during cleanup.

```bash
COOKED_TEST_DATABASE_URL='postgres://user:pass@localhost:5432/cooked_test?sslmode=disable' \
go test ./internal/db -run TestMigrationLifecycle -v
```

The test verifies empty schema → v6 fixture → latest, latest → previous version → latest, full rollback, and empty schema → latest. It also verifies registered-role creation and M1 preference/anonymization integrity.

## Initial administrator

After the target account has registered, grant the first administrator role with an explicit audit reason:

```bash
go run ./cmd/admin bootstrap --email admin@example.com --reason "initial production administrator"
```

## Google sign-in

Google sign-in is disabled unless every `GOOGLE_OAUTH_*` value in `.env.example` is configured. Return URLs are exact-match allow-listed. The callback redirects with a two-minute, single-use Cooked exchange code; it never places Cooked access or refresh tokens in a URL.

## Media storage and worker

M2 uses any S3-compatible object store. Configure the `S3_*` values from `.env.example`, create the private bucket named by `S3_BUCKET`, and keep anonymous bucket access disabled. Upload and download access is granted through short-lived signed URLs.

The API only persists notification intent and media-upload state. Delivery, image validation, metadata stripping, responsive variants, retries, quarantine, and orphan cleanup run in the separate worker:

```bash
go run ./cmd/worker
```

The documented v1 image-safety heuristic rejects malformed files, MIME spoofing, files over 5 MB, dimensions over 12,000 pixels, and decompression-bomb candidates over 40 megapixels. Every accepted image is decoded and re-encoded to strip metadata before public use. Semantic unsafe-content detection can be added behind the processor boundary when a provider is selected; until then, pending and rejected assets remain quarantined and never receive a public URL.

For a systemd deployment, install `deploy/systemd/cooked-worker.service` (shown below) and the equivalent API unit. Both read the same protected environment file; migrations remain a separate pre-start deployment command.

```ini
[Unit]
Description=Cooked background worker
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=cooked
WorkingDirectory=/opt/cooked
EnvironmentFile=/etc/cooked/cooked.env
ExecStart=/opt/cooked/bin/cooked-worker
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

After installing or changing the unit:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cooked-worker.service
sudo systemctl status cooked-worker.service
```

## Launch operations

Production configuration, error contracts, security evidence, migration/cutover checkpoints, backup restoration, provider incidents, and release conditions are maintained in [environment.md](environment.md), [error-codes.md](error-codes.md), [security-review.md](security-review.md), [operations.md](operations.md), and [release-signoff.md](release-signoff.md).

## Swagger UI

With the API running, open `http://localhost:8080/docs/`. The UI loads the exact OpenAPI contract embedded in the binary from `/docs/openapi.yaml`, supports bearer-token authorization, and enables “Try it out” requests against `/api/v1`. Documentation, health, and metrics routes do not consume the global API rate-limit budget; requests sent from the UI still do.
