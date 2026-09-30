package application

import (
	"context"
	"notes/internal/domain"
	"strings"
	"testing"
	"time"

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

func (m *MockNoteRepository) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Note, error) {
	args := m.Called(ctx, filter)
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

	// The repository writes the stored row back into the note it was given, so
	// the mock mirrors that contract.
	fillIn := func(args mock.Arguments) {
		n := args.Get(1).(*domain.Note)
		n.CreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		n.UpdatedAt = time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)
	}

	t.Run("success", func(t *testing.T) {
		repo.On("Update", ctx, mock.AnythingOfType("*domain.Note")).
			Run(fillIn).Return(nil).Once()

		note, err := service.UpdateNote(ctx, 1, "New", "New Content")

		assert.NoError(t, err)
		assert.Equal(t, int64(1), note.ID)
		assert.Equal(t, "New", note.Title)
		assert.Equal(t, "New Content", note.Content)
		// created_at comes from the store, not from the request payload.
		assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), note.CreatedAt)
		repo.AssertExpectations(t)
	})

	t.Run("update non-existent note", func(t *testing.T) {
		repo.On("Update", ctx, mock.AnythingOfType("*domain.Note")).
			Return(domain.ErrNoteNotFound).Once()

		_, err := service.UpdateNote(ctx, 999, "Title", "Content")

		assert.ErrorIs(t, err, domain.ErrNoteNotFound)
		repo.AssertExpectations(t)
	})

	t.Run("invalid payload does not reach the repository", func(t *testing.T) {
		_, err := service.UpdateNote(ctx, 1, "ab", "Content")

		assert.ErrorIs(t, err, domain.ErrInvalidInput)
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

func TestNoteService_ListNotes(t *testing.T) {
	repo := new(MockNoteRepository)
	service := NewNoteService(repo)
	ctx := context.Background()

	filter := domain.ListFilter{Query: "report", Limit: 10, Offset: 0}

	t.Run("passes filter through to repository", func(t *testing.T) {
		expected := []*domain.Note{{ID: 1, Title: "Weekly report"}}
		repo.On("List", ctx, filter).Return(expected, nil).Once()

		notes, err := service.ListNotes(ctx, filter)

		assert.NoError(t, err)
		assert.Equal(t, expected, notes)
		repo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		repo.On("List", ctx, filter).Return(nil, domain.ErrInternal).Once()

		notes, err := service.ListNotes(ctx, filter)

		assert.ErrorIs(t, err, domain.ErrInternal)
		assert.Nil(t, notes)
		repo.AssertExpectations(t)
	})
}

func TestNoteService_Validation(t *testing.T) {
	repo := new(MockNoteRepository)
	service := NewNoteService(repo)
	ctx := context.Background()

	cases := []struct {
		name    string
		title   string
		content string
		wantErr string
	}{
		{"empty title", "", "content", "title is required"},
		{"title too short", "ab", "content", "title must be at least 3 characters"},
		{"title too long", strings.Repeat("a", 101), "content", "title must be at most 100 characters"},
		{"empty content", "Valid Title", "", "content is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.CreateNote(ctx, tc.title, tc.content)

			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.Contains(t, err.Error(), tc.wantErr)
			repo.AssertNotCalled(t, "Create")
		})
	}

	t.Run("multibyte title is measured in runes", func(t *testing.T) {
		// "Привет" is 6 runes but 12 bytes; it must pass validation.
		repo.On("Create", ctx, mock.AnythingOfType("*domain.Note")).Return(nil).Once()

		note, err := service.CreateNote(ctx, "Привет", "content")

		assert.NoError(t, err)
		assert.Equal(t, "Привет", note.Title)
		repo.AssertExpectations(t)
	})
}
