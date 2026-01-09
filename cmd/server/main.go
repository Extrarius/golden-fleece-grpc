package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"notes-service/internal/api/gateway"
	grpcapi "notes-service/internal/api/grpc"
	"notes-service/internal/api/swagger"
	"notes-service/internal/config"
	"notes-service/internal/repository/memory"
	notesService "notes-service/internal/service/notes"
)

const configFile = "config.yml"

//go:embed swagger-specs/*
var swaggerSpecs embed.FS

func main() {
	// Загружаем конфигурацию из файла
	appConfig, err := config.InitConfig[config.Config](configFile)
	if err != nil {
		log.Fatalf("Error initializing config: %v", err)
	}

	// Получаем порты из конфига
	grpcPort := strconv.Itoa(appConfig.Server.PortGRPC)
	httpPort := strconv.Itoa(appConfig.Server.PortHTTP)

	// Отладочное логирование
	if appConfig.Server.PortGRPC == 0 {
		log.Printf("⚠️  Warning: PortGRPC is 0, using default 50051")
		grpcPort = "50051"
	}
	if appConfig.Server.PortHTTP == 0 {
		log.Printf("⚠️  Warning: PortHTTP is 0, using default 8080")
		httpPort = "8080"
	}
	log.Printf("📋 Config loaded: gRPC port=%s, HTTP port=%s", grpcPort, httpPort)

	// Проверка конфигурации Swagger
	if appConfig.Swagger == nil {
		log.Printf("⚠️  Warning: Swagger config is nil")
	} else {
		log.Printf("📋 Swagger config: enabled=%v", appConfig.Swagger.Enabled)
	}

	addr := "0.0.0.0:" + grpcPort
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
	httpAddr := "0.0.0.0:" + httpPort

	gatewayCtx, gatewayCancel := context.WithCancel(context.Background())

	// Формируем адрес gRPC для Gateway (добавляем localhost если адрес начинается с :)
	grpcAddr := addr
	if grpcAddr[0] == ':' {
		grpcAddr = "localhost" + grpcAddr
	}

	// Создаем общий HTTP mux для Gateway и Swagger
	httpMux := http.NewServeMux()

	// Важно: регистрируем Swagger ПЕРЕД Gateway
	// чтобы маршруты /swagger/ обрабатывались Swagger, а не Gateway
	// Добавляем Swagger UI на общий mux (если включен в конфиге)
	if appConfig.Swagger != nil && appConfig.Swagger.Enabled {
		log.Printf("🔧 Initializing Swagger UI...")
		swagger.ServeSwagger(httpMux, swaggerSpecs)
		log.Printf("📖 Swagger UI available at http://localhost:%s/swagger/", httpPort)
		log.Printf("📖 Swagger UI also at http://172.17.207.2:%s/swagger/ (WSL IP)", httpPort)
	} else {
		log.Printf("⚠️  Swagger UI is disabled or not configured")
	}

	// Запускаем Gateway на том же mux
	// Gateway обрабатывает только зарегистрированные пути (/notes/v1/*)
	go func() {
		if err := gateway.Setup(gatewayCtx, grpcAddr, httpAddr, appConfig.Gateway, httpMux); err != nil {
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
	// Даем серверу время на завершение активных запросов из конфига
	gatewayCancel() // Отменяем контекст Gateway для остановки HTTP сервера
	shutdownTimeout := time.Duration(appConfig.Server.GracefulShutdownTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
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
