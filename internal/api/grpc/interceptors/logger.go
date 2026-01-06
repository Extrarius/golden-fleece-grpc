package interceptors

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggerUnaryInterceptor перехватывает запросы и логирует информацию о них:
// - начало запроса (Method name)
// - время выполнения хендлера
// - конец запроса (статус ответа + затраченное время)
func LoggerUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Логируем начало запроса
	log.Printf("Incoming request: %s", info.FullMethod)

	// Засекаем время начала выполнения
	start := time.Now()

	// Вызываем следующий обработчик в цепочке
	resp, err := handler(ctx, req)

	// Вычисляем время выполнения
	duration := time.Since(start)

	// Логируем результат запроса
	if err != nil {
		// Извлекаем статус из ошибки
		st, ok := status.FromError(err)
		if ok {
			log.Printf("Request %s failed with status %s: %v (duration: %v)",
				info.FullMethod, st.Code(), st.Message(), duration)
		} else {
			log.Printf("Request %s failed with error: %v (duration: %v)",
				info.FullMethod, err, duration)
		}
	} else {
		log.Printf("Request %s completed successfully (duration: %v)",
			info.FullMethod, duration)
	}

	return resp, err
}
