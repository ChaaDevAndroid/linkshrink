package postgres

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies every pending migration. It opens its own connection, because
// closing the migrator also closes the *sql.DB it was given.
func Migrate(dataSourceName string) error {
	db, err := sql.Open("pgx", dataSourceName)
	if err != nil {
		return fmt.Errorf("migrate open: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		db.Close()
		return fmt.Errorf("migrate source: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		db.Close()
		return fmt.Errorf("migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		db.Close()
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("no pending migrations")
			return nil
		}
		return fmt.Errorf("migrate up: %w", err)
	}

	slog.Info("migrations applied")
	return nil
}
