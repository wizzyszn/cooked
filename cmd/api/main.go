package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wizzyszn/cooked/internal/app"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/logger"
	"github.com/wizzyszn/cooked/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("initialze configurations %v", err)
	}

	zapLogger := logger.Init(cfg.Server.Env)
	zapLogger.Infow("Configuration loaded", "env", cfg.Server.Env, "port", cfg.Server.Port)
	defer zapLogger.Sync()

	zapLogger.Info("Connecting to Database....")
	database, err := db.Connect(&cfg.Database, cfg.Server.Env)
	if err != nil {
		zapLogger.Fatalw("Failed to connect to database.", "error", err)
	}

	// if err := db.Migrate(database); err != nil {
	// 	zapLogger.Fatalw("Failed to migrate database", "error", err)
	// }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := app.NewDependencies(cfg, database, zapLogger)
	r := router.Init(deps)

	srv := http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeOut) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeOut) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeOut) * time.Second,
	}

	go func() {
		zapLogger.Info("Starting server...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLogger.Fatalw("server failed to start", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit

	zapLogger.Info("Shutting down server....")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zapLogger.Errorw("server forced to shutdown", "error", err)
	}
	zapLogger.Info("Server exited.")
}
