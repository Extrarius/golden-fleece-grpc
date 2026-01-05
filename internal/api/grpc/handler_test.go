package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"notes-service/internal/model"
	"notes-service/internal/repository/memory"
	notesv1 "notes-service/pkg/proto/notes/v1"
)

// mockNoteService - мок сервиса для тестирования handler
type mockNoteService struct {
	createFunc func(ctx context.Context, title, content string) (model.Note, error)
	getFunc    func(ctx context.Context, id string) (model.Note, error)
	listFunc   func(ctx context.Context) ([]model.Note, error)
	updateFunc func(ctx context.Context, id, title, content string) (model.Note, error)
	deleteFunc func(ctx context.Context, id string) error
}

func (m *mockNoteService) Create(ctx context.Context, title, content string) (model.Note, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, title, content)
	}
	return model.Note{}, nil
}

func (m *mockNoteService) Get(ctx context.Context, id string) (model.Note, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return model.Note{}, nil
}

func (m *mockNoteService) List(ctx context.Context) ([]model.Note, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	return nil, nil
}

func (m *mockNoteService) Update(ctx context.Context, id, title, content string) (model.Note, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, title, content)
	}
	return model.Note{}, nil
}

func (m *mockNoteService) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func TestGetNote_NotFoundWithDetails(t *testing.T) {
	// Arrange
	ctx := context.Background()
	noteID := "non-existent-id"

	mockService := &mockNoteService{
		getFunc: func(ctx context.Context, id string) (model.Note, error) {
			return model.Note{}, memory.ErrNoteNotFound
		},
	}

	handler := NewHandler(mockService)

	// Act
	_, err := handler.GetNote(ctx, &notesv1.GetNoteRequest{Id: noteID})

	// Assert
	require.Error(t, err, "Expected error for non-existent note")

	st := status.Convert(err)
	assert.Equal(t, codes.NotFound, st.Code(), "Expected NotFound status code")
	assert.Contains(t, st.Message(), "note not found", "Expected error message to contain 'note not found'")

	// Проверка Details
	require.Len(t, st.Details(), 1, "Expected exactly one detail in error")

	errorDetails, ok := st.Details()[0].(*notesv1.ErrorDetails)
	require.True(t, ok, "Expected detail to be of type ErrorDetails")

	assert.Contains(t, errorDetails.Reason, "was searched but not found", "Expected reason to contain 'was searched but not found'")
	assert.Contains(t, errorDetails.Reason, noteID, "Expected reason to contain the note ID")
	assert.Equal(t, noteID, errorDetails.NoteId, "Expected NoteId to match requested ID")
}

func TestGetNote_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	noteID := "test-id-123"

	expectedNote := model.Note{
		ID:        noteID,
		Title:     "Test Title",
		Content:   "Test Content",
	}

	mockService := &mockNoteService{
		getFunc: func(ctx context.Context, id string) (model.Note, error) {
			if id == noteID {
				return expectedNote, nil
			}
			return model.Note{}, memory.ErrNoteNotFound
		},
	}

	handler := NewHandler(mockService)

	// Act
	resp, err := handler.GetNote(ctx, &notesv1.GetNoteRequest{Id: noteID})

	// Assert
	require.NoError(t, err, "Expected no error for existing note")
	require.NotNil(t, resp, "Expected non-nil response")
	require.NotNil(t, resp.Note, "Expected non-nil note in response")
	assert.Equal(t, noteID, resp.Note.Id, "Expected note ID to match")
	assert.Equal(t, expectedNote.Title, resp.Note.Title, "Expected note title to match")
	assert.Equal(t, expectedNote.Content, resp.Note.Content, "Expected note content to match")
}

func TestHandleError_NotFound(t *testing.T) {
	// Arrange
	err := memory.ErrNoteNotFound

	// Act
	grpcErr := handleError(err)

	// Assert
	require.Error(t, grpcErr, "Expected error")

	st := status.Convert(grpcErr)
	assert.Equal(t, codes.NotFound, st.Code(), "Expected NotFound status code")

	// Проверка Details
	require.Len(t, st.Details(), 1, "Expected exactly one detail in error")

	errorDetails, ok := st.Details()[0].(*notesv1.ErrorDetails)
	require.True(t, ok, "Expected detail to be of type ErrorDetails")

	assert.Contains(t, errorDetails.Reason, "not found in the database", "Expected reason to contain 'not found in the database'")
	assert.Equal(t, "NOTE_NOT_FOUND", errorDetails.InternalErrorCode, "Expected internal error code to be 'NOTE_NOT_FOUND'")
}

func TestHandleError_ValidationError(t *testing.T) {
	// Arrange
	err := errors.New("title cannot be empty")

	// Act
	grpcErr := handleError(err)

	// Assert
	require.Error(t, grpcErr, "Expected error")

	st := status.Convert(grpcErr)
	assert.Equal(t, codes.InvalidArgument, st.Code(), "Expected InvalidArgument status code")

	// Проверка Details
	require.Len(t, st.Details(), 1, "Expected exactly one detail in error")

	errorDetails, ok := st.Details()[0].(*notesv1.ErrorDetails)
	require.True(t, ok, "Expected detail to be of type ErrorDetails")

	assert.Contains(t, errorDetails.Reason, "Title field validation failed", "Expected reason to contain validation failure message")
	assert.Equal(t, "VALIDATION_ERROR", errorDetails.InternalErrorCode, "Expected internal error code to be 'VALIDATION_ERROR'")
}

func TestHandleError_InternalError(t *testing.T) {
	// Arrange
	err := errors.New("some internal error")

	// Act
	grpcErr := handleError(err)

	// Assert
	require.Error(t, grpcErr, "Expected error")

	st := status.Convert(grpcErr)
	assert.Equal(t, codes.Internal, st.Code(), "Expected Internal status code")

	// Проверка Details
	require.Len(t, st.Details(), 1, "Expected exactly one detail in error")

	errorDetails, ok := st.Details()[0].(*notesv1.ErrorDetails)
	require.True(t, ok, "Expected detail to be of type ErrorDetails")

	assert.Contains(t, errorDetails.Reason, "internal error occurred", "Expected reason to contain internal error message")
	assert.Equal(t, "INTERNAL_ERROR", errorDetails.InternalErrorCode, "Expected internal error code to be 'INTERNAL_ERROR'")
}

