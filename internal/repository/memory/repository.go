package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"notes-service/internal/model"
	"notes-service/internal/repository"

	"github.com/google/uuid"
)

// ErrNoteNotFound возвращается, когда заметка не найдена
var ErrNoteNotFound = errors.New("note not found")

var _ repository.NoteRepository = (*repo)(nil)

type repo struct {
	mu    sync.RWMutex
	notes map[string]model.Note
}

// NewRepository создает новый экземпляр in-memory репозитория на основе map
func NewRepository() repository.NoteRepository {
	return &repo{
		notes: make(map[string]model.Note),
	}
}

// Create создает новую заметку и возвращает созданную заметку с ID
func (r *repo) Create(ctx context.Context, note model.Note) (model.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Генерируем UUID если не передан
	if note.ID == "" {
		note.ID = uuid.New().String()
	}

	// Устанавливаем временные метки
	now := time.Now()
	if note.CreatedAt.IsZero() {
		note.CreatedAt = now
	}
	note.UpdatedAt = now

	// Сохраняем заметку
	r.notes[note.ID] = note

	return note, nil
}

// GetByID возвращает заметку по её ID
func (r *repo) GetByID(ctx context.Context, id string) (model.Note, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	note, exists := r.notes[id]
	if !exists {
		return model.Note{}, ErrNoteNotFound
	}

	return note, nil
}

// List возвращает список всех заметок
func (r *repo) List(ctx context.Context) ([]model.Note, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	notes := make([]model.Note, 0, len(r.notes))
	for _, note := range r.notes {
		notes = append(notes, note)
	}

	return notes, nil
}

// Update обновляет существующую заметку и возвращает обновленную заметку
func (r *repo) Update(ctx context.Context, note model.Note) (model.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем существование заметки
	_, exists := r.notes[note.ID]
	if !exists {
		return model.Note{}, ErrNoteNotFound
	}

	// Обновляем временную метку
	note.UpdatedAt = time.Now()

	// Сохраняем обновленную заметку
	r.notes[note.ID] = note

	return note, nil
}

// Delete удаляет заметку по ID
func (r *repo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем существование заметки
	_, exists := r.notes[id]
	if !exists {
		return ErrNoteNotFound
	}

	delete(r.notes, id)

	return nil
}
