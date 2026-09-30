//go:build integration

// Package main holds the end-to-end test. It is excluded from the default
// build by the "integration" tag, so `go build ./...` and `go test ./...`
// work without a database. Run it with:
//
//	go test -tags=integration -v ./internal/
package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"notes/internal/application"
	"notes/internal/domain"
	"notes/internal/infrastructure/persistence"
	"notes/internal/infrastructure/persistence/postgres"
)

// testDBConfig reads connection settings from the environment. The test is
// skipped when they are absent, which keeps `go test ./...` usable offline.
func testDBConfig(t *testing.T) (host, port, user, pass, name string) {
	t.Helper()

	env := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	host = env("NOTES_TEST_DB_HOST", "")
	if host == "" {
		t.Skip("set NOTES_TEST_DB_HOST to run integration tests")
	}

	return host,
		env("NOTES_TEST_DB_PORT", "5432"),
		env("NOTES_TEST_DB_USER", "postgres"),
		env("NOTES_TEST_DB_PASS", "password"),
		env("NOTES_TEST_DB_NAME", "notes_db")
}

func TestNoteIntegration(t *testing.T) {
	host, port, user, pass, name := testDBConfig(t)

	db, err := persistence.NewPostgresDB(host, port, user, pass, name)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	dao := postgres.NewNoteDAO(db)
	repo := postgres.NewPostgresNoteRepository(dao)
	service := application.NewNoteService(repo)

	ctx := context.Background()

	t.Run("Full CRUD Cycle", func(t *testing.T) {
		title := "Integration Title"
		content := "Integration Content"

		// 1. Create
		note, err := service.CreateNote(ctx, title, content)
		require.NoError(t, err)
		require.NotZero(t, note.ID)

		// 2. Read
		found, err := service.GetNote(ctx, note.ID)
		require.NoError(t, err)
		assert.Equal(t, title, found.Title)
		assert.Equal(t, content, found.Content)

		// 3. Update
		newTitle := "Updated Integration Title"
		updated, err := service.UpdateNote(ctx, note.ID, newTitle, content)
		require.NoError(t, err)
		assert.Equal(t, newTitle, updated.Title)

		// 4. List
		list, err := service.ListNotes(ctx, domain.ListFilter{Query: "Updated Integration"})
		require.NoError(t, err)
		assert.NotEmpty(t, list)

		// 5. Delete
		require.NoError(t, service.DeleteNote(ctx, note.ID))

		// Verify deletion
		_, err = service.GetNote(ctx, note.ID)
		assert.ErrorIs(t, err, domain.ErrNoteNotFound)
	})

	t.Run("timestamps round-trip", func(t *testing.T) {
		note, err := service.CreateNote(ctx, "Timestamp Title", "content")
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.DeleteNote(ctx, note.ID) })

		assert.WithinDuration(t, time.Now(), note.CreatedAt, time.Minute)
		assert.WithinDuration(t, time.Now(), note.UpdatedAt, time.Minute)

		updated, err := service.UpdateNote(ctx, note.ID, "Timestamp Title v2", "content")
		require.NoError(t, err)
		assert.False(t, updated.UpdatedAt.Before(note.UpdatedAt),
			"updated_at must not move backwards")
	})

	t.Run("domain validation rejects bad input before hitting the database", func(t *testing.T) {
		_, err := service.CreateNote(ctx, "ab", "content")
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	// UPDATE ... RETURNING must preserve created_at; the old SELECT-then-UPDATE
	// pair could reset it, which sqlmock cannot catch because it never runs SQL.
	t.Run("update preserves created_at and advances updated_at", func(t *testing.T) {
		note, err := service.CreateNote(ctx, "Immutable Stamp", "content")
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.DeleteNote(ctx, note.ID) })

		before := note.CreatedAt
		updated, err := service.UpdateNote(ctx, note.ID, "Immutable Stamp v2", "content v2")
		require.NoError(t, err)

		assert.WithinDuration(t, before, updated.CreatedAt, time.Second,
			"created_at must not change on update")
		assert.False(t, updated.UpdatedAt.Before(note.UpdatedAt),
			"updated_at must advance on update")
	})

	// Without a preceding SELECT, a note that does not exist must still 404.
	t.Run("update of a missing note reports not found", func(t *testing.T) {
		_, err := service.UpdateNote(ctx, 999_999_999, "No Such Note", "content")
		assert.ErrorIs(t, err, domain.ErrNoteNotFound)
	})

	// Paging must not repeat or skip records, which requires ORDER BY.
	t.Run("paging is stable and ordered by id", func(t *testing.T) {
		var ids []int64
		for i := 0; i < 5; i++ {
			note, err := service.CreateNote(ctx, fmt.Sprintf("Paged Note %d", i), "content")
			require.NoError(t, err)
			t.Cleanup(func() { _ = service.DeleteNote(ctx, note.ID) })
			ids = append(ids, note.ID)
		}

		first, err := service.ListNotes(ctx, domain.ListFilter{Query: "Paged Note", Limit: 2, Offset: 0})
		require.NoError(t, err)
		second, err := service.ListNotes(ctx, domain.ListFilter{Query: "Paged Note", Limit: 2, Offset: 2})
		require.NoError(t, err)

		require.Len(t, first, 2)
		require.Len(t, second, 2)

		// The two pages must not overlap, and every id must be ascending.
		for _, page := range [][]*domain.Note{first, second} {
			for i := 1; i < len(page); i++ {
				assert.Less(t, page[i-1].ID, page[i].ID, "results must ascend by id")
			}
		}
		assert.Equal(t, first[1].ID, second[0].ID-1,
			"the second page must continue where the first ended")
	})
}
