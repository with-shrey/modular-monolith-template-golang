package migrator

import (
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// RunMigrations runs all pending goose migrations from the embedded FS against dbUrl.
func RunMigrations(dbUrl string, files fs.FS) error {
	db, err := sql.Open("pgx", dbUrl)
	if err != nil {
		panic(fmt.Errorf("failed to open database: %w", err))
	}
	defer db.Close() //nolint:errcheck

	goose.SetBaseFS(files)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to up migrations: %w", err)
	}
	return nil
}
