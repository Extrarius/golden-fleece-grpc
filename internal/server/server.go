package server

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	grpcapi "notes-service/internal/api/grpc"
	"notes-service/internal/api/grpcgateway"
	"notes-service/internal/api/swagger"
	"notes-service/internal/config"
	"notes-service/internal/repository/memory"
	notesService "notes-service/internal/service/notes"

	"google.golang.org/grpc"
)

// Server представляет сервер приложения с gRPC и HTTP Gateway
type Server struct {
	// HTTP компоненты
	Mux           *http.ServeMux
	HTTPAddr      string
	GatewayCtx    context.Context
	GatewayCancel context.CancelFunc

	// gRPC компоненты
	GRPCServer *grpc.Server
	GRPCAddr   string
	Listener   net.Listener

	// Контекст сервера для graceful shutdown стримов
	// Этот контекст отменяется при shutdown для корректного завершения стримов
	Ctx    context.Context
	Cancel context.CancelFunc

	// Конфигурация
	Config *config.Config

	// Swagger спецификации
	SwaggerSpecs embed.FS
}

// NewServer создает и инициализирует новый экземпляр сервера
func NewServer(cfg *config.Config, swaggerSpecs embed.FS) (*Server, error) {
	// Получаем порты из конфига с дефолтными значениями
	grpcPort := cfg.Server.PortGRPC
	httpPort := cfg.Server.PortHTTP

	if grpcPort == 0 {
		grpcPort = 50051
		log.Printf("⚠️  Warning: PortGRPC is 0, using default 50051")
	}
	if httpPort == 0 {
		httpPort = 8080
		log.Printf("⚠️  Warning: PortHTTP is 0, using default 8080")
	}

	log.Printf("📋 Config loaded: gRPC port=%d, HTTP port=%d", grpcPort, httpPort)

	// Проверка конфигурации Swagger
	if cfg.Swagger == nil {
		log.Printf("⚠️  Warning: Swagger config is nil")
	} else {
		log.Printf("📋 Swagger config: enabled=%v", cfg.Swagger.Enabled)
	}

	grpcAddr := "0.0.0.0:" + strconv.Itoa(grpcPort)
	httpAddr := "0.0.0.0:" + strconv.Itoa(httpPort)

	// Создаем listener для gRPC
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", grpcAddr, err)
	}

	// Создаем контекст сервера для graceful shutdown стримов
	// Этот контекст будет отменен при получении сигнала shutdown
	// В отличие от unary методов, где контекст автоматически отменяется при GracefulStop(),
	// в стримах необходимо явно слушать этот контекст для корректного завершения
	serverCtx, serverCancel := context.WithCancel(context.Background())

	// Создаем контекст для Gateway
	gatewayCtx, gatewayCancel := context.WithCancel(context.Background())

	// Создаем HTTP mux
	mux := http.NewServeMux()

	return &Server{
		Mux:           mux,
		HTTPAddr:      httpAddr,
		GatewayCtx:    gatewayCtx,
		GatewayCancel: gatewayCancel,
		GRPCAddr:      grpcAddr,
		Listener:      listener,
		Ctx:           serverCtx,
		Cancel:        serverCancel,
		Config:        cfg,
		SwaggerSpecs:  swaggerSpecs,
	}, nil
}

// Initialize инициализирует компоненты сервера (Repository → Service → Handler)
func (s *Server) Initialize() error {
	// Инициализация компонентов (DI): Repository → Service → Handler
	noteRepo := memory.NewRepository()
	log.Println("Initialized in-memory repository (map-based)")

	noteSvc := notesService.NewNoteService(noteRepo)
	log.Println("Initialized note service")

	noteHandler := grpcapi.NewHandler(noteSvc, s.Ctx)
	log.Println("Initialized gRPC handler with server context for graceful shutdown")

	// Создание gRPC сервера с интерцепторами и конфигурацией
	s.GRPCServer = grpcapi.NewServer(noteHandler)

	return nil
}

// ServeSwagger регистрирует маршруты Swagger UI на HTTP mux
func (s *Server) ServeSwagger() {
	if s.Config.Swagger == nil || !s.Config.Swagger.Enabled {
		log.Printf("⚠️  Swagger UI is disabled or not configured")
		return
	}

	log.Printf("🔧 Initializing Swagger UI...")
	swagger.ServeSwagger(s.Mux, s.SwaggerSpecs)

	// Извлекаем порт из адреса для логирования
	httpPort := strconv.Itoa(s.Config.Server.PortHTTP)
	if httpPort == "0" {
		httpPort = "8080"
	}
	log.Printf("📖 Swagger UI available at http://localhost:%s/swagger/", httpPort)
	log.Printf("📖 Swagger UI also at http://172.17.207.2:%s/swagger/ (WSL IP)", httpPort)
}

// Start запускает gRPC и HTTP Gateway серверы в горутинах
// Возвращает канал ошибок для отслеживания ошибок серверов
func (s *Server) Start() <-chan error {
	errChan := make(chan error, 2)

	// Запуск gRPC сервера в горутине
	go func() {
		log.Printf("gRPC server listening on %s", s.GRPCAddr)
		if err := s.GRPCServer.Serve(s.Listener); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Формируем адрес gRPC для Gateway (добавляем localhost если адрес начинается с :)
	grpcAddr := s.GRPCAddr
	if grpcAddr[0] == ':' {
		grpcAddr = "localhost" + grpcAddr
	}

	// Запускаем Gateway на том же mux
	// Gateway доступен с префиксом /api/v1/ (пути из proto: /notes/v1/*)
	go func() {
		if err := grpcgateway.Setup(s.GatewayCtx, grpcAddr, s.HTTPAddr, s.Config.Gateway, s.Mux); err != nil {
			errChan <- fmt.Errorf("HTTP Gateway error: %w", err)
		}
	}()

	return errChan
}

// Shutdown выполняет graceful shutdown сервера
func (s *Server) Shutdown() error {
	log.Println("Starting graceful shutdown...")

	// КРИТИЧЕСКИ ВАЖНО: Отменяем контекст сервера ПЕРЕД GracefulStop()
	// Это необходимо для корректного завершения стримов, которые слушают serverCtx
	// В отличие от unary методов, где контекст автоматически отменяется при GracefulStop(),
	// в стримах необходимо явно отменить serverCtx, чтобы они корректно завершились
	log.Println("Cancelling server context to signal streaming methods to stop...")
	s.Cancel() // Отменяем контекст сервера для завершения стримов

	s.GatewayCancel() // Отменяем контекст Gateway для остановки HTTP сервера

	shutdownTimeout := time.Duration(s.Config.Server.GracefulShutdownTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		s.GRPCServer.GracefulStop()
		close(stopped)
	}()

	// Ожидаем завершения или таймаут
	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
		return nil
	case <-ctx.Done():
		log.Println("Graceful shutdown timeout, forcing stop...")
		s.GRPCServer.Stop()
		log.Println("gRPC server stopped forcefully")
		return ctx.Err()
	}
}
