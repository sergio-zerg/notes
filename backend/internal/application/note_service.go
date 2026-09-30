package application

import (
	"context"
	"fmt"
	"notes/internal/domain"
)

type NoteService struct {
	repo domain.NoteRepository
}

func NewNoteService(repo domain.NoteRepository) *NoteService {
	return &NoteService{repo: repo}
}

func (s *NoteService) CreateNote(ctx context.Context, title, content string) (*domain.Note, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	note := &domain.Note{
		Title:   title,
		Content: content,
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
	note, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	note.Title = title
	note.Content = content

	if err := s.repo.Update(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *NoteService) DeleteNote(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *NoteService) ListNotes(ctx context.Context) ([]*domain.Note, error) {
	return s.repo.List(ctx)
}
