package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/golang-migrate/migrate/v4/database/postgres"
	"gorm.io/gorm"
)

//go:embed migrations/*sql
var migrationsFS embed.FS

func Migrate(database *gorm.DB) error {
	m, err := newMigrator(database)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func newMigrator(database *gorm.DB) (*migrate.Migrate, error) {
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sqlDB: %w", err)
	}

	ensurePgcrypto(sqlDB)

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

func MigrateUp(database *gorm.DB) error {
	err := Migrate(database)
	if err != nil {
		return err
	}
	return nil

}

func MigrateDown(database *gorm.DB) error {
	m, err := newMigrator(database)

	if err != nil {
		return err
	}

	err = m.Down()
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func MigrateToVersion(database *gorm.DB, version uint) error {
	m, err := newMigrator(database)
	if err != nil {
		return nil
	}

	 err = m.Migrate(version)
	 if err != nil {
		return fmt.Errorf("migrate to version %d: %w",version,err)
	 }
	return nil
}

func ensurePgcrypto(db *sql.DB) error {
	_, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	return err
}
