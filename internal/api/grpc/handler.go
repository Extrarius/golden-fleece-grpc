package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"notes-service/internal/converter"
	"notes-service/internal/repository/memory"
	svc "notes-service/internal/service"
	notesv1 "notes-service/pkg/proto/notes/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handler реализует gRPC сервер для NotesService
type Handler struct {
	notesv1.UnimplementedNotesServiceServer

	noteService svc.NoteService
}

// NewHandler создает новый экземпляр gRPC хэндлера
func NewHandler(noteService svc.NoteService) *Handler {
	return &Handler{
		noteService: noteService,
	}
}

// CreateNote создает новую заметку
func (h *Handler) CreateNote(ctx context.Context, req *notesv1.CreateNoteRequest) (*notesv1.CreateNoteResponse, error) {
	// Вызываем бизнес-логику
	note, err := h.noteService.Create(ctx, req.GetTitle(), req.GetContent())
	if err != nil {
		return nil, handleError(err)
	}

	// Конвертируем domain модель в proto
	protoNote := converter.ModelToProto(note)

	return &notesv1.CreateNoteResponse{
		Note: protoNote,
	}, nil
}

// GetNote возвращает заметку по её UUID
func (h *Handler) GetNote(ctx context.Context, req *notesv1.GetNoteRequest) (*notesv1.GetNoteResponse, error) {
	// Вызываем бизнес-логику
	note, err := h.noteService.Get(ctx, req.GetId())
	if err != nil {
		// Если заметка не найдена, возвращаем детализированную ошибку
		if errors.Is(err, memory.ErrNoteNotFound) {
			st := status.New(codes.NotFound, "note not found")
			errorDetails := &notesv1.ErrorDetails{
				Reason: fmt.Sprintf("Note with ID %s was searched but not found in DB", req.GetId()),
				NoteId: req.GetId(),
			}
			st, errWithDetails := st.WithDetails(errorDetails)
			if errWithDetails != nil {
				// Если не удалось добавить Details, просто возвращаем ошибку без деталей
				return nil, status.Errorf(codes.NotFound, "note not found: %v", err)
			}
			return nil, st.Err()
		}
		return nil, handleError(err)
	}

	// Конвертируем domain модель в proto
	protoNote := converter.ModelToProto(note)

	return &notesv1.GetNoteResponse{
		Note: protoNote,
	}, nil
}

// ListNotes возвращает список всех заметок
func (h *Handler) ListNotes(ctx context.Context, req *notesv1.ListNotesRequest) (*notesv1.ListNotesResponse, error) {
	// Вызываем бизнес-логику
	notes, err := h.noteService.List(ctx)
	if err != nil {
		return nil, handleError(err)
	}

	// Конвертируем domain модели в proto
	protoNotes := converter.ModelsToProtos(notes)

	return &notesv1.ListNotesResponse{
		Notes: protoNotes,
	}, nil
}

// UpdateNote обновляет существующую заметку
func (h *Handler) UpdateNote(ctx context.Context, req *notesv1.UpdateNoteRequest) (*notesv1.UpdateNoteResponse, error) {
	// Вызываем бизнес-логику
	note, err := h.noteService.Update(ctx, req.GetId(), req.GetTitle(), req.GetContent())
	if err != nil {
		return nil, handleError(err)
	}

	// Конвертируем domain модель в proto
	protoNote := converter.ModelToProto(note)

	return &notesv1.UpdateNoteResponse{
		Note: protoNote,
	}, nil
}

// DeleteNote удаляет заметку по UUID
func (h *Handler) DeleteNote(ctx context.Context, req *notesv1.DeleteNoteRequest) (*notesv1.DeleteNoteResponse, error) {
	// Вызываем бизнес-логику
	err := h.noteService.Delete(ctx, req.GetId())
	if err != nil {
		return nil, handleError(err)
	}

	return &notesv1.DeleteNoteResponse{}, nil
}

// handleError конвертирует внутренние ошибки в gRPC статусы с детализацией
func handleError(err error) error {
	if err == nil {
		return nil
	}

	// Проверяем специфичные ошибки репозитория
	if errors.Is(err, memory.ErrNoteNotFound) {
		st := status.New(codes.NotFound, "note not found")
		errorDetails := &notesv1.ErrorDetails{
			Reason:            "The requested note was not found in the database",
			InternalErrorCode: "NOTE_NOT_FOUND",
		}
		st, _ = st.WithDetails(errorDetails)
		return st.Err()
	}

	// Проверяем ошибки валидации (содержат "cannot be empty")
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "cannot be empty") || strings.Contains(errMsg, "invalid") {
		st := status.New(codes.InvalidArgument, err.Error())
		errorDetails := &notesv1.ErrorDetails{
			Reason:            fmt.Sprintf("Validation failed: %s", err.Error()),
			InternalErrorCode: "VALIDATION_ERROR",
		}
		// Попытаемся извлечь поле из сообщения об ошибке
		if strings.Contains(errMsg, "title") {
			errorDetails.Reason = "Title field validation failed: " + err.Error()
		} else if strings.Contains(errMsg, "id") {
			errorDetails.Reason = "ID field validation failed: " + err.Error()
		}
		st, _ = st.WithDetails(errorDetails)
		return st.Err()
	}

	// Все остальные ошибки - Internal
	st := status.New(codes.Internal, "internal error")
	errorDetails := &notesv1.ErrorDetails{
		Reason:            fmt.Sprintf("An internal error occurred: %v", err),
		InternalErrorCode: "INTERNAL_ERROR",
	}
	st, _ = st.WithDetails(errorDetails)
	return st.Err()
}
