package persistence

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// migrationsFS embeds the SQL migration files into the binary, so that
// schema management no longer depends on the process working directory.
// This is what lets the container image run without shipping the .sql files.
//
//go:embed migrations/postgres/*.sql
var migrationsFS embed.FS

// migrationsDir is the path within migrationsFS holding the SQL files.
const migrationsDir = "migrations/postgres"

func NewPostgresDB(host, port, user, pass, dbname string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbname)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		// The close error cannot add information here: the caller gets the
		// connection failure, which is the reason this function returns nil.
		_ = db.Close()
		return nil, err
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	migrations, err := fs.Sub(migrationsFS, migrationsDir)
	if err != nil {
		return fmt.Errorf("prepare migrations fs: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}
