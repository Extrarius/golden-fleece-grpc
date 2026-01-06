package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	notesv1 "notes-service/pkg/proto/notes/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// testSubscribeToEvents тестирует server-side streaming - подписку на события
func testSubscribeToEvents(ctx context.Context, client notesv1.NotesServiceClient) {
	log.Println("\n=== Testing Server-Side Streaming: SubscribeToEvents ===")
	log.Println("Subscribing to events...")

	// Создаем контекст с увеличенным таймаутом для длительного стрима
	streamCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Подписываемся на события
	stream, err := client.SubscribeToEvents(streamCtx, &notesv1.SubscribeToEventsRequest{})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("✅ Successfully subscribed to events stream")
	log.Println("Waiting for events...")

	eventCount := 0
	healthCheckCount := 0
	noteCreatedCount := 0

	// Читаем сообщения из стрима
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			log.Println("\n📡 Stream closed by server (EOF)")
			break
		}
		if err != nil {
			log.Fatalf("Error receiving event: %v", err)
		}

		eventCount++

		// Обрабатываем разные типы событий
		switch event := resp.Event.(type) {
		case *notesv1.EventResponse_HealthCheck:
			healthCheckCount++
			if healthCheckCount == 1 {
				// Первое сообщение - приветственное
				log.Printf("\n✅ Received welcome message: %s", event.HealthCheck.Message)
				if event.HealthCheck.Timestamp != nil {
					log.Printf("   Timestamp: %v", event.HealthCheck.Timestamp.AsTime())
				}
			} else {
				// Последующие - периодические health-check
				log.Printf("💓 Health check #%d: %s", healthCheckCount-1, event.HealthCheck.Message)
			}

		case *notesv1.EventResponse_NoteCreated:
			noteCreatedCount++
			log.Printf("\n🎉 New note created event #%d:", noteCreatedCount)
			log.Printf("   Note ID: %s", event.NoteCreated.NoteId)
			if event.NoteCreated.Note != nil {
				log.Printf("   Title: %s", event.NoteCreated.Note.Title)
				log.Printf("   Content: %s", event.NoteCreated.Note.Content)
				if event.NoteCreated.Note.CreatedAt != nil {
					log.Printf("   Created at: %v", event.NoteCreated.Note.CreatedAt.AsTime())
				}
			}

		default:
			log.Printf("⚠️  Unknown event type: %T", event)
		}
	}

	log.Printf("\n=== Stream Statistics ===")
	log.Printf("Total events received: %d", eventCount)
	log.Printf("Health checks: %d", healthCheckCount)
	log.Printf("Note created events: %d", noteCreatedCount)
}

// testUploadMetrics тестирует client-side streaming - загрузку метрик
func testUploadMetrics(ctx context.Context, client notesv1.NotesServiceClient) {
	log.Println("\n=== Testing Client-Side Streaming: UploadMetrics ===")
	log.Println("Uploading metrics...")

	// Создаем стрим для отправки метрик
	stream, err := client.UploadMetrics(ctx)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	log.Println("✅ Successfully created upload stream")

	// Отправляем несколько метрик
	metrics := []float64{10.5, 20.3, 15.7, 30.2, 25.1}
	for i, value := range metrics {
		metric := &notesv1.MetricRequest{
			Value: value,
			Name:  fmt.Sprintf("metric_%d", i+1),
		}

		if err := stream.Send(metric); err != nil {
			log.Fatalf("Failed to send metric: %v", err)
		}

		log.Printf("📤 Sent metric: %s = %.2f", metric.Name, metric.Value)

		// Имитация задержки между отправками
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("\n✅ Finished sending all metrics")

	// Завершаем отправку и получаем результат
	summary, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Failed to receive summary: %v", err)
	}

	log.Printf("\n✅ Summary received:")
	log.Printf("   Sum:     %.2f", summary.Sum)
	log.Printf("   Average: %.2f", summary.Average)
	log.Printf("   Count:   %d", summary.Count)

	// Проверяем корректность вычислений
	expectedSum := 10.5 + 20.3 + 15.7 + 30.2 + 25.1
	expectedAverage := expectedSum / float64(len(metrics))
	if summary.Count == int64(len(metrics)) &&
		summary.Sum == expectedSum &&
		summary.Average == expectedAverage {
		log.Println("\n✅ All calculations are correct!")
	} else {
		log.Printf("\n⚠️  Calculation mismatch: expected sum=%.2f, got %.2f", expectedSum, summary.Sum)
	}
}

// testUploadMetricsEmpty тестирует client-side streaming с пустым стримом
func testUploadMetricsEmpty(ctx context.Context, client notesv1.NotesServiceClient) {
	log.Println("\n=== Testing Client-Side Streaming: UploadMetrics (Empty Stream) ===")
	log.Println("Testing empty metrics stream...")

	// Создаем стрим для отправки метрик
	stream, err := client.UploadMetrics(ctx)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	log.Println("✅ Successfully created upload stream")
	log.Println("📤 Not sending any metrics...")

	// Немедленно завершаем отправку без отправки метрик
	summary, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Failed to receive summary: %v", err)
	}

	log.Printf("\n✅ Summary received for empty stream:")
	log.Printf("   Sum:     %.2f", summary.Sum)
	log.Printf("   Average: %.2f", summary.Average)
	log.Printf("   Count:   %d", summary.Count)

	// Проверяем, что для пустого стрима count = 0
	if summary.Count == 0 && summary.Sum == 0 && summary.Average == 0 {
		log.Println("\n✅ Empty stream handled correctly!")
	} else {
		log.Printf("\n⚠️  Unexpected values for empty stream")
	}
}

// testChat тестирует bidirectional streaming - асинхронный чат
func testChat(ctx context.Context, client notesv1.NotesServiceClient) {
	log.Println("\n=== Testing Bidirectional Streaming: Chat ===")
	log.Println("Starting chat...")

	// Создаем контекст с увеличенным таймаутом для длительного чата
	chatCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Создаем bidirectional стрим
	stream, err := client.Chat(chatCtx)
	if err != nil {
		log.Fatalf("Failed to create chat stream: %v", err)
	}

	log.Println("✅ Successfully created chat stream")

	errChan := make(chan error, 2)
	var wg sync.WaitGroup

	receivedCount := 0
	sentCount := 0
	acknowledgedCount := 0
	notificationCount := 0

	// Горутина для чтения сообщений от сервера
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("📡 Server closed stream (EOF)")
				return
			}
			if err != nil {
				errChan <- fmt.Errorf("error receiving message: %w", err)
				return
			}

			receivedCount++

			correlationID := msg.GetCorrelationId()

			// Обработка входящего сообщения через one-of
			switch content := msg.GetContent().(type) {
			case *notesv1.ChatMessage_TextMessage:
				// Получено текстовое сообщение
				text := content.TextMessage.GetText()
				timestamp := content.TextMessage.GetTimestamp()

				// Проверяем тип сообщения по correlation_id
				if len(correlationID) > 0 && correlationID[:4] == "noti" {
					// Это уведомление от сервера
					notificationCount++
					log.Printf("📨 Received notification #%d: correlation_id=%s, text=%s, timestamp=%v",
						notificationCount, correlationID, text, timestamp)
				} else if len(correlationID) > 0 && correlationID[:6] == "client" {
					// Это подтверждение нашего сообщения
					acknowledgedCount++
					log.Printf("✅ Received acknowledgment #%d: correlation_id=%s, text=%s, timestamp=%v",
						acknowledgedCount, correlationID, text, timestamp)
				} else {
					log.Printf("📨 Received text message: correlation_id=%s, text=%s, timestamp=%v",
						correlationID, text, timestamp)
				}

			case *notesv1.ChatMessage_Error:
				// Получена бизнесовая ошибка от сервера (не разрывающая соединение)
				errorMsg := content.Error
				log.Printf("❌ Received error: correlation_id=%s, code=%s, message=%s, details=%s",
					correlationID, errorMsg.GetCode(), errorMsg.GetMessage(), errorMsg.GetDetails())
				// Обработка ошибки без разрыва соединения - продолжаем работу
				// Можно добавить логику обработки конкретных типов ошибок

			case nil:
				// Content не установлен
				log.Printf("⚠️ Received message without content: correlation_id=%s", correlationID)
			}

			// Проверка отмены контекста
			select {
			case <-chatCtx.Done():
				log.Println("Context cancelled in receive goroutine")
				return
			default:
			}
		}
	}()

	// Горутина для отправки сообщений на сервер
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Включаем одно пустое сообщение для тестирования валидации ошибок
		messages := []string{"Hello", "How are you?", "", "Test message"}
		for i, text := range messages {
			correlationID := fmt.Sprintf("client-msg-%d", i+1)
			msg := &notesv1.ChatMessage{
				CorrelationId: correlationID,
				Content: &notesv1.ChatMessage_TextMessage{
					TextMessage: &notesv1.ChatTextMessage{
						Text:      text,
						Timestamp: timestamppb.Now(),
					},
				},
			}

			if err := stream.Send(msg); err != nil {
				errChan <- fmt.Errorf("error sending message: %w", err)
				return
			}

			sentCount++
			if text == "" {
				log.Printf("📤 Sent message #%d: correlation_id=%s, text='' (empty - testing validation)",
					sentCount, correlationID)
			} else {
				log.Printf("📤 Sent message #%d: correlation_id=%s, text=%s",
					sentCount, correlationID, text)
			}

			// Задержка между отправками
			time.Sleep(2 * time.Second)

			// Проверка отмены контекста
			select {
			case <-chatCtx.Done():
				return
			default:
			}
		}

		// Даем время на получение ответов
		log.Println("📤 Finished sending all messages, waiting for responses...")
		time.Sleep(15 * time.Second)

		// Закрываем отправку
		if err := stream.CloseSend(); err != nil {
			errChan <- fmt.Errorf("error closing send stream: %w", err)
			return
		}
		log.Println("📤 Closed client send stream")
	}()

	// Ожидание завершения горутин
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Обработка ошибок
	if err, ok := <-errChan; ok {
		log.Printf("❌ Chat error: %v", err)
	} else {
		log.Println("\n=== Chat Statistics ===")
		log.Printf("Messages sent: %d", sentCount)
		log.Printf("Messages received: %d", receivedCount)
		log.Printf("Acknowledgments received: %d", acknowledgedCount)
		log.Printf("Notifications received: %d", notificationCount)
		log.Println("✅ Chat completed successfully")
	}
}
