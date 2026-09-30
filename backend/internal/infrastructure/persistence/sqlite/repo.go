package sqlite

import (
	"context"
	"notes/internal/domain"
	"time"
)

type SQLiteNoteRepository struct {
	dao *NoteDAO
}

func NewSQLiteNoteRepository(dao *NoteDAO) *SQLiteNoteRepository {
	return &SQLiteNoteRepository{dao: dao}
}

func (r *SQLiteNoteRepository) Create(ctx context.Context, n *domain.Note) error {
	n.CreatedAt = time.Now()
	n.UpdatedAt = time.Now()
	id, err := r.dao.Insert(ctx, n)
	if err != nil {
		return err
	}
	n.ID = id
	return nil
}

func (r *SQLiteNoteRepository) GetByID(ctx context.Context, id int64) (*domain.Note, error) {
	return r.dao.FindByID(ctx, id)
}

func (r *SQLiteNoteRepository) Update(ctx context.Context, n *domain.Note) error {
	n.UpdatedAt = time.Now()
	return r.dao.Update(ctx, n)
}

func (r *SQLiteNoteRepository) Delete(ctx context.Context, id int64) error {
	return r.dao.Delete(ctx, id)
}

func (r *SQLiteNoteRepository) List(ctx context.Context) ([]*domain.Note, error) {
	return r.dao.SelectAll(ctx)
}
