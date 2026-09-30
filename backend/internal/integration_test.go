package main

import (
	"context"
	"notes/internal/application"
	"notes/internal/infrastructure/persistence"
	"notes/internal/infrastructure/persistence/sqlite"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteIntegration(t *testing.T) {
	// Use a temporary database file for integration tests
	dbPath := "test_notes.db"
	db, err := persistence.NewSQLiteDB(dbPath)
	require.NoError(t, err)
	defer db.Close()

	dao := sqlite.NewNoteDAO(db)
	repo := sqlite.NewSQLiteNoteRepository(dao)
	service := application.NewNoteService(repo)

	ctx := context.Background()

	t.Run("Full CRUD Cycle", func(t *testing.T) {
		// 1. Create
		title := "Integration Title"
		content := "Integration Content"
		note, err := service.CreateNote(ctx, title, content)
		require.NoError(t, err)
		assert.NotZero(t, note.ID)

		// 2. Read
		found, err := service.GetNote(ctx, note.ID)
		require.NoError(t, err)
		assert.Equal(t, title, found.Title)

		// 3. Update
		newTitle := "Updated Integration Title"
		updated, err := service.UpdateNote(ctx, note.ID, newTitle, content)
		require.NoError(t, err)
		assert.Equal(t, newTitle, updated.Title)

		// 4. List
		list, err := service.ListNotes(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, list)

		// 5. Delete
		err = service.DeleteNote(ctx, note.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetNote(ctx, note.ID)
		assert.Error(t, err)
	})
}
