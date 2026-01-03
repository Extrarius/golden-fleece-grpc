package repository

import (
	"context"

	"notes-service/internal/model"
)

// NoteRepository интерфейс для работы с заметками в хранилище
type NoteRepository interface {
	// Create создает новую заметку и возвращает созданную заметку с ID
	Create(ctx context.Context, note model.Note) (model.Note, error)

	// GetByID возвращает заметку по её ID
	GetByID(ctx context.Context, id string) (model.Note, error)

	// List возвращает список всех заметок
	List(ctx context.Context) ([]model.Note, error)

	// Update обновляет существующую заметку и возвращает обновленную заметку
	Update(ctx context.Context, note model.Note) (model.Note, error)

	// Delete удаляет заметку по ID
	Delete(ctx context.Context, id string) error
}
