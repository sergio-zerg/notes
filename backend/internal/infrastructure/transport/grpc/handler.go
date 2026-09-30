package grpc

import (
	"context"
	"notes/api/proto"
	"notes/internal/application"
	"notes/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NoteHandler struct {
	proto.UnimplementedNoteServiceServer
	service *application.NoteService
}

func NewNoteHandler(service *application.NoteService) *NoteHandler {
	return &NoteHandler{service: service}
}

func (h *NoteHandler) CreateNote(ctx context.Context, req *proto.CreateNoteRequest) (*proto.Note, error) {
	note, err := h.service.CreateNote(ctx, req.Title, req.Content)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create note: %v", err)
	}
	return mapToProto(note), nil
}

func (h *NoteHandler) GetNote(ctx context.Context, req *proto.GetNoteRequest) (*proto.Note, error) {
	note, err := h.service.GetNote(ctx, req.Id)
	if err != nil {
		if err == domain.ErrNoteNotFound {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get note: %v", err)
	}
	return mapToProto(note), nil
}

func (h *NoteHandler) UpdateNote(ctx context.Context, req *proto.UpdateNoteRequest) (*proto.Note, error) {
	note, err := h.service.UpdateNote(ctx, req.Id, req.Title, req.Content)
	if err != nil {
		if err == domain.ErrNoteNotFound {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to update note: %v", err)
	}
	return mapToProto(note), nil
}

func (h *NoteHandler) DeleteNote(ctx context.Context, req *proto.DeleteNoteRequest) (*proto.DeleteNoteResponse, error) {
	err := h.service.DeleteNote(ctx, req.Id)
	if err != nil {
		if err == domain.ErrNoteNotFound {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete note: %v", err)
	}
	return &proto.DeleteNoteResponse{Success: true}, nil
}

func (h *NoteHandler) ListNotes(ctx context.Context, req *proto.ListNotesRequest) (*proto.ListNotesResponse, error) {
	notes, err := h.service.ListNotes(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list notes: %v", err)
	}

	var protoNotes []*proto.Note
	for _, n := range notes {
		protoNotes = append(protoNotes, mapToProto(n))
	}
	return &proto.ListNotesResponse{Notes: protoNotes}, nil
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
