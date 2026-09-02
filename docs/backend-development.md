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
