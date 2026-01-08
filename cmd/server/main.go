package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notes-service/internal/api/gateway"
	grpcapi "notes-service/internal/api/grpc"
	"notes-service/internal/repository/memory"
	notesService "notes-service/internal/service/notes"
)

const (
	defaultPort     = "50051"
	defaultHTTPPort = "8080"
)

func main() {
	// Получаем порт из переменной окружения или используем значение по умолчанию
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	addr := "0.0.0.0:" + port
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

	noteHandler := grpcapi.NewHandler(noteSvc)
	log.Println("Initialized gRPC handler")

	// Создание gRPC сервера с интерцепторами и конфигурацией
	grpcServer := grpcapi.NewServer(noteHandler)

	// Канал для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Запуск gRPC сервера в горутине
	errChan := make(chan error, 2)
	go func() {
		log.Printf("gRPC server listening on %s", addr)
		if err := grpcServer.Serve(listener); err != nil {
			errChan <- err
		}
	}()

	// Запуск HTTP Gateway сервера в горутине
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}
	httpAddr := "0.0.0.0:" + httpPort

	gatewayCtx, gatewayCancel := context.WithCancel(context.Background())

	// Формируем адрес gRPC для Gateway (добавляем localhost если адрес начинается с :)
	grpcAddr := addr
	if grpcAddr[0] == ':' {
		grpcAddr = "localhost" + grpcAddr
	}

	go func() {
		if err := gateway.Setup(gatewayCtx, grpcAddr, httpAddr); err != nil {
			errChan <- fmt.Errorf("HTTP Gateway error: %w", err)
		}
	}()

	// Ожидание сигнала или ошибки
	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Starting graceful shutdown...", sig)
	}

	// Graceful shutdown
	// Даем серверу до 5 секунд на завершение активных запросов
	gatewayCancel() // Отменяем контекст Gateway для остановки HTTP сервера
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
