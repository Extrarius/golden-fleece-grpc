package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"notes-service/internal/converter"
	"notes-service/internal/model"
	"notes-service/internal/repository/memory"
	svc "notes-service/internal/service"
	notesv1 "notes-service/pkg/proto/notes/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// eventServiceProvider интерфейс для доступа к EventService
type eventServiceProvider interface {
	GetEventService() interface {
		Subscribe() chan model.Note
		Unsubscribe(chan model.Note)
		Publish(model.Note)
	}
}

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

// SubscribeToEvents подписывается на события создания заметок (server-side streaming)
func (h *Handler) SubscribeToEvents(req *notesv1.SubscribeToEventsRequest, stream notesv1.NotesService_SubscribeToEventsServer) error {
	// 1. Получаем EventService из noteService через интерфейс
	provider, ok := h.noteService.(eventServiceProvider)
	if !ok {
		return status.Errorf(codes.Internal, "event service not available")
	}

	eventService := provider.GetEventService()
	eventCh := eventService.Subscribe()
	defer eventService.Unsubscribe(eventCh)

	// 2. Отправить приветственное сообщение (health-check) сразу после подключения
	if err := stream.Send(&notesv1.EventResponse{
		Event: &notesv1.EventResponse_HealthCheck{
			HealthCheck: &notesv1.HealthCheck{
				Message:   "Connected to events stream",
				Timestamp: timestamppb.Now(),
			},
		},
	}); err != nil {
		return err
	}

	// 3. Запустить горутину для периодических health-check сообщений
	ctx := stream.Context()
	ticker := time.NewTicker(30 * time.Second) // Отправляем health-check каждые 30 секунд
	defer ticker.Stop()

	healthCheckErrChan := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := stream.Send(&notesv1.EventResponse{
					Event: &notesv1.EventResponse_HealthCheck{
						HealthCheck: &notesv1.HealthCheck{
							Message:   "Health check",
							Timestamp: timestamppb.Now(),
						},
					},
				}); err != nil {
					healthCheckErrChan <- err
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 4. Основной цикл обработки событий
	for {
		select {
		case note := <-eventCh:
			// note уже имеет тип model.Note из канала
			// Конвертируем в proto и отправляем событие
			protoNote := converter.ModelToProto(note)
			if err := stream.Send(&notesv1.EventResponse{
				Event: &notesv1.EventResponse_NoteCreated{
					NoteCreated: &notesv1.NoteCreatedEvent{
						NoteId: note.ID,
						Note:   protoNote,
					},
				},
			}); err != nil {
				return err
			}

		case err := <-healthCheckErrChan:
			return err
		case <-ctx.Done():
			// Клиент отключился
			log.Printf("Client disconnected from events stream")
			return nil
		}
	}
}

// UploadMetrics обрабатывает client-side streaming - загрузку метрик
func (h *Handler) UploadMetrics(stream notesv1.NotesService_UploadMetricsServer) error {
	var sum float64
	var count int64

	log.Println("Starting to receive metrics stream...")

	// Читаем метрики из стрима до io.EOF
	for {
		metric, err := stream.Recv()
		if err == io.EOF {
			// Клиент завершил отправку, вычисляем результат
			average := float64(0)
			if count > 0 {
				average = sum / float64(count)
			}

			log.Printf("Received all metrics: count=%d, sum=%.2f, average=%.2f", count, sum, average)

			// Отправляем финальный ответ
			if err := stream.SendAndClose(&notesv1.SummaryResponse{
				Sum:     sum,
				Average: average,
				Count:   count,
			}); err != nil {
				log.Printf("Error sending summary response: %v", err)
				return err
			}

			log.Println("Successfully sent summary response")
			return nil
		}
		if err != nil {
			log.Printf("Error receiving metric: %v", err)
			return err
		}

		// Накопление данных
		sum += metric.GetValue()
		count++

		log.Printf("Received metric: name=%s, value=%.2f (count=%d)",
			metric.GetName(), metric.GetValue(), count)
	}
}

// Chat обрабатывает bidirectional streaming - асинхронный чат с correlation ID
func (h *Handler) Chat(stream notesv1.NotesService_ChatServer) error {
	ctx := stream.Context()
	errChan := make(chan error, 2)
	var wg sync.WaitGroup

	log.Println("Chat stream established")

	// Горутина для чтения сообщений от клиента
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("Client closed send stream")
				return
			}
			if err != nil {
				errChan <- fmt.Errorf("error receiving message: %w", err)
				return
			}

			correlationID := msg.GetCorrelationId()

			// Обработка входящего сообщения через one-of
			switch content := msg.GetContent().(type) {
			case *notesv1.ChatMessage_TextMessage:
				// Получено текстовое сообщение
				text := content.TextMessage.GetText()
				log.Printf("📥 Received text message: correlation_id=%s, text=%s",
					correlationID, text)

				// Валидация: если текст пустой, отправляем бизнесовую ошибку через one-of
				if strings.TrimSpace(text) == "" {
					errorResponse := &notesv1.ChatMessage{
						CorrelationId: correlationID,
						Content: &notesv1.ChatMessage_Error{
							Error: &notesv1.ChatError{
								Code:    "VALIDATION_ERROR",
								Message: "Message text cannot be empty",
								Details: "The text field must contain at least one non-whitespace character",
							},
						},
					}

					if err := stream.Send(errorResponse); err != nil {
						errChan <- fmt.Errorf("error sending validation error: %w", err)
						return
					}

					log.Printf("📤 Sent validation error: correlation_id=%s", correlationID)
					continue // Продолжаем работу, соединение не разрывается
				}

				// Отправить подтверждение с тем же correlation_id через text_message
				response := &notesv1.ChatMessage{
					CorrelationId: correlationID,
					Content: &notesv1.ChatMessage_TextMessage{
						TextMessage: &notesv1.ChatTextMessage{
							Text:      fmt.Sprintf("Acknowledged: %s", text),
							Timestamp: timestamppb.Now(),
						},
					},
				}

				if err := stream.Send(response); err != nil {
					errChan <- fmt.Errorf("error sending acknowledgment: %w", err)
					return
				}

				log.Printf("📤 Sent acknowledgment: correlation_id=%s", correlationID)

			case *notesv1.ChatMessage_Error:
				// Получена ошибка от клиента (если клиент отправляет ошибки)
				log.Printf("📥 Received error from client: correlation_id=%s, code=%s, message=%s",
					correlationID, content.Error.GetCode(), content.Error.GetMessage())
				// Можно обработать ошибку от клиента, но обычно клиент не отправляет ошибки

			case nil:
				// Content не установлен (старое сообщение или ошибка десериализации)
				log.Printf("⚠️ Received message without content: correlation_id=%s", correlationID)
				errorResponse := &notesv1.ChatMessage{
					CorrelationId: correlationID,
					Content: &notesv1.ChatMessage_Error{
						Error: &notesv1.ChatError{
							Code:    "INVALID_MESSAGE",
							Message: "Message content is missing",
							Details: "The message must contain either text_message or error",
						},
					},
				}

				if err := stream.Send(errorResponse); err != nil {
					errChan <- fmt.Errorf("error sending invalid message error: %w", err)
					return
				}
			}

			// Проверка отмены контекста
			select {
			case <-ctx.Done():
				log.Println("Context cancelled in receive goroutine")
				return
			default:
			}
		}
	}()

	// Горутина для отправки независимых уведомлений
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		notificationCounter := int64(0)

		for {
			select {
			case <-ticker.C:
				notificationCounter++
				notification := &notesv1.ChatMessage{
					CorrelationId: fmt.Sprintf("notification-%d", notificationCounter),
					Content: &notesv1.ChatMessage_TextMessage{
						TextMessage: &notesv1.ChatTextMessage{
							Text:      fmt.Sprintf("Server notification #%d", notificationCounter),
							Timestamp: timestamppb.Now(),
						},
					},
				}

				if err := stream.Send(notification); err != nil {
					errChan <- fmt.Errorf("error sending notification: %w", err)
					return
				}

				log.Printf("📤 Sent notification: correlation_id=%s", notification.GetCorrelationId())

			case <-ctx.Done():
				log.Println("Context cancelled in send goroutine")
				return
			}
		}
	}()

	// Ожидание завершения горутин или ошибки
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Вернуть первую ошибку, если она есть, или nil при нормальном завершении
	if err, ok := <-errChan; ok {
		log.Printf("Chat stream error: %v", err)
		return err
	}

	log.Println("Chat stream completed successfully")
	return nil
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
