package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("initialize configurations: %v", err)
	}

	zapLogger := logger.Init(cfg.Server.Env)
	defer zapLogger.Sync()

	zapLogger.Info("Connecting to Database....")
	database, err := db.Connect(&cfg.Database, cfg.Server.Env)
	if err != nil {
		zapLogger.Fatalw("Failed to connect to database.", "error", err)
	}
	if sqlDB, err := database.DB(); err == nil {
		defer sqlDB.Close()
	}

	if err := run(database, zapLogger, os.Args[1:]); err != nil {
		zapLogger.Fatalw("migration failed", "error", err)
	}
}

func run(database *gorm.DB, zapLogger *zap.SugaredLogger, args []string) error {
	switch args[0] {
	case "up":
		if len(args) == 1 {
			if err := db.MigrateUp(database); err != nil {
				return err
			}
			return logVersion(database, zapLogger, "migrate up complete")
		}
		n, err := parsePositiveInt(args[1], "up")
		if err != nil {
			return err
		}
		if err := db.MigrateSteps(database, n); err != nil {
			return err
		}
		return logVersion(database, zapLogger, "migrate up complete")

	case "down":
		if len(args) == 1 || args[1] == "all" {
			if err := db.MigrateDown(database); err != nil {
				return err
			}
			return logVersion(database, zapLogger, "migrate down complete")
		}
		n, err := parsePositiveInt(args[1], "down")
		if err != nil {
			return err
		}
		if err := db.MigrateSteps(database, -n); err != nil {
			return err
		}
		return logVersion(database, zapLogger, "migrate down complete")

	case "goto", "to":
		if len(args) < 2 {
			return fmt.Errorf("%s requires a version", args[0])
		}
		version, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid version %q", args[1])
		}
		if err := db.MigrateToVersion(database, uint(version)); err != nil {
			return err
		}
		return logVersion(database, zapLogger, "migrate goto complete")

	case "force":
		if len(args) < 2 {
			return errors.New("force requires a version")
		}
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version %q", args[1])
		}
		if err := db.ForceVersion(database, version); err != nil {
			return err
		}
		return logVersion(database, zapLogger, "forced migration version")

	case "version", "status":
		return logVersion(database, zapLogger, "current migration version")

	case "help", "-h", "--help":
		printUsage()
		return nil

	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parsePositiveInt(raw, cmd string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s expects a positive integer, got %q", cmd, raw)
	}
	return n, nil
}

func logVersion(database *gorm.DB, zapLogger *zap.SugaredLogger, msg string) error {
	version, dirty, err := db.CurrentVersion(database)
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			zapLogger.Infow(msg, "version", nil, "dirty", false)
			return nil
		}
		return err
	}
	zapLogger.Infow(msg, "version", version, "dirty", dirty)
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: migrate <command> [args]

Commands:
  up [N]           Apply all pending migrations, or N steps
  down [N|all]     Roll back all migrations, or N steps
  goto <version>   Migrate to a specific version
  force <version>  Set version without running migrations (dirty recovery)
  version          Print the current schema version
  help             Show this help

Examples:
  go run ./cmd/migrate up
  go run ./cmd/migrate up 1
  go run ./cmd/migrate down 1
  go run ./cmd/migrate goto 2
  go run ./cmd/migrate version
`)
}
