package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

//go:embed migrations/*sql
var migrationsFS embed.FS

func Migrate(database *gorm.DB) error {
	return apply(database, "run migrations", func(m *migrate.Migrate) error {
		return m.Up()
	})
}

func newMigrator(database *gorm.DB) (*migrate.Migrate, error) {
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sqlDB: %w", err)
	}

	if err := ensurePgcrypto(sqlDB); err != nil {
		return nil, fmt.Errorf("ensure pgcrypto: %w", err)
	}
	if err := ensurePgTrgm(sqlDB); err != nil {
		return nil, fmt.Errorf("ensure pg_trgm: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("init migration source: %w", err)
	}
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("init postgres driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("init migrate: %w", err)
	}

	return m, nil
}

func apply(database *gorm.DB, op string, fn func(*migrate.Migrate) error) error {
	m, err := newMigrator(database)
	if err != nil {
		return err
	}
	if err := fn(m); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func MigrateUp(database *gorm.DB) error {
	return Migrate(database)
}

func MigrateDown(database *gorm.DB) error {
	return apply(database, "migrate down", func(m *migrate.Migrate) error {
		return m.Down()
	})
}

func MigrateSteps(database *gorm.DB, n int) error {
	return apply(database, fmt.Sprintf("migrate steps %d", n), func(m *migrate.Migrate) error {
		return m.Steps(n)
	})
}

func MigrateToVersion(database *gorm.DB, version uint) error {
	return apply(database, fmt.Sprintf("migrate to version %d", version), func(m *migrate.Migrate) error {
		return m.Migrate(version)
	})
}

func ForceVersion(database *gorm.DB, version int) error {
	return apply(database, fmt.Sprintf("force version %d", version), func(m *migrate.Migrate) error {
		return m.Force(version)
	})
}

func CurrentVersion(database *gorm.DB) (uint, bool, error) {
	m, err := newMigrator(database)
	if err != nil {
		return 0, false, err
	}
	version, dirty, err := m.Version()
	if err != nil {
		return 0, dirty, fmt.Errorf("get migration version: %w", err)
	}
	return version, dirty, nil
}

func ensurePgcrypto(db *sql.DB) error {
	_, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	return err
}
func ensurePgTrgm(db *sql.DB) error {
	// Extensions are database-global while migration tests use concurrent schemas.
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(context.Background(), `SELECT pg_advisory_lock(19032026)`); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(19032026)`)
	_, err = conn.ExecContext(context.Background(), `CREATE EXTENSION IF NOT EXISTS pg_trgm`)
	return err
}
