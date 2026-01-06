package interceptors

import (
	"context"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var validator protovalidate.Validator

func init() {
	var err error
	validator, err = protovalidate.New()
	if err != nil {
		panic("failed to initialize validator: " + err.Error())
	}
}

// ValidateUnaryInterceptor валидирует входящие запросы используя protovalidate.
// Правила валидации определяются в proto файлах через аннотации (buf.validate.field).
// Если валидация не пройдена, возвращается ошибка с кодом InvalidArgument.
func ValidateUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Проверяем, что запрос является proto.Message (имеет правила валидации)
	if msg, ok := req.(proto.Message); ok {
		if err := validator.Validate(msg); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
		}
	}

	return handler(ctx, req)
}
