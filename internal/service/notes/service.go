package notes

import (
	"context"
	"errors"
	"strings"
	"time"

	"notes-service/internal/model"
	"notes-service/internal/repository"
	svc "notes-service/internal/service"
)

var _ svc.NoteService = (*service)(nil)

type service struct {
	noteRepository repository.NoteRepository
}

// NewNoteService создает новый экземпляр сервиса для работы с заметками
func NewNoteService(noteRepository repository.NoteRepository) svc.NoteService {
	return &service{
		noteRepository: noteRepository,
	}
}

// Create создает новую заметку с указанными title и content
func (s *service) Create(ctx context.Context, title, content string) (model.Note, error) {
	// Валидация: title не должен быть пустым
	title = strings.TrimSpace(title)
	if title == "" {
		return model.Note{}, errors.New("title cannot be empty")
	}

	// Создаем новую заметку
	note := model.Note{
		Title:     title,
		Content:   strings.TrimSpace(content),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Сохраняем через репозиторий (UUID будет сгенерирован в репозитории)
	createdNote, err := s.noteRepository.Create(ctx, note)
	if err != nil {
		return model.Note{}, err
	}

	return createdNote, nil
}

// Get возвращает заметку по её ID
func (s *service) Get(ctx context.Context, id string) (model.Note, error) {
	if id == "" {
		return model.Note{}, errors.New("id cannot be empty")
	}

	note, err := s.noteRepository.GetByID(ctx, id)
	if err != nil {
		return model.Note{}, err
	}

	return note, nil
}

// List возвращает список всех заметок
func (s *service) List(ctx context.Context) ([]model.Note, error) {
	notes, err := s.noteRepository.List(ctx)
	if err != nil {
		return nil, err
	}

	return notes, nil
}

// Update обновляет заметку с указанным ID (title и content опциональны)
func (s *service) Update(ctx context.Context, id, title, content string) (model.Note, error) {
	if id == "" {
		return model.Note{}, errors.New("id cannot be empty")
	}

	// Получаем существующую заметку
	existingNote, err := s.noteRepository.GetByID(ctx, id)
	if err != nil {
		return model.Note{}, err
	}

	// Обновляем поля только если они переданы (не пустые после TrimSpace)
	titleTrimmed := strings.TrimSpace(title)
	if titleTrimmed != "" {
		existingNote.Title = titleTrimmed
	}

	// Content всегда обновляется, даже если пустой
	existingNote.Content = strings.TrimSpace(content)

	// Валидация обновленной заметки
	if err := existingNote.Validate(); err != nil {
		return model.Note{}, err
	}

	// Обновляем временную метку
	existingNote.UpdatedAt = time.Now()

	// Сохраняем через репозиторий
	updatedNote, err := s.noteRepository.Update(ctx, existingNote)
	if err != nil {
		return model.Note{}, err
	}

	return updatedNote, nil
}

// Delete удаляет заметку по ID
func (s *service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	err := s.noteRepository.Delete(ctx, id)
	if err != nil {
		return err
	}

	return nil
}
