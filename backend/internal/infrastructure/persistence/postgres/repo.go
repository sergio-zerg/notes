package postgres

import (
	"context"
	"notes/internal/domain"
	"time"
)

type PostgresNoteRepository struct {
	dao *NoteDAO
}

func NewPostgresNoteRepository(dao *NoteDAO) *PostgresNoteRepository {
	return &PostgresNoteRepository{dao: dao}
}

func (r *PostgresNoteRepository) Create(ctx context.Context, n *domain.Note) error {
	// One instant for both stamps, so a fresh note has
	// created_at == updated_at rather than two slightly different values.
	// The column is TIMESTAMPTZ, so the instant is what matters; UTC is
	// explicit to avoid depending on the container's local zone.
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now

	id, err := r.dao.Insert(ctx, n)
	if err != nil {
		return err
	}
	n.ID = id
	return nil
}

func (r *PostgresNoteRepository) GetByID(ctx context.Context, id int64) (*domain.Note, error) {
	return r.dao.FindByID(ctx, id)
}

// Update applies the note's fields and refreshes n in place with the row as the
// database stored it. The caller receives the authoritative created_at without
// a preliminary SELECT.
func (r *PostgresNoteRepository) Update(ctx context.Context, n *domain.Note) error {
	n.UpdatedAt = time.Now().UTC()

	updated, err := r.dao.Update(ctx, n)
	if err != nil {
		return err
	}
	*n = *updated
	return nil
}

func (r *PostgresNoteRepository) Delete(ctx context.Context, id int64) error {
	return r.dao.Delete(ctx, id)
}

func (r *PostgresNoteRepository) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Note, error) {
	return r.dao.SelectAll(ctx, filter)
}
