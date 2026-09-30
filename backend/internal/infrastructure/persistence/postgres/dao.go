package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"notes/internal/domain"
)

type NoteDAO struct {
	db *sql.DB
}

func NewNoteDAO(db *sql.DB) *NoteDAO {
	return &NoteDAO{db: db}
}

func (dao *NoteDAO) Insert(ctx context.Context, n *domain.Note) (int64, error) {
	query := `INSERT INTO notes (title, content, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`
	var id int64
	err := dao.db.QueryRowContext(ctx, query, n.Title, n.Content, n.CreatedAt, n.UpdatedAt).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (dao *NoteDAO) FindByID(ctx context.Context, id int64) (*domain.Note, error) {
	query := `SELECT id, title, content, created_at, updated_at FROM notes WHERE id = $1`
	row := dao.db.QueryRowContext(ctx, query, id)
	var n domain.Note
	err := row.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNoteNotFound
		}
		return nil, err
	}
	return &n, nil
}

// Update writes the note and returns the row as Postgres stored it.
//
// RETURNING collapses the write and the read-back into a single statement, so
// the caller learns the authoritative created_at (which the UPDATE does not
// touch) without a second round trip. It also removes the window in which a
// concurrent DELETE could slip between a SELECT and the UPDATE.
func (dao *NoteDAO) Update(ctx context.Context, n *domain.Note) (*domain.Note, error) {
	query := `UPDATE notes SET title = $1, content = $2, updated_at = $3
	          WHERE id = $4
	          RETURNING id, title, content, created_at, updated_at`

	row := dao.db.QueryRowContext(ctx, query, n.Title, n.Content, n.UpdatedAt, n.ID)

	var updated domain.Note
	if err := row.Scan(&updated.ID, &updated.Title, &updated.Content, &updated.CreatedAt, &updated.UpdatedAt); err != nil {
		// No returned row means the note never existed or was removed
		// concurrently; both are a 404, not a silent success.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoteNotFound
		}
		return nil, err
	}
	return &updated, nil
}

func (dao *NoteDAO) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM notes WHERE id = $1`
	result, err := dao.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNoteNotFound
	}
	return nil
}

func (dao *NoteDAO) SelectAll(ctx context.Context, filter domain.ListFilter) ([]*domain.Note, error) {
	// Clauses are appended in SQL order: WHERE, then ORDER BY, then LIMIT and
	// OFFSET. ORDER BY after LIMIT is a syntax error in Postgres.
	const columns = `SELECT id, title, content, created_at, updated_at FROM notes`

	var args []any
	argCount := 1

	query := columns
	if filter.Query != "" {
		query += ` WHERE (title ILIKE $1 OR content ILIKE $1)`
		args = append(args, "%"+filter.Query+"%")
		argCount++
	}

	// A deterministic order is required for OFFSET paging to be meaningful:
	// without it Postgres may return rows in any order, so consecutive pages
	// can repeat or skip records. id is unique, which makes it a stable
	// tiebreaker even for notes created within the same timestamp.
	query += ` ORDER BY id`

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
		argCount++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := dao.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	// A close error on a read path carries no useful information: either the
	// scan below fails and is reported, or the rows were consumed cleanly.
	defer func() { _ = rows.Close() }()

	// Initialised non-nil so an empty result encodes as [] in JSON rather
	// than null.
	notes := make([]*domain.Note, 0)
	for rows.Next() {
		var n domain.Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, &n)
	}
	// rows.Err reports failures that surface after the last successful Next,
	// e.g. a connection dropped mid-scan.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notes, nil
}
