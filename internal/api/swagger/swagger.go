package swagger

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed embed/*
var swaggerContent embed.FS

// ServeSwagger добавляет маршруты для Swagger UI и swagger.json в указанный mux
// swaggerSpecs - embedded файловая система со swagger.json файлом (например, из pkg/api/notes/v1/)
// Эта функция может быть переиспользована в разных проектах
//
// Создает следующие маршруты:
// - GET /swagger/ - статические файлы Swagger UI (dist/, index.html)
// - GET /swagger.json - основной swagger.json файл из swaggerSpecs
// - GET /swagger/specs/ - дополнительные swagger.json файлы из swaggerSpecs
func ServeSwagger(mux *http.ServeMux, swaggerSpecs embed.FS) {
	// Получаем встроенные файлы Swagger UI
	swaggerUI, err := fs.Sub(swaggerContent, "embed")
	if err != nil {
		log.Fatalf("Failed to get embedded Swagger UI files: %v", err)
	}

	// Создаем файловый сервер для статических файлов Swagger UI
	// StripPrefix убирает /swagger из пути перед поиском файла
	swaggerStaticsHandler := http.StripPrefix("/swagger", http.FileServer(http.FS(swaggerUI)))
	// Явно указываем метод GET для статических файлов (Go 1.21+)
	mux.Handle("GET /swagger/", swaggerStaticsHandler)

	// Создаем файловый сервер для swagger.json файлов (specs)
	swaggerSpecsHandler := http.StripPrefix("/swagger/specs", http.FileServer(http.FS(swaggerSpecs)))
	// Явно указываем метод GET для спецификаций
	mux.Handle("GET /swagger/specs/", swaggerSpecsHandler)

	// Редирект с /swagger на /swagger/index.html
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/swagger" {
			http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
			return
		}
		swaggerStaticsHandler.ServeHTTP(w, r)
	})

	// Основной эндпоинт для swagger.json (для обратной совместимости с index.html)
	// Ищем notes.swagger.json в корне swaggerSpecs
	// Функция-обработчик для swagger.json (поддерживает GET и OPTIONS для CORS)
	swaggerJSONHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Пытаемся найти swagger.json файл в swaggerSpecs
		// Пробуем разные варианты путей
		var swaggerJSON []byte
		var err error

		paths := []string{"notes.swagger.json", "swagger-specs/notes.swagger.json"}
		for _, path := range paths {
			swaggerJSON, err = swaggerSpecs.ReadFile(path)
			if err == nil {
				break
			}
		}

		if err != nil {
			// Если не найден, пробуем найти первый .json файл в любом месте
			entries, err := fs.ReadDir(swaggerSpecs, ".")
			if err != nil {
				http.Error(w, "Swagger specs not found", http.StatusNotFound)
				return
			}
			found := false
			for _, entry := range entries {
				if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[len(entry.Name())-5:] == ".json" {
					swaggerJSON, err = swaggerSpecs.ReadFile(entry.Name())
					if err == nil {
						found = true
						break
					}
				}
			}
			if !found {
				http.Error(w, "Swagger JSON not found", http.StatusNotFound)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(swaggerJSON)
	}

	// Регистрируем GET и OPTIONS для swagger.json (CORS preflight)
	mux.HandleFunc("GET /swagger.json", swaggerJSONHandler)
	mux.HandleFunc("OPTIONS /swagger.json", swaggerJSONHandler)

	log.Println("Swagger UI enabled at /swagger/")
	log.Println("Swagger JSON available at /swagger.json")
	log.Println("Swagger specs available at /swagger/specs/")
}
