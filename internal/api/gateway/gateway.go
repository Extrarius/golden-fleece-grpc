package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"notes-service/internal/api/http/middleware"
	"notes-service/internal/config"
	notesv1 "notes-service/pkg/proto/notes/v1"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Setup настраивает и запускает HTTP Gateway сервер
// Если mux == nil, создается новый http.ServeMux, иначе используется переданный
func Setup(ctx context.Context, grpcAddr string, httpAddr string, cfg *config.ConfigGateway, mux *http.ServeMux) error {
	// Создаем обычный http.ServeMux если не передан
	if mux == nil {
		mux = http.NewServeMux()
	}

	// Создаем runtime.ServeMux для HTTP Gateway с настройкой передачи метаданных
	// Передаем HTTP заголовки (особенно Authorization) в gRPC metadata
	gwMux := runtime.NewServeMux(
		runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
			md := metadata.New(nil)
			// Передача заголовка authorization из HTTP в gRPC metadata
			// Это необходимо для работы Auth интерцептора на gRPC сервере
			if auth := req.Header.Get("Authorization"); auth != "" {
				md.Set("authorization", auth)
			}
			return md
		}),
	)

	// Настройка опций для Gateway
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Регистрация хендлеров NotesService на runtime.ServeMux
	err := notesv1.RegisterNotesServiceHandlerFromEndpoint(
		ctx,
		gwMux,
		grpcAddr, // Адрес gRPC сервера (например, "localhost:50051")
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	// Добавляем gateway handler на общий mux
	// Оборачиваем runtime.ServeMux в handler, который пропускает /swagger/ пути
	// чтобы они обрабатывались другими handlers (Swagger UI)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропускаем пути к Swagger UI - они должны обрабатываться другими handlers
		if strings.HasPrefix(r.URL.Path, "/swagger") {
			// Если путь начинается с /swagger, но не обработан Swagger handler,
			// значит Swagger не зарегистрирован или путь неверный - возвращаем 404
			http.NotFound(w, r)
			return
		}
		// Все остальные пути обрабатываются Gateway
		gwMux.ServeHTTP(w, r)
	}))

	// Применение middleware (в обратном порядке выполнения):
	// 1. WebSocket Proxy (для streaming методов - самый внешний слой)
	// 2. CORS (обработка CORS заголовков)
	// 3. Logging (логирует все запросы)
	// 4. Rate Limiting (ограничивает количество запросов)
	var handler http.Handler = mux
	handler = middleware.RateLimit(handler, cfg.RateLimitRPS, cfg.RateLimitBurst)
	handler = middleware.Logging(handler)
	c := setupCORS(cfg)
	handler = c.Handler(handler)
	// WebSocket proxy должен быть последним (самым внешним), чтобы корректно обрабатывать upgrade
	handler = setupWebSocketProxy(handler)

	// Запуск HTTP сервера Gateway
	// Swagger UI доступен по адресу /swagger/ (если добавлен через ServeSwagger)
	// WebSocket эндпоинты доступны для streaming методов:
	// - /notes.v1.NotesService/SubscribeToEvents (server-side streaming)
	// - /notes.v1.NotesService/UploadMetrics (client-side streaming)
	// - /notes.v1.NotesService/Chat (bidirectional streaming)
	log.Printf("HTTP Gateway server listening on %s", httpAddr)
	log.Printf("CORS enabled for origins: %s", cfg.CORSAllowedOrigins)
	log.Printf("WebSocket proxy enabled for streaming methods")
	return http.ListenAndServe(httpAddr, handler)
}

// setupCORS настраивает CORS middleware используя конфигурацию
func setupCORS(cfg *config.ConfigGateway) *cors.Cors {
	origins := strings.Split(cfg.CORSAllowedOrigins, ",")
	// Убираем пробелы из origins
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	maxAge := cfg.CORSMaxAge
	if maxAge == 0 {
		maxAge = 86400 // 24 часа по умолчанию
	}

	return cors.New(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders: []string{
			"Content-Type",
			"Authorization",
			"X-Requested-With",
		},
		AllowCredentials: true,
		MaxAge:           maxAge,
	})
}

// setupWebSocketProxy настраивает WebSocket прокси для streaming методов
// Обертывает HTTP handler для автоматической обработки WebSocket upgrade
// для всех gRPC streaming методов (server-side, client-side, bidirectional)
//
// Библиотека wsproxy автоматически обнаруживает и проксирует все streaming методы gRPC
// через gRPC-Gateway, конвертируя WebSocket соединения в gRPC стримы.
// Streaming методы доступны через WebSocket по тем же путям, что и обычные методы:
// - /notes.v1.NotesService/SubscribeToEvents (server-side streaming)
// - /notes.v1.NotesService/UploadMetrics (client-side streaming)
// - /notes.v1.NotesService/Chat (bidirectional streaming)
func setupWebSocketProxy(handler http.Handler) http.Handler {
	// wsproxy.WebsocketProxy автоматически обрабатывает WebSocket upgrade
	// для всех streaming методов gRPC через gRPC-Gateway
	// Когда клиент делает WebSocket запрос к streaming методу, библиотека:
	// 1. Обнаруживает WebSocket upgrade запрос
	// 2. Устанавливает WebSocket соединение
	// 3. Конвертирует WebSocket в gRPC стрим
	// 4. Проксирует данные между WebSocket и gRPC стримом
	return wsproxy.WebsocketProxy(handler)
}
