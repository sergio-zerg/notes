package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"notes/internal/domain"
)

func mockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// A zero row count means the note vanished; the DAO must translate that into
// ErrNoteNotFound so the transports can answer 404 instead of a silent success.
func TestDeleteReturnsNotFoundWhenNoRowsAffected(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	mock.ExpectExec(`DELETE FROM notes WHERE id = \$1`).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := dao.Delete(context.Background(), 42)

	if !errors.Is(err, domain.ErrNoteNotFound) {
		t.Errorf("Delete() = %v, want ErrNoteNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestDeleteSuccess(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	mock.ExpectExec(`DELETE FROM notes WHERE id = \$1`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := dao.Delete(context.Background(), 1); err != nil {
		t.Errorf("Delete() = %v, want nil", err)
	}
}

// An UPDATE that returns no row means the note never existed or was deleted
// concurrently. Both are a 404, not a silent success.
func TestUpdateReturnsNotFoundWhenNoRowsReturned(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	note := &domain.Note{ID: 7, Title: "Valid Title", Content: "body"}

	mock.ExpectQuery(`UPDATE notes SET`).
		WithArgs(note.Title, note.Content, note.UpdatedAt, note.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}))

	got, err := dao.Update(context.Background(), note)

	if !errors.Is(err, domain.ErrNoteNotFound) {
		t.Errorf("Update() = %v, want ErrNoteNotFound", err)
	}
	if got != nil {
		t.Errorf("Update() = %+v, want nil alongside the error", got)
	}
}

// RETURNING must give back the stored row, including created_at, which the
// UPDATE statement itself never touches.
func TestUpdateReturnsStoredRow(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	createdAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)

	note := &domain.Note{ID: 7, Title: "New Title", Content: "new body", UpdatedAt: updatedAt}

	mock.ExpectQuery(`UPDATE notes SET .* RETURNING id, title, content, created_at, updated_at`).
		WithArgs(note.Title, note.Content, note.UpdatedAt, note.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}).
			AddRow(int64(7), "New Title", "new body", createdAt, updatedAt))

	got, err := dao.Update(context.Background(), note)

	if err != nil {
		t.Fatalf("Update() = %v, want nil", err)
	}
	if got.ID != 7 {
		t.Errorf("ID = %d, want 7", got.ID)
	}
	if got.Title != "New Title" || got.Content != "new body" {
		t.Errorf("Title/Content = %q/%q, want %q/%q", got.Title, got.Content, "New Title", "new body")
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, createdAt)
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, updatedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// The repository copies the stored row back over the note it was handed, so the
// caller ends up with created_at even though it never read the row first.
func TestRepositoryUpdateRefreshesNoteInPlace(t *testing.T) {
	db, mock := mockDB(t)
	repo := NewPostgresNoteRepository(NewNoteDAO(db))

	createdAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	storedAt := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)

	note := &domain.Note{ID: 7, Title: "New Title", Content: "new body"}
	mock.ExpectQuery(`UPDATE notes SET`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}).
			AddRow(int64(7), "New Title", "new body", createdAt, storedAt))

	if err := repo.Update(context.Background(), note); err != nil {
		t.Fatalf("Update() = %v, want nil", err)
	}

	if !note.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", note.CreatedAt, createdAt)
	}
	// RETURNING is the authority on both stamps: the stored values win over
	// whatever the repository proposed.
	if !note.UpdatedAt.Equal(storedAt) {
		t.Errorf("UpdatedAt = %v, want the stored value %v", note.UpdatedAt, storedAt)
	}
}

// timeArg captures the time.Time the DAO sends for a positional argument.
type timeArg struct{ got *time.Time }

func (t timeArg) Match(v driver.Value) bool {
	ts, ok := v.(time.Time)
	if !ok {
		return false
	}
	*t.got = ts
	return true
}

// The repository must send a UTC timestamp, so that stored values do not depend
// on the process's local zone — the container image sets no TZ.
func TestRepositoryUpdateSendsUTCTimestamp(t *testing.T) {
	db, mock := mockDB(t)
	repo := NewPostgresNoteRepository(NewNoteDAO(db))

	var sentAt time.Time
	mock.ExpectQuery(`UPDATE notes SET`).
		WithArgs("New Title", "new body", timeArg{got: &sentAt}, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}).
			AddRow(int64(7), "New Title", "new body", time.Now().UTC(), time.Now().UTC()))

	note := &domain.Note{ID: 7, Title: "New Title", Content: "new body"}
	if err := repo.Update(context.Background(), note); err != nil {
		t.Fatalf("Update() = %v, want nil", err)
	}

	if sentAt.IsZero() {
		t.Fatal("updated_at was not sent")
	}
	if sentAt.Location() != time.UTC {
		t.Errorf("updated_at location = %v, want UTC", sentAt.Location())
	}
	if time.Since(sentAt) > time.Minute {
		t.Errorf("updated_at = %v, want roughly now", sentAt)
	}
}

func TestFindByIDMapsNoRowsToNotFound(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	mock.ExpectQuery(`SELECT id, title, content, created_at, updated_at FROM notes WHERE id = \$1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}))

	_, err := dao.FindByID(context.Background(), 5)

	if !errors.Is(err, domain.ErrNoteNotFound) {
		t.Errorf("FindByID() = %v, want ErrNoteNotFound", err)
	}
}

// The paging placeholders must stay contiguous when a search term is present,
// otherwise Postgres rejects the query with a syntax error.
func TestSelectAllBuildsContiguousPlaceholders(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	mock.ExpectQuery(`WHERE \(title ILIKE \$1 OR content ILIKE \$1\) ORDER BY id LIMIT \$2 OFFSET \$3`).
		WithArgs("%report%", 10, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}))

	filter := domain.ListFilter{Query: "report", Limit: 10, Offset: 20}
	if _, err := dao.SelectAll(context.Background(), filter); err != nil {
		t.Errorf("SelectAll() = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// Clause order is part of the contract: ORDER BY must precede LIMIT/OFFSET,
// otherwise Postgres answers "syntax error at or near ORDER". The expectations
// are anchored so that a reordered clause fails here rather than in production.
func TestSelectAllClauseOrder(t *testing.T) {
	cases := []struct {
		name   string
		filter domain.ListFilter
		query  string
	}{
		{
			"no filters",
			domain.ListFilter{},
			`^SELECT id, title, content, created_at, updated_at FROM notes ORDER BY id$`,
		},
		{
			"limit only",
			domain.ListFilter{Limit: 5},
			`ORDER BY id LIMIT \$1$`,
		},
		{
			"limit and offset",
			domain.ListFilter{Limit: 5, Offset: 10},
			`ORDER BY id LIMIT \$1 OFFSET \$2$`,
		},
		{
			"search and paging",
			domain.ListFilter{Query: "x", Limit: 5, Offset: 10},
			`ILIKE \$1\) ORDER BY id LIMIT \$2 OFFSET \$3$`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := mockDB(t)
			dao := NewNoteDAO(db)

			mock.ExpectQuery(tc.query).
				WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}))

			if _, err := dao.SelectAll(context.Background(), tc.filter); err != nil {
				t.Fatalf("SelectAll() = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestSelectAllWithoutFilters(t *testing.T) {
	db, mock := mockDB(t)
	dao := NewNoteDAO(db)

	mock.ExpectQuery(`^SELECT id, title, content, created_at, updated_at FROM notes ORDER BY id$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content", "created_at", "updated_at"}))

	notes, err := dao.SelectAll(context.Background(), domain.ListFilter{})
	if err != nil {
		t.Fatalf("SelectAll() = %v", err)
	}
	if notes == nil {
		t.Error("notes = nil, want an empty slice")
	}
}
