package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	_ "notes-service/pkg/proto/notes/v1" // Явный импорт для регистрации proto типов
	notesv1 "notes-service/pkg/proto/notes/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	defaultAddress = "localhost:50051"
	defaultToken   = "my-secret-token"
)

func main() {
	// Получаем адрес сервера из переменной окружения или используем значение по умолчанию
	address := os.Getenv("SERVER_ADDRESS")
	if address == "" {
		address = defaultAddress
	}

	// Получаем токен авторизации из переменной окружения или используем значение по умолчанию
	token := os.Getenv("AUTH_TOKEN")
	if token == "" {
		token = defaultToken
	}

	log.Printf("Connecting to gRPC server at %s...", address)

	// Создаем соединение с сервером
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Для plaintext соединения
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	log.Println("Connected successfully!")

	// Создаем клиент для NotesService
	client := notesv1.NewNotesServiceClient(conn)

	// Создаем контекст с метаданными для авторизации
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Добавляем метаданные с токеном авторизации
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", token))

	// Выбираем, какой тест запустить через переменную окружения или аргумент
	testType := os.Getenv("TEST_TYPE")
	if testType == "" && len(os.Args) > 1 {
		testType = os.Args[1]
	}

	switch testType {
	case "streaming", "stream":
		// Тестируем server-side streaming
		testSubscribeToEvents(ctx, client)
	case "upload", "metrics", "client-streaming":
		// Тестируем client-side streaming - загрузку метрик
		testUploadMetrics(ctx, client)
	case "upload-empty", "metrics-empty":
		// Тестируем client-side streaming с пустым стримом
		testUploadMetricsEmpty(ctx, client)
	case "chat", "bidirectional", "bidi":
		// Тестируем bidirectional streaming - асинхронный чат
		testChat(ctx, client)
	case "error":
		// Тестируем обработку детализированных ошибок
		testErrorHandling(ctx, client)
	case "success":
		// Тестируем успешный запрос
		testSuccessfulRequest(ctx, client)
	default:
		// По умолчанию тестируем streaming
		log.Println("No TEST_TYPE specified, testing streaming by default")
		log.Println("Available test types: streaming, upload/metrics/client-streaming, chat/bidirectional/bidi, error, success")
		log.Println("Usage: TEST_TYPE=streaming go run . OR go run . streaming")
		testSubscribeToEvents(ctx, client)
	}
}

// testErrorHandling тестирует обработку детализированных ошибок
func testErrorHandling(ctx context.Context, client notesv1.NotesServiceClient) {
	log.Println("\n=== Testing Rich Error Handling ===")

	// Пытаемся получить несуществующую заметку
	nonExistentID := "non-existent-id-12345"
	log.Printf("Attempting to get note with ID: %s", nonExistentID)

	resp, err := client.GetNote(ctx, &notesv1.GetNoteRequest{
		Id: nonExistentID,
	})

	if err != nil {
		// Конвертируем ошибку в gRPC статус
		st := status.Convert(err)

		log.Printf("\n❌ Error occurred:")
		log.Printf("  Status Code: %s", st.Code().String())
		log.Printf("  Status Message: %s", st.Message())

		// Проверяем код ошибки
		if st.Code() == codes.NotFound {
			log.Println("\n✅ Correctly received NotFound status")

			// Извлекаем Details из ошибки
			details := st.Details()
			log.Printf("\n📋 Error Details (count: %d):", len(details))

			if len(details) == 0 {
				log.Println("  ⚠️  No details found in error")
				log.Printf("  Full status: %+v", st)
				// Попробуем извлечь Details через anypb
				log.Println("\n  Trying to extract details using anypb...")
				// Проверяем, есть ли Details в proto формате
				if st.Proto() != nil && len(st.Proto().Details) > 0 {
					log.Printf("  Found %d proto details\n", len(st.Proto().Details))
					for i, detail := range st.Proto().Details {
						log.Printf("    Detail #%d: TypeURL=%s\n", i+1, detail.TypeUrl)
						// Попробуем распаковать как ErrorDetails
						if detail.TypeUrl == "type.googleapis.com/notes.v1.ErrorDetails" ||
							detail.TypeUrl == "/notes.v1.ErrorDetails" {
							var errorDetails notesv1.ErrorDetails
							opts := proto.UnmarshalOptions{}
							if err := anypb.UnmarshalTo(detail, &errorDetails, opts); err == nil {
								log.Println("  ✅ Successfully extracted ErrorDetails from anypb:")
								fmt.Printf("    📝 Error reason: %s\n", errorDetails.Reason)
								if errorDetails.NoteId != "" {
									fmt.Printf("    🆔 Note ID: %s\n", errorDetails.NoteId)
								}
								if errorDetails.InternalErrorCode != "" {
									fmt.Printf("    🔢 Internal Error Code: %s\n", errorDetails.InternalErrorCode)
								}
							} else {
								log.Printf("    Failed to unmarshal: %v", err)
							}
						}
					}
				}
			} else {
				for i, detail := range details {
					log.Printf("\n  Detail #%d:", i+1)
					log.Printf("    Type: %T", detail)
					log.Printf("    Value: %+v", detail)

					// Проверяем тип детали
					switch t := detail.(type) {
					case *notesv1.ErrorDetails:
						log.Printf("    Type: ErrorDetails")
						log.Printf("    Reason: %s", t.Reason)
						if t.NoteId != "" {
							log.Printf("    Note ID: %s", t.NoteId)
						}
						if t.InternalErrorCode != "" {
							log.Printf("    Internal Error Code: %s", t.InternalErrorCode)
						}

						// Выводим полную информацию
						log.Println("\n  ✅ Successfully extracted ErrorDetails:")
						fmt.Printf("    📝 Error reason: %s\n", t.Reason)
						if t.NoteId != "" {
							fmt.Printf("    🆔 Note ID: %s\n", t.NoteId)
						}
						if t.InternalErrorCode != "" {
							fmt.Printf("    🔢 Internal Error Code: %s\n", t.InternalErrorCode)
						}
					default:
						log.Printf("    ⚠️  Unknown type: %T", t)
						log.Printf("    Raw value: %+v", t)
					}
				}
			}
		} else {
			log.Printf("\n⚠️  Unexpected status code: %s", st.Code().String())
		}
	} else {
		log.Printf("\n✅ Note found (unexpected!): %+v", resp)
	}
}

// testSuccessfulRequest демонстрирует успешный запрос
func testSuccessfulRequest(ctx context.Context, client notesv1.NotesServiceClient) {
	log.Println("\n=== Testing Successful Request ===")

	// Сначала создаем заметку
	createResp, err := client.CreateNote(ctx, &notesv1.CreateNoteRequest{
		Title:   "Test Note Title",
		Content: "This is a test note content with enough characters",
	})
	if err != nil {
		log.Printf("Failed to create note: %v", err)
		return
	}

	log.Printf("Created note with ID: %s", createResp.Note.Id)

	// Теперь получаем созданную заметку
	getResp, err := client.GetNote(ctx, &notesv1.GetNoteRequest{
		Id: createResp.Note.Id,
	})
	if err != nil {
		log.Printf("Failed to get note: %v", err)
		return
	}

	log.Printf("Successfully retrieved note:")
	log.Printf("  ID: %s", getResp.Note.Id)
	log.Printf("  Title: %s", getResp.Note.Title)
	log.Printf("  Content: %s", getResp.Note.Content)
}
