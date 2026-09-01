// Package database provides support for connecting to Postgres via GORM.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"
)

type Config struct {
	DSN                string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	SlowQueryThreshold time.Duration
}

// Open never dials the database, call Ping to verify connectivity.
// A zero SlowQueryThreshold disables slow query warnings.
func Open(cfg Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: logger.NewSlogLogger(slog.Default(), logger.Config{
			SlowThreshold:             cfg.SlowQueryThreshold,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
		TranslateError:       true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err = db.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
		return nil, fmt.Errorf("registering otel plugin: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("unwrapping sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	return db, nil
}

// Ping waits until the database can serve queries or ctx is done.
func Ping(ctx context.Context, db *gorm.DB) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, time.Second)
		defer cancel()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("unwrapping sql.DB: %w", err)
	}

	for attempts := 1; ; attempts++ {
		if err := sqlDB.PingContext(ctx); err == nil {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempts) * 100 * time.Millisecond):
		}
	}

	var n int

	return db.WithContext(ctx).Raw("SELECT 1").Scan(&n).Error
}
