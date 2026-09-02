package db

import (
	"fmt"
	"time"

	"github.com/wizzyszn/cooked/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.DatabaseConfig, env string) (*gorm.DB, error) {
	logLevel := logger.Info
	if env == "developement" {
		logLevel = logger.Silent
	}
	dialector := postgres.Open(cfg.DSN())
	database, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	sqlDB, err := database.DB()

	if err != nil {
		return nil, fmt.Errorf("getting underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConnection)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConnection)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxConnectionLifeTime) * time.Hour)

	return database, nil
}
