package swagger

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// SetupSwaggerUI настраивает маршруты для Swagger UI и swagger.json
// Использует локальные файлы Swagger UI
func SetupSwaggerUI(mux *http.ServeMux, swaggerJSONPath string, swaggerUIDir string) {
	// Создаем файловый сервер для статических файлов Swagger UI
	// Используем относительный путь
	fs := http.FileServer(http.Dir(swaggerUIDir))

	// Эндпоинт для swagger.json (регистрируем первым, так как это точный путь)
	mux.HandleFunc("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		setupSwaggerJSON(w, r, swaggerJSONPath)
	})

	// Редирект с /swagger на /swagger-ui/
	mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger-ui/", http.StatusMovedPermanently)
	})

	// Обслуживаем Swagger UI файлы
	// StripPrefix убирает /swagger-ui/ из URL перед передачей в FileServer
	// Например: /swagger-ui/dist/file.js -> /dist/file.js -> FileServer ищет swagger-ui/dist/file.js
	mux.Handle("/swagger-ui/", http.StripPrefix("/swagger-ui/", fs))
}

// setupSwaggerJSON обрабатывает запрос к swagger.json
func setupSwaggerJSON(w http.ResponseWriter, r *http.Request, swaggerJSONPath string) {
	// Устанавливаем заголовки для CORS и правильной обработки JSON
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Обрабатываем OPTIONS запрос (preflight для CORS)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Проверяем метод для остальных запросов
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем рабочую директорию (где запущен сервер)
	wd, err := os.Getwd()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get working directory: %v", err), http.StatusInternalServerError)
		return
	}

	fullPath := filepath.Join(wd, swaggerJSONPath)

	// Проверяем существование файла
	if _, err := os.Stat(fullPath); err != nil {
		http.Error(w, fmt.Sprintf("Swagger file not found: %s", fullPath), http.StatusNotFound)
		return
	}

	// Устанавливаем заголовок Content-Type для JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	http.ServeFile(w, r, fullPath)
}
