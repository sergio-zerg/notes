package grpc

import (
	"context"
	"errors"
	"log/slog"

	"notes/api/proto"
	"notes/internal/application"
	"notes/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

type NoteHandler struct {
	proto.UnimplementedNoteServiceServer
	service *application.NoteService
	logger  *slog.Logger
}

func NewNoteHandler(service *application.NoteService) *NoteHandler {
	return &NoteHandler{
		service: service,
		logger:  slog.Default(),
	}
}

// toStatus maps a domain error to a gRPC status. Client errors keep their
// message; anything unexpected is logged and reduced to a generic Internal so
// that internal details are not leaked to the caller.
func (h *NoteHandler) toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrNoteNotFound):
		return status.Error(codes.NotFound, "note not found")
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		h.logger.Error("grpc request failed", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}

func (h *NoteHandler) CreateNote(ctx context.Context, req *proto.CreateNoteRequest) (*proto.Note, error) {
	note, err := h.service.CreateNote(ctx, req.Title, req.Content)
	if err != nil {
		return nil, h.toStatus(err)
	}
	return mapToProto(note), nil
}

func (h *NoteHandler) GetNote(ctx context.Context, req *proto.GetNoteRequest) (*proto.Note, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "note id must be positive")
	}

	note, err := h.service.GetNote(ctx, req.GetId())
	if err != nil {
		return nil, h.toStatus(err)
	}
	return mapToProto(note), nil
}

func (h *NoteHandler) UpdateNote(ctx context.Context, req *proto.UpdateNoteRequest) (*proto.Note, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "note id must be positive")
	}

	note, err := h.service.UpdateNote(ctx, req.GetId(), req.Title, req.Content)
	if err != nil {
		return nil, h.toStatus(err)
	}
	return mapToProto(note), nil
}

func (h *NoteHandler) DeleteNote(ctx context.Context, req *proto.DeleteNoteRequest) (*proto.DeleteNoteResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "note id must be positive")
	}

	if err := h.service.DeleteNote(ctx, req.GetId()); err != nil {
		return nil, h.toStatus(err)
	}
	return &proto.DeleteNoteResponse{Success: true}, nil
}

func (h *NoteHandler) ListNotes(ctx context.Context, req *proto.ListNotesRequest) (*proto.ListNotesResponse, error) {
	// The request's paging and search fields are honoured rather than dropped.
	filter := domain.ListFilter{
		Query:  req.GetQuery(),
		Limit:  clampInt(int(req.GetLimit()), 1, maxListLimit, defaultListLimit),
		Offset: clampInt(int(req.GetOffset()), 0, maxListLimit, 0),
	}

	notes, err := h.service.ListNotes(ctx, filter)
	if err != nil {
		return nil, h.toStatus(err)
	}

	protoNotes := make([]*proto.Note, 0, len(notes))
	for _, n := range notes {
		protoNotes = append(protoNotes, mapToProto(n))
	}
	return &proto.ListNotesResponse{Notes: protoNotes}, nil
}

func clampInt(v, min, max, fallback int) int {
	if v == 0 {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func mapToProto(n *domain.Note) *proto.Note {
	return &proto.Note{
		Id:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		CreatedAt: timestamppb.New(n.CreatedAt),
		UpdatedAt: timestamppb.New(n.UpdatedAt),
	}
}
