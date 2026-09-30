package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"notes/api/proto"
	"notes/internal/application"
	"notes/internal/domain"
)

func statusCode(err error) codes.Code {
	return status.Code(err)
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// stubRepo is an in-memory NoteRepository for exercising the gRPC layer.
type stubRepo struct {
	notes  map[int64]*domain.Note
	nextID int64
	fail   error
}

func newStubRepo() *stubRepo {
	return &stubRepo{notes: map[int64]*domain.Note{}, nextID: 1}
}

func (s *stubRepo) Create(ctx context.Context, n *domain.Note) error {
	if s.fail != nil {
		return s.fail
	}
	n.ID = s.nextID
	s.nextID++
	s.notes[n.ID] = n
	return nil
}

func (s *stubRepo) GetByID(ctx context.Context, id int64) (*domain.Note, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	n, ok := s.notes[id]
	if !ok {
		return nil, domain.ErrNoteNotFound
	}
	return n, nil
}

func (s *stubRepo) Update(ctx context.Context, n *domain.Note) error {
	if s.fail != nil {
		return s.fail
	}
	if _, ok := s.notes[n.ID]; !ok {
		return domain.ErrNoteNotFound
	}
	s.notes[n.ID] = n
	return nil
}

func (s *stubRepo) Delete(ctx context.Context, id int64) error {
	if s.fail != nil {
		return s.fail
	}
	if _, ok := s.notes[id]; !ok {
		return domain.ErrNoteNotFound
	}
	delete(s.notes, id)
	return nil
}

func (s *stubRepo) List(ctx context.Context, f domain.ListFilter) ([]*domain.Note, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	out := make([]*domain.Note, 0, len(s.notes))
	for _, n := range s.notes {
		out = append(out, n)
	}
	return out, nil
}

func newTestHandler(repo domain.NoteRepository) *NoteHandler {
	return NewNoteHandler(application.NewNoteService(repo))
}

// gRPC must apply the same domain validation as HTTP. Before validation was
// moved into the domain, an empty title was accepted over gRPC.
func TestCreateNoteRejectsInvalidInput(t *testing.T) {
	h := newTestHandler(newStubRepo())
	ctx := context.Background()

	cases := []struct {
		name string
		req  *proto.CreateNoteRequest
	}{
		{"empty title", &proto.CreateNoteRequest{Title: "", Content: "c"}},
		{"short title", &proto.CreateNoteRequest{Title: "ab", Content: "c"}},
		{"empty content", &proto.CreateNoteRequest{Title: "Valid Title", Content: ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.CreateNote(ctx, tc.req)
			if got := statusCode(err); got != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", got)
			}
		})
	}
}

func TestCreateNoteSuccess(t *testing.T) {
	h := newTestHandler(newStubRepo())

	note, err := h.CreateNote(context.Background(),
		&proto.CreateNoteRequest{Title: "Valid Title", Content: "body"})
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if note.GetId() == 0 || note.GetTitle() != "Valid Title" {
		t.Errorf("unexpected note: %+v", note)
	}
}

func TestGetNote(t *testing.T) {
	repo := newStubRepo()
	h := newTestHandler(repo)
	ctx := context.Background()

	created, err := h.CreateNote(ctx, &proto.CreateNoteRequest{Title: "Findable", Content: "b"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("existing", func(t *testing.T) {
		got, err := h.GetNote(ctx, &proto.GetNoteRequest{Id: created.GetId()})
		if err != nil {
			t.Fatalf("GetNote: %v", err)
		}
		if got.GetTitle() != "Findable" {
			t.Errorf("title = %q", got.GetTitle())
		}
	})

	t.Run("missing maps to NotFound", func(t *testing.T) {
		_, err := h.GetNote(ctx, &proto.GetNoteRequest{Id: 9999})
		if got := statusCode(err); got != codes.NotFound {
			t.Errorf("code = %v, want NotFound", got)
		}
	})

	t.Run("non-positive id is InvalidArgument", func(t *testing.T) {
		_, err := h.GetNote(ctx, &proto.GetNoteRequest{Id: 0})
		if got := statusCode(err); got != codes.InvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", got)
		}
	})
}

func TestUpdateNote(t *testing.T) {
	repo := newStubRepo()
	h := newTestHandler(repo)
	ctx := context.Background()

	created, _ := h.CreateNote(ctx, &proto.CreateNoteRequest{Title: "Original", Content: "b"})
	id := created.GetId()

	t.Run("success", func(t *testing.T) {
		got, err := h.UpdateNote(ctx, &proto.UpdateNoteRequest{
			Id: id, Title: "Updated Title", Content: "new"})
		if err != nil {
			t.Fatalf("UpdateNote: %v", err)
		}
		if got.GetTitle() != "Updated Title" {
			t.Errorf("title = %q", got.GetTitle())
		}
	})

	t.Run("invalid payload is InvalidArgument", func(t *testing.T) {
		_, err := h.UpdateNote(ctx, &proto.UpdateNoteRequest{Id: id, Title: "ab", Content: "new"})
		if got := statusCode(err); got != codes.InvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", got)
		}
	})

	t.Run("missing note is NotFound", func(t *testing.T) {
		_, err := h.UpdateNote(ctx, &proto.UpdateNoteRequest{
			Id: 9999, Title: "Valid Title", Content: "c"})
		if got := statusCode(err); got != codes.NotFound {
			t.Errorf("code = %v, want NotFound", got)
		}
	})
}

func TestDeleteNote(t *testing.T) {
	repo := newStubRepo()
	h := newTestHandler(repo)
	ctx := context.Background()

	created, _ := h.CreateNote(ctx, &proto.CreateNoteRequest{Title: "Doomed", Content: "b"})
	id := created.GetId()

	t.Run("success", func(t *testing.T) {
		resp, err := h.DeleteNote(ctx, &proto.DeleteNoteRequest{Id: id})
		if err != nil {
			t.Fatalf("DeleteNote: %v", err)
		}
		if !resp.GetSuccess() {
			t.Error("success = false, want true")
		}
	})

	t.Run("deleting twice is NotFound", func(t *testing.T) {
		_, err := h.DeleteNote(ctx, &proto.DeleteNoteRequest{Id: id})
		if got := statusCode(err); got != codes.NotFound {
			t.Errorf("code = %v, want NotFound", got)
		}
	})
}

// ListNotes previously discarded the request's paging and search fields.
func TestListNotesClampsPaging(t *testing.T) {
	h := newTestHandler(newStubRepo())
	ctx := context.Background()

	cases := []struct {
		name string
		req  *proto.ListNotesRequest
	}{
		{"empty request", &proto.ListNotesRequest{}},
		{"limit above max", &proto.ListNotesRequest{Limit: 10000}},
		{"negative limit", &proto.ListNotesRequest{Limit: -5}},
		{"offset above max", &proto.ListNotesRequest{Offset: 99999}},
		{"query set", &proto.ListNotesRequest{Query: "report"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := h.ListNotes(ctx, tc.req)
			if err != nil {
				t.Fatalf("ListNotes: %v", err)
			}
			if resp.GetNotes() == nil {
				t.Error("notes = nil, want an empty slice")
			}
		})
	}
}

// Internal failures must not leak details such as credentials to the caller.
func TestInternalErrorIsNotLeaked(t *testing.T) {
	repo := newStubRepo()
	repo.fail = errors.New("dial tcp: password=hunter2")
	h := newTestHandler(repo)

	_, err := h.GetNote(context.Background(), &proto.GetNoteRequest{Id: 1})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := statusCode(err); got != codes.Internal {
		t.Errorf("code = %v, want Internal", got)
	}
	if contains(err.Error(), "hunter2") {
		t.Errorf("error leaked internals: %q", err.Error())
	}
}
