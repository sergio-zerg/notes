package domain

import "context"

// ListFilter defines parameters for listing notes.
type ListFilter struct {
	Query  string
	Limit  int
	Offset int
}

// NoteRepository defines the interface for note persistence.
// Following DDD, this interface lives in the domain layer.
//
// Every method returns an error wrapping ErrNoteNotFound when the target note
// does not exist, so callers need no separate existence check.
type NoteRepository interface {
	// Create assigns the generated ID and timestamps to note.
	Create(ctx context.Context, note *Note) error

	GetByID(ctx context.Context, id int64) (*Note, error)

	// Update writes the note's title and content and replaces note in place
	// with the row as stored, so the caller learns the authoritative values
	// (notably created_at, which the write does not change) without reading
	// the row beforehand.
	Update(ctx context.Context, note *Note) error

	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter ListFilter) ([]*Note, error)
}
