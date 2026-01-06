package notes

import (
	"sync"

	"notes-service/internal/model"
)

// EventService управляет подписчиками на события создания заметок
type EventService struct {
	subscribers map[chan model.Note]bool
	mu          sync.RWMutex
}

// NewEventService создает новый экземпляр EventService
func NewEventService() *EventService {
	return &EventService{
		subscribers: make(map[chan model.Note]bool),
	}
}

// Subscribe добавляет нового подписчика и возвращает канал для получения событий
func (s *EventService) Subscribe() chan model.Note {
	ch := make(chan model.Note, 10) // Буферизованный канал для защиты от backpressure
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[ch] = true
	return ch
}

// Unsubscribe удаляет подписчика и закрывает его канал
func (s *EventService) Unsubscribe(ch chan model.Note) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subscribers[ch]; ok {
		close(ch)
		delete(s.subscribers, ch)
	}
}

// Publish отправляет событие всем подписчикам
// Если канал подписчика переполнен, событие пропускается (защита от backpressure)
func (s *EventService) Publish(note model.Note) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subscribers {
		select {
		case ch <- note:
			// Событие успешно отправлено
		default:
			// Канал переполнен, пропускаем (защита от backpressure)
		}
	}
}
