package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpchandler "notes-service/internal/api/grpc"
	"notes-service/internal/api/grpc/interceptors"
	"notes-service/internal/repository/memory"
	notesService "notes-service/internal/service/notes"
	notesv1 "notes-service/pkg/proto/notes/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

const (
	defaultPort = "50051"
)

func main() {
	// Получаем порт из переменной окружения или используем значение по умолчанию
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	addr := ":" + port
	log.Printf("Starting Notes Service on %s", addr)

	// Создаем listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Инициализация компонентов (DI): Repository → Service → Handler
	noteRepo := memory.NewRepository()
	log.Println("Initialized in-memory repository (map-based)")

	noteSvc := notesService.NewNoteService(noteRepo)
	log.Println("Initialized note service")

	noteHandler := grpchandler.NewHandler(noteSvc)
	log.Println("Initialized gRPC handler")

	// Создание gRPC сервера с интерцепторами и конфигурацией
	// Порядок интерцепторов важен:
	// 1. Logger - логирует все запросы (включая заблокированные)
	// 2. Auth - проверяет авторизацию и блокирует неавторизованные запросы
	// MaxConcurrentStreams: ограничивает количество одновременных стримов до 25
	// для защиты сервера от перегрузки и контроля использования ресурсов
	grpcServer := grpc.NewServer(
		// Ограничиваем количество одновременных стримов
		grpc.MaxConcurrentStreams(25),
		// KeepAlive параметры для защиты от зависших соединений
		// Time: время между пингами для проверки активности соединения
		// Timeout: время ожидания ответа на ping перед разрывом соединения
		// MaxConnectionIdle: время, после которого неактивное соединение будет закрыто
		// MaxConnectionAge: максимальное время жизни соединения (ротация для профилактики деградации)
		// MaxConnectionAgeGrace: время ожидания завершения активных запросов перед закрытием
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     30 * time.Minute, // Закрытие неактивных соединений через 30 минут
			MaxConnectionAge:      1 * time.Hour,    // Максимальное время жизни соединения (ротация)
			MaxConnectionAgeGrace: 5 * time.Second,  // Ожидание завершения активных запросов перед закрытием
			Time:                  10 * time.Minute, // Время между пингами (рекомендуется 5-10 минут для backend-to-backend)
			Timeout:               20 * time.Second, // Время ожидания ответа на ping
		}),
		// Интерцепторы: Logger → Validate → Auth
		grpc.ChainUnaryInterceptor(
			interceptors.LoggerUnaryInterceptor,   // Логирует все запросы и время выполнения
			interceptors.ValidateUnaryInterceptor, // Валидирует запросы по правилам из proto
			interceptors.AuthUnaryInterceptor,     // Проверяет авторизацию токена
		),
	)

	// Регистрация сервиса
	notesv1.RegisterNotesServiceServer(grpcServer, noteHandler)
	log.Println("Registered NotesService")

	// Настройка reflection (для grpcurl/grpcui)
	reflection.Register(grpcServer)
	log.Println("Enabled gRPC reflection")

	// Канал для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Запуск gRPC сервера в горутине
	errChan := make(chan error, 1)
	go func() {
		log.Printf("gRPC server listening on %s", addr)
		if err := grpcServer.Serve(listener); err != nil {
			errChan <- err
		}
	}()

	// Ожидание сигнала или ошибки
	select {
	case err := <-errChan:
		log.Fatalf("gRPC server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Starting graceful shutdown...", sig)
	}

	// Graceful shutdown
	// Даем серверу до 5 секунд на завершение активных запросов
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	// Ожидаем завершения или таймаут
	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
	case <-ctx.Done():
		log.Println("Graceful shutdown timeout, forcing stop...")
		grpcServer.Stop()
		log.Println("gRPC server stopped forcefully")
	}

	log.Println("Notes Service stopped")
}
