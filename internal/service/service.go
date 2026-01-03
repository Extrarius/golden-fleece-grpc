package service

import (
	"context"

	"notes-service/internal/model"
)

// NoteService интерфейс для бизнес-логики работы с заметками
type NoteService interface {
	// Create создает новую заметку с указанными title и content
	Create(ctx context.Context, title, content string) (model.Note, error)

	// Get возвращает заметку по её ID
	Get(ctx context.Context, id string) (model.Note, error)

	// List возвращает список всех заметок
	List(ctx context.Context) ([]model.Note, error)

	// Update обновляет заметку с указанным ID (title и content опциональны)
	Update(ctx context.Context, id, title, content string) (model.Note, error)

	// Delete удаляет заметку по ID
	Delete(ctx context.Context, id string) error
}
