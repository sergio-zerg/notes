package domain

import "context"

// NoteRepository defines the interface for note persistence.
// Following DDD, this interface lives in the domain layer.
type NoteRepository interface {
	Create(ctx context.Context, note *Note) error
	GetByID(ctx context.Context, id int64) (*Note, error)
	Update(ctx context.Context, note *Note) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*Note, error)
}
