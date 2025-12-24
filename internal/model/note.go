package model

import (
	"errors"
	"strings"
	"time"
)

// Note представляет заметку (доменная модель)
type Note struct {
	ID        string    // UUID заметки
	Title     string    // Заголовок заметки
	Content   string    // Содержание заметки
	CreatedAt time.Time // Дата создания
	UpdatedAt time.Time // Дата последнего обновления
}

// Validate проверяет валидность заметки
func (n *Note) Validate() error {
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("title cannot be empty")
	}
	return nil
}

// IsEmpty проверяет, пуста ли заметка
func (n *Note) IsEmpty() bool {
	return n.ID == "" && n.Title == "" && n.Content == ""
}
