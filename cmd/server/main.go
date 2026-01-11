package main

import (
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"

	"notes-service/internal/config"
	"notes-service/internal/server"
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

	log.Printf("Starting Notes Service")

	// Создаем и инициализируем сервер
	srv, err := server.NewServer(appConfig, swaggerSpecs)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Cancel() // Гарантируем отмену контекста при завершении

	// Инициализируем компоненты (Repository → Service → Handler)
	if err := srv.Initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Регистрируем Swagger UI (если включен в конфиге)
	srv.ServeSwagger()

	// Запускаем сервер (gRPC и HTTP Gateway) и получаем канал ошибок
	errChan := srv.Start()

	// Канал для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Ожидание сигнала завершения или ошибки
	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
	}

	// Выполняем graceful shutdown
	if err := srv.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Notes Service stopped")
}
