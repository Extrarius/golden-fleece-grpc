package interceptors

import (
	"io"
	"log"

	"google.golang.org/grpc"
)

// wrappedServerStream оборачивает grpc.ServerStream для переопределения методов
// и логирования каждого сообщения в стриме
type wrappedServerStream struct {
	grpc.ServerStream
}

// RecvMsg переопределяет метод для логирования входящих сообщений
func (w *wrappedServerStream) RecvMsg(m interface{}) error {
	err := w.ServerStream.RecvMsg(m)
	if err != nil && err != io.EOF {
		log.Printf("📥 Stream RecvMsg error: %v", err)
		return err
	}
	if err == nil {
		log.Printf("📥 Stream RecvMsg: received message of type %T", m)
	} else {
		log.Printf("📥 Stream RecvMsg: received EOF (stream closed)")
	}
	return err
}

// SendMsg переопределяет метод для логирования исходящих сообщений
func (w *wrappedServerStream) SendMsg(m interface{}) error {
	log.Printf("📤 Stream SendMsg: sending message of type %T", m)
	err := w.ServerStream.SendMsg(m)
	if err != nil {
		log.Printf("📤 Stream SendMsg error: %v", err)
	} else {
		log.Printf("📤 Stream SendMsg: message sent successfully")
	}
	return err
}

// StreamInterceptor логирует каждое сообщение в стриме
// Вызывается при установлении стримингового соединения
func StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Printf("🔌 Stream connection established: %s", info.FullMethod)

	// Оборачиваем ServerStream для логирования каждого сообщения
	wrapped := &wrappedServerStream{
		ServerStream: ss,
	}

	// Вызываем обработчик с обернутым стримом
	err := handler(srv, wrapped)
	if err != nil {
		log.Printf("❌ Stream handler error: %v (method: %s)", err, info.FullMethod)
	} else {
		log.Printf("✅ Stream completed successfully: %s", info.FullMethod)
	}

	return err
}
