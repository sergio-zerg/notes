package application

import (
	"context"
	"notes/internal/domain"
)

type NoteService struct {
	repo domain.NoteRepository
}

func NewNoteService(repo domain.NoteRepository) *NoteService {
	return &NoteService{repo: repo}
}

func (s *NoteService) CreateNote(ctx context.Context, title, content string) (*domain.Note, error) {
	note := &domain.Note{
		Title:   title,
		Content: content,
	}
	if err := note.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *NoteService) GetNote(ctx context.Context, id int64) (*domain.Note, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *NoteService) UpdateNote(ctx context.Context, id int64, title, content string) (*domain.Note, error) {
	candidate := &domain.Note{
		ID:      id,
		Title:   title,
		Content: content,
	}
	// Validate before touching the database so a malformed payload costs
	// nothing, and so the error is about the input rather than about
	// whether the note happens to exist.
	if err := candidate.Validate(); err != nil {
		return nil, err
	}

	// A single UPDATE ... RETURNING replaces the old SELECT-then-UPDATE pair:
	// one round trip instead of two, and no window in which a concurrent
	// write could be silently overwritten by a stale read.
	if err := s.repo.Update(ctx, candidate); err != nil {
		return nil, err
	}
	return candidate, nil
}

func (s *NoteService) DeleteNote(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *NoteService) ListNotes(ctx context.Context, filter domain.ListFilter) ([]*domain.Note, error) {
	return s.repo.List(ctx, filter)
}
