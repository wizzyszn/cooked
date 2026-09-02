package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/logger"
	"github.com/wizzyszn/cooked/internal/media"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/user"
)

type jobRunner interface{ RunOnce(context.Context) error }

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("initialize configurations: %v", err)
	}
	zapLogger := logger.Init(cfg.Server.Env)
	defer zapLogger.Sync()
	database, err := db.Connect(&cfg.Database, cfg.Server.Env)
	if err != nil {
		zapLogger.Fatalw("connect to database", "error", err)
	}

	users := user.NewRepository(database)
	notificationStore := notify.NewStore(database)
	var providers []notify.ChannelProvider
	if cfg.Brevo.Enabled() {
		providers = append(providers, notify.NewBrevoEmailProvider(&cfg.Brevo, zapLogger))
	}
	var runners []jobRunner
	if len(providers) > 0 {
		runners = append(runners, notify.NewWorker(notificationStore, users, providers, cfg.Worker.ID, cfg.Worker.BatchSize, zapLogger))
	} else {
		zapLogger.Warn("Brevo is not configured; notification jobs will remain queued")
	}
	if cfg.ObjectStorage.Enabled() {
		objects, e := media.NewS3Store(cfg.ObjectStorage)
		if e != nil {
			zapLogger.Fatalw("initialize object storage", "error", e)
		}
		runners = append(runners, media.NewProcessor(media.NewRepository(database), objects, cfg.Worker.ID, cfg.Worker.BatchSize, zapLogger))
	} else {
		zapLogger.Warn("object storage is not configured; media jobs will remain queued")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	ticker := time.NewTicker(time.Duration(cfg.Worker.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()
	zapLogger.Infow("worker started", "worker_id", cfg.Worker.ID)
	for {
		for _, runner := range runners {
			runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			if e := runner.RunOnce(runCtx); e != nil && ctx.Err() == nil {
				zapLogger.Errorw("worker cycle failed", "error", e)
			}
			cancel()
		}
		select {
		case <-ctx.Done():
			zapLogger.Info("worker stopped")
			return
		case <-ticker.C:
		}
	}
}
