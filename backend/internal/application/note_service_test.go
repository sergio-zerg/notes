package application

import (
	"context"
	"notes/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockNoteRepository struct {
	mock.Mock
}

func (m *MockNoteRepository) Create(ctx context.Context, n *domain.Note) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockNoteRepository) GetByID(ctx context.Context, id int64) (*domain.Note, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Note), args.Error(1)
}

func (m *MockNoteRepository) Update(ctx context.Context, n *domain.Note) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockNoteRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNoteRepository) List(ctx context.Context) ([]*domain.Note, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Note), args.Error(1)
}

func TestNoteService_CreateNote(t *testing.T) {
	repo := new(MockNoteRepository)
	service := NewNoteService(repo)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		title := "Test Title"
		content := "Test Content"
		repo.On("Create", ctx, mock.AnythingOfType("*domain.Note")).Return(nil).Once()

		note, err := service.CreateNote(ctx, title, content)

		assert.NoError(t, err)
		assert.NotNil(t, note)
		assert.Equal(t, title, note.Title)
		repo.AssertExpectations(t)
	})

	t.Run("missing title", func(t *testing.T) {
		_, err := service.CreateNote(ctx, "", "content")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})

	t.Run("repository error", func(t *testing.T) {
		repo.On("Create", ctx, mock.AnythingOfType("*domain.Note")).Return(domain.ErrNoteNotFound).Once()
		_, err := service.CreateNote(ctx, "Title", "Content")
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})
}

func TestNoteService_GetNote(t *testing.T) {
	repo := new(MockNoteRepository)
	service := NewNoteService(repo)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := &domain.Note{ID: 1, Title: "Test"}
		repo.On("GetByID", ctx, int64(1)).Return(expected, nil).Once()

		note, err := service.GetNote(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, expected, note)
		repo.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		repo.On("GetByID", ctx, int64(404)).Return(nil, domain.ErrNoteNotFound).Once()

		note, err := service.GetNote(ctx, 404)

		assert.Nil(t, note)
		assert.ErrorIs(t, err, domain.ErrNoteNotFound)
		repo.AssertExpectations(t)
	})
}

func TestNoteService_UpdateNote(t *testing.T) {
	repo := new(MockNoteRepository)
	service := NewNoteService(repo)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		existing := &domain.Note{ID: 1, Title: "Old", Content: "Old"}
		repo.On("GetByID", ctx, int64(1)).Return(existing, nil).Once()
		repo.On("Update", ctx, mock.AnythingOfType("*domain.Note")).Return(nil).Once()

		note, err := service.UpdateNote(ctx, 1, "New", "New Content")

		assert.NoError(t, err)
		assert.Equal(t, "New", note.Title)
		assert.Equal(t, "New Content", note.Content)
		repo.AssertExpectations(t)
	})

	t.Run("update non-existent note", func(t *testing.T) {
		repo.On("GetByID", ctx, int64(999)).Return(nil, domain.ErrNoteNotFound).Once()

		_, err := service.UpdateNote(ctx, 999, "Title", "Content")

		assert.ErrorIs(t, err, domain.ErrNoteNotFound)
		repo.AssertExpectations(t)
	})
}

func TestNoteService_DeleteNote(t *testing.T) {
	repo := new(MockNoteRepository)
	service := NewNoteService(repo)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo.On("Delete", ctx, int64(1)).Return(nil).Once()
		err := service.DeleteNote(ctx, 1)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("delete fails", func(t *testing.T) {
		repo.On("Delete", ctx, int64(1)).Return(domain.ErrNoteNotFound).Once()
		err := service.DeleteNote(ctx, 1)
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})
}
