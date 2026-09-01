// Package migrations provides support for applying the versioned SQL schema.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var migrationsFS embed.FS

func Up(ctx context.Context, dsn string) error {
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
