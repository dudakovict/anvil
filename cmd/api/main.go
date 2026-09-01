// Command api runs the HTTP API server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Registers the generated swagger spec.
	_ "github.com/dudakovict/anvil/docs"

	"github.com/dudakovict/anvil/internal/adapter/postgres"
	"github.com/dudakovict/anvil/internal/adapter/rest"
	"github.com/dudakovict/anvil/internal/config"
	"github.com/dudakovict/anvil/internal/core/service"
	"github.com/dudakovict/anvil/internal/platform/auth"
	"github.com/dudakovict/anvil/internal/platform/database"
	"github.com/dudakovict/anvil/internal/platform/logger"
	"github.com/dudakovict/anvil/internal/platform/telemetry"
	"github.com/dudakovict/anvil/migrations"
)

// version is stamped at build time via ldflags.
var version = "dev"

// main godoc
//
//	@title						anvil API
//	@version					1.0
//	@description				Production-ready Go backend starter.
//	@BasePath					/api/v1
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and the token.
func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	slog.SetDefault(logger.New(os.Stderr, level))

	slog.InfoContext(ctx, "starting", "env", cfg.Env, "addr", cfg.HTTP.Addr, "version", version)

	shutdownTelemetry, err := telemetry.Init(ctx, telemetry.Config{
		Enabled:        cfg.OTel.Enabled,
		ServiceName:    cfg.OTel.ServiceName,
		ServiceVersion: version,
		Environment:    cfg.Env,
		OTLPEndpoint:   cfg.OTel.OTLPEndpoint,
		OTLPInsecure:   cfg.OTel.OTLPInsecure,
		SampleRatio:    cfg.OTel.SampleRatio,
	})
	if err != nil {
		return err
	}
	defer func() {
		// The signal ctx is already cancelled so flushing needs a fresh one.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:contextcheck // intentional: parent ctx is cancelled
		defer cancel()

		if shutdownErr := shutdownTelemetry(flushCtx); shutdownErr != nil { //nolint:contextcheck // fresh deadline ctx, see above
			slog.Error("shutting down telemetry", "err", shutdownErr)
		}
	}()

	db, err := database.Open(database.Config{
		DSN:                cfg.DB.DSN,
		MaxOpenConns:       cfg.DB.MaxOpenConns,
		MaxIdleConns:       cfg.DB.MaxIdleConns,
		ConnMaxLifetime:    cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime:    cfg.DB.ConnMaxIdleTime,
		SlowQueryThreshold: cfg.DB.SlowQueryThreshold,
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err = database.Ping(pingCtx, db); err != nil {
		return err
	}

	if cfg.DB.Migrate {
		if err = migrations.Up(ctx, cfg.DB.DSN); err != nil {
			return err
		}

		slog.InfoContext(ctx, "migrations applied")
	}

	users := service.NewUserService(postgres.NewUserRepository(db))

	router := rest.NewRouter(rest.Deps{
		Cfg:   cfg,
		DB:    sqlDB,
		Users: users,
		Auth:  auth.New(auth.Config{Secret: cfg.Auth.Secret, TTL: cfg.Auth.TokenTTL}),
	})

	srv := rest.NewServer(cfg.HTTP, router)

	return rest.Run(ctx, srv, cfg.HTTP.ShutdownTimeout)
}
