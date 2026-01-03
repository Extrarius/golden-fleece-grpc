package notes

import (
	"context"
	"errors"
	"testing"
	"time"

	"notes-service/internal/model"
	"notes-service/internal/repository"
	"notes-service/internal/repository/memory"
)

// mockRepository - простой mock репозитория для тестирования
type mockRepository struct {
	notes          map[string]model.Note
	createError    error
	getByIDError   error
	listError      error
	updateError    error
	deleteError    error
	shouldFailGet  bool
	shouldFailList bool
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		notes: make(map[string]model.Note),
	}
}

func (m *mockRepository) Create(ctx context.Context, note model.Note) (model.Note, error) {
	if m.createError != nil {
		return model.Note{}, m.createError
	}

	// Генерируем ID если его нет (для тестов)
	if note.ID == "" {
		note.ID = "test-id-" + time.Now().Format("20060102150405")
	}

	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()
	m.notes[note.ID] = note
	return note, nil
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (model.Note, error) {
	if m.getByIDError != nil {
		return model.Note{}, m.getByIDError
	}

	if m.shouldFailGet {
		return model.Note{}, memory.ErrNoteNotFound
	}

	note, exists := m.notes[id]
	if !exists {
		return model.Note{}, memory.ErrNoteNotFound
	}

	return note, nil
}

func (m *mockRepository) List(ctx context.Context) ([]model.Note, error) {
	if m.listError != nil {
		return nil, m.listError
	}

	if m.shouldFailList {
		return nil, errors.New("list error")
	}

	notes := make([]model.Note, 0, len(m.notes))
	for _, note := range m.notes {
		notes = append(notes, note)
	}

	return notes, nil
}

func (m *mockRepository) Update(ctx context.Context, note model.Note) (model.Note, error) {
	if m.updateError != nil {
		return model.Note{}, m.updateError
	}

	if _, exists := m.notes[note.ID]; !exists {
		return model.Note{}, memory.ErrNoteNotFound
	}

	note.UpdatedAt = time.Now()
	m.notes[note.ID] = note
	return note, nil
}

func (m *mockRepository) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}

	if _, exists := m.notes[id]; !exists {
		return memory.ErrNoteNotFound
	}

	delete(m.notes, id)
	return nil
}

// Проверяем, что mockRepository реализует интерфейс
var _ repository.NoteRepository = (*mockRepository)(nil)

func TestNoteService_Create_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	title := "Test Note"
	content := "Test Content"

	note, err := service.Create(ctx, title, content)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if note.Title != title {
		t.Errorf("Expected title %q, got %q", title, note.Title)
	}

	if note.Content != content {
		t.Errorf("Expected content %q, got %q", content, note.Content)
	}

	if note.ID == "" {
		t.Error("Expected note to have ID")
	}

	if note.CreatedAt.IsZero() {
		t.Error("Expected note to have CreatedAt")
	}

	if note.UpdatedAt.IsZero() {
		t.Error("Expected note to have UpdatedAt")
	}
}

func TestNoteService_Create_EmptyTitle(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	note, err := service.Create(ctx, "", "content")

	if err == nil {
		t.Error("Expected error for empty title")
	}

	if !note.IsEmpty() {
		t.Error("Expected empty note on error")
	}

	if err.Error() != "title cannot be empty" {
		t.Errorf("Expected 'title cannot be empty', got: %v", err)
	}
}

func TestNoteService_Create_WhitespaceTitle(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	note, err := service.Create(ctx, "   ", "content")

	if err == nil {
		t.Error("Expected error for whitespace-only title")
	}

	if !note.IsEmpty() {
		t.Error("Expected empty note on error")
	}
}

func TestNoteService_Create_TrimsContent(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	title := "Test Note"
	content := "  Test Content  "

	note, err := service.Create(ctx, title, content)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if note.Content != "Test Content" {
		t.Errorf("Expected trimmed content, got: %q", note.Content)
	}
}

func TestNoteService_Get_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем заметку через mock напрямую для подготовки данных
	testNote := model.Note{
		ID:        "test-id",
		Title:     "Test Note",
		Content:   "Test Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["test-id"] = testNote

	note, err := service.Get(ctx, "test-id")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if note.ID != "test-id" {
		t.Errorf("Expected ID %q, got %q", "test-id", note.ID)
	}

	if note.Title != "Test Note" {
		t.Errorf("Expected title %q, got %q", "Test Note", note.Title)
	}
}

func TestNoteService_Get_EmptyID(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	note, err := service.Get(ctx, "")

	if err == nil {
		t.Error("Expected error for empty ID")
	}

	if !note.IsEmpty() {
		t.Error("Expected empty note on error")
	}

	if err.Error() != "id cannot be empty" {
		t.Errorf("Expected 'id cannot be empty', got: %v", err)
	}
}

func TestNoteService_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	note, err := service.Get(ctx, "non-existent-id")

	if err == nil {
		t.Error("Expected error for non-existent note")
	}

	if !note.IsEmpty() {
		t.Error("Expected empty note on error")
	}

	if !errors.Is(err, memory.ErrNoteNotFound) {
		t.Errorf("Expected ErrNoteNotFound, got: %v", err)
	}
}

func TestNoteService_List_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем несколько заметок
	note1 := model.Note{
		ID:        "id-1",
		Title:     "Note 1",
		Content:   "Content 1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	note2 := model.Note{
		ID:        "id-2",
		Title:     "Note 2",
		Content:   "Content 2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["id-1"] = note1
	mockRepo.notes["id-2"] = note2

	notes, err := service.List(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("Expected 2 notes, got %d", len(notes))
	}
}

func TestNoteService_List_Empty(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	notes, err := service.List(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(notes) != 0 {
		t.Errorf("Expected 0 notes, got %d", len(notes))
	}
}

func TestNoteService_Update_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем заметку
	testNote := model.Note{
		ID:        "test-id",
		Title:     "Original Title",
		Content:   "Original Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["test-id"] = testNote

	// Обновляем заметку
	newTitle := "Updated Title"
	newContent := "Updated Content"

	updatedNote, err := service.Update(ctx, "test-id", newTitle, newContent)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if updatedNote.Title != newTitle {
		t.Errorf("Expected title %q, got %q", newTitle, updatedNote.Title)
	}

	if updatedNote.Content != newContent {
		t.Errorf("Expected content %q, got %q", newContent, updatedNote.Content)
	}

	if updatedNote.ID != "test-id" {
		t.Errorf("Expected ID to remain %q, got %q", "test-id", updatedNote.ID)
	}

	if !updatedNote.UpdatedAt.After(testNote.UpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestNoteService_Update_EmptyID(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	note, err := service.Update(ctx, "", "title", "content")

	if err == nil {
		t.Error("Expected error for empty ID")
	}

	if !note.IsEmpty() {
		t.Error("Expected empty note on error")
	}

	if err.Error() != "id cannot be empty" {
		t.Errorf("Expected 'id cannot be empty', got: %v", err)
	}
}

func TestNoteService_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	note, err := service.Update(ctx, "non-existent-id", "title", "content")

	if err == nil {
		t.Error("Expected error for non-existent note")
	}

	if !note.IsEmpty() {
		t.Error("Expected empty note on error")
	}

	if !errors.Is(err, memory.ErrNoteNotFound) {
		t.Errorf("Expected ErrNoteNotFound, got: %v", err)
	}
}

func TestNoteService_Update_PartialUpdate(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем заметку
	testNote := model.Note{
		ID:        "test-id",
		Title:     "Original Title",
		Content:   "Original Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["test-id"] = testNote

	// Обновляем только title, content оставляем пустым
	newTitle := "Updated Title"

	updatedNote, err := service.Update(ctx, "test-id", newTitle, "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if updatedNote.Title != newTitle {
		t.Errorf("Expected title %q, got %q", newTitle, updatedNote.Title)
	}

	// Content должен стать пустым (так как передали пустую строку)
	if updatedNote.Content != "" {
		t.Errorf("Expected empty content, got %q", updatedNote.Content)
	}
}

func TestNoteService_Update_EmptyTitleAfterTrim(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем заметку
	testNote := model.Note{
		ID:        "test-id",
		Title:     "Original Title",
		Content:   "Original Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["test-id"] = testNote

	// Пытаемся обновить с пустым title после trim (только пробелы)
	note, err := service.Update(ctx, "test-id", "   ", "content")
	// Это должно пройти, так как пустой title не обновляется, остается оригинальный
	// Но если мы передадим только пробелы как title и это приведет к пустому title после trim,
	// то валидация должна сработать
	// На самом деле, если title после trim пустой, он не обновляется (строка 90 в service.go)
	// Но тогда мы передаем пустой content, что делает title пустым после валидации
	// Нужно посмотреть на логику: если titleTrimmed == "", то title не обновляется
	// Но если мы передадим title с пробелами, он не обновится, останется старый
	// А если старый title валиден, то ошибки не будет
	// Проверим: если title только пробелы, он не обновляется (остается оригинальный)
	// Но content обновляется на "content"
	// Тогда валидация пройдет, так как оригинальный title валиден
	if err != nil {
		t.Fatalf("Expected no error (whitespace title is not updated), got: %v", err)
	}

	// Title должен остаться оригинальным
	if note.Title != "Original Title" {
		t.Errorf("Expected title to remain 'Original Title', got %q", note.Title)
	}
}

func TestNoteService_Update_OnlyContent(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем заметку
	testNote := model.Note{
		ID:        "test-id",
		Title:     "Original Title",
		Content:   "Original Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["test-id"] = testNote

	// Обновляем только content (передаем пустой title, который не обновится)
	newContent := "Only Content Updated"

	updatedNote, err := service.Update(ctx, "test-id", "", newContent)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Title должен остаться оригинальным
	if updatedNote.Title != "Original Title" {
		t.Errorf("Expected title to remain 'Original Title', got %q", updatedNote.Title)
	}

	// Content должен обновиться
	if updatedNote.Content != newContent {
		t.Errorf("Expected content %q, got %q", newContent, updatedNote.Content)
	}
}

func TestNoteService_Delete_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	// Создаем заметку
	testNote := model.Note{
		ID:        "test-id",
		Title:     "Test Note",
		Content:   "Test Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.notes["test-id"] = testNote

	// Удаляем заметку
	err := service.Delete(ctx, "test-id")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Проверяем, что заметка удалена
	_, exists := mockRepo.notes["test-id"]
	if exists {
		t.Error("Expected note to be deleted")
	}
}

func TestNoteService_Delete_EmptyID(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	err := service.Delete(ctx, "")

	if err == nil {
		t.Error("Expected error for empty ID")
	}

	if err.Error() != "id cannot be empty" {
		t.Errorf("Expected 'id cannot be empty', got: %v", err)
	}
}

func TestNoteService_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := NewNoteService(mockRepo)

	err := service.Delete(ctx, "non-existent-id")

	if err == nil {
		t.Error("Expected error for non-existent note")
	}

	if !errors.Is(err, memory.ErrNoteNotFound) {
		t.Errorf("Expected ErrNoteNotFound, got: %v", err)
	}
}
