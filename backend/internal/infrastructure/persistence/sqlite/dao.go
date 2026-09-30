package sqlite

import (
	"context"
	"database/sql"
	"notes/internal/domain"
)

// NoteDAO handles raw SQL interactions.
type NoteDAO struct {
	db *sql.DB
}

func NewNoteDAO(db *sql.DB) *NoteDAO {
	return &NoteDAO{db: db}
}

func (dao *NoteDAO) Insert(ctx context.Context, n *domain.Note) (int64, error) {
	query := `INSERT INTO notes (title, content, created_at, updated_at) VALUES (?, ?, ?, ?)`
	res, err := dao.db.ExecContext(ctx, query, n.Title, n.Content, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (dao *NoteDAO) FindByID(ctx context.Context, id int64) (*domain.Note, error) {
	query := `SELECT id, title, content, created_at, updated_at FROM notes WHERE id = ?`
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

func (dao *NoteDAO) Update(ctx context.Context, n *domain.Note) error {
	query := `UPDATE notes SET title = ?, content = ?, updated_at = ? WHERE id = ?`
	_, err := dao.db.ExecContext(ctx, query, n.Title, n.Content, n.UpdatedAt, n.ID)
	return err
}

func (dao *NoteDAO) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM notes WHERE id = ?`
	_, err := dao.db.ExecContext(ctx, query, id)
	return err
}

func (dao *NoteDAO) SelectAll(ctx context.Context) ([]*domain.Note, error) {
	query := `SELECT id, title, content, created_at, updated_at FROM notes`
	rows, err := dao.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*domain.Note
	for rows.Next() {
		var n domain.Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, &n)
	}
	return notes, nil
}
