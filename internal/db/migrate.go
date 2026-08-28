package db

import (
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

/*
The .sql files are compiled into the binary, so the container needs no migrations
directory mounted next to it and the image cannot drift from the schema it expects
*/
//go:embed migrations/*.sql
var migrationsFS embed.FS

/*
goose talks database/sql, the app talks pgxpool. We open a second, short-lived
handle just for the migration and close it right after; the pool stays untouched.
goose records what it applied in goose_db_version, so a restart is a no-op
*/
func Migrate(url string) error {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("opening db for migrations: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("selecting goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
