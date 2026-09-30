package persistence

import (
	"io/fs"
	"strings"
	"testing"
)

// TestMigrationsEmbedded guards the //go:embed pattern in db.go. If the SQL
// files are moved or renamed without updating the pattern, the build breaks
// with a confusing "no matching files" error; this test states the intent.
func TestMigrationsEmbedded(t *testing.T) {
	entries, err := fs.Glob(migrationsFS, "migrations/postgres/*.sql")
	if err != nil {
		t.Fatalf("glob embedded migrations: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no migrations embedded; check the //go:embed pattern in db.go")
	}

	for _, name := range entries {
		body, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		// goose requires the Up/Down annotations to recognise a migration.
		if !strings.Contains(string(body), "-- +goose Up") {
			t.Errorf("%s: missing '-- +goose Up' annotation", name)
		}
		if !strings.Contains(string(body), "-- +goose Down") {
			t.Errorf("%s: missing '-- +goose Down' annotation", name)
		}
	}
}
