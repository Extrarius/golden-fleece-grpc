package interceptors

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// authorizationHeader - имя заголовка для авторизации в metadata
	authorizationHeader = "authorization"
	// expectedToken - ожидаемый токен (хардкод для задания)
	expectedToken = "my-secret-token"
)

// AuthUnaryInterceptor проверяет наличие и валидность токена авторизации в metadata запроса.
// Токен должен быть передан в заголовке "authorization" в формате "Bearer <token>".
// Если токен отсутствует или невалиден, возвращается ошибка с кодом Unauthenticated.
func AuthUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Извлекаем metadata из контекста
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "metadata not provided")
	}

	// Получаем значение заголовка authorization
	authHeaders := md.Get(authorizationHeader)
	if len(authHeaders) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authorization header not provided")
	}

	// Берем первое значение заголовка
	authHeader := authHeaders[0]

	// Проверяем формат токена (должен начинаться с "Bearer ")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, status.Errorf(codes.Unauthenticated, "invalid authorization header format")
	}

	// Извлекаем токен (часть после "Bearer ")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Сравниваем токен с ожидаемым значением
	if token != expectedToken {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token")
	}

	// Токен валиден, пропускаем запрос дальше к хендлеру
	return handler(ctx, req)
}
