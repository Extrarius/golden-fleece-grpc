package grpc

import (
	"log"
	"time"

	"notes-service/internal/api/grpc/interceptors"
	notesv1 "notes-service/pkg/proto/notes/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// NewServer создает и настраивает gRPC сервер с интерцепторами и конфигурацией
func NewServer(handler notesv1.NotesServiceServer) *grpc.Server {
	// Создание gRPC сервера с интерцепторами и конфигурацией
	// Порядок интерцепторов важен:
	// 1. Logger - логирует все запросы (включая заблокированные)
	// 2. Validate - валидирует запросы по правилам из proto
	// 3. Auth - проверяет авторизацию и блокирует неавторизованные запросы
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
		// Стриминговые интерцепторы: логирование каждого сообщения в стриме
		grpc.ChainStreamInterceptor(
			interceptors.StreamInterceptor, // Логирует каждое сообщение в стримах (RecvMsg/SendMsg)
		),
	)

	// Регистрация сервиса
	notesv1.RegisterNotesServiceServer(grpcServer, handler)
	log.Println("Registered NotesService")

	// Настройка reflection (для grpcurl/grpcui)
	reflection.Register(grpcServer)
	log.Println("Enabled gRPC reflection")

	return grpcServer
}
