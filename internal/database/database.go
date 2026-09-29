// Package database manages the Postgres connection and applies embedded schema migrations.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/database/migrations"
)

// Connect opens a connection pool to Postgres and verifies it is reachable.
func Connect(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return db, nil
}

// Migrate applies all embedded schema migrations that have not yet run.
func Migrate(db *sql.DB) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("loading embedded migrations: %w", err)
	}

	dbDriver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx", dbDriver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
