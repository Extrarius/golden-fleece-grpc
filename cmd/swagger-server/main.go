package main

import (
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

//go:embed embed
var embeddedFiles embed.FS

// getWSLIP пытается получить IP адрес WSL машины
func getWSLIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.String()

	// Возвращаем IP только если это не localhost
	if ip != "127.0.0.1" && !strings.HasPrefix(ip, "127.") {
		return ip, nil
	}
	return "", nil
}

func main() {
	// Получаем порт из переменной окружения или используем значение по умолчанию
	port := os.Getenv("SWAGGER_PORT")
	if port == "" {
		port = "8082"
	}

	// Используем 0.0.0.0 для прослушивания на всех IPv4 интерфейсах
	addr := "0.0.0.0:" + port

	// Создаем HTTP маршрутизатор
	mux := http.NewServeMux()

	// Получаем встроенные файлы Swagger UI
	// embeddedFiles содержит все файлы из директории embed
	swaggerUI, err := fs.Sub(embeddedFiles, "embed")
	if err != nil {
		log.Fatalf("Ошибка получения встроенных файлов Swagger UI: %v", err)
	}

	// Создаем файловый сервер для встроенных статических файлов
	fileServer := http.FileServer(http.FS(swaggerUI))

	// Обслуживаем Swagger UI файлы
	mux.Handle("/swagger-ui/", http.StripPrefix("/swagger-ui/", fileServer))

	// Загружаем встроенный swagger.json
	swaggerJSON, err := embeddedFiles.ReadFile("embed/notes.swagger.json")
	if err != nil {
		log.Fatalf("Ошибка загрузки встроенного swagger.json: %v", err)
	}

	// Обслуживаем swagger.json файл с CORS заголовками
	mux.HandleFunc("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(swaggerJSON)
	})

	// Редирект с корня на Swagger UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/swagger-ui/", http.StatusMovedPermanently)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	// Получаем IP адрес хоста для доступа из Windows (WSL)
	hostIP := "localhost"
	if wslIP := os.Getenv("WSL_HOST_IP"); wslIP != "" {
		hostIP = wslIP
	} else {
		// Пытаемся получить IP адрес WSL
		if ip, err := getWSLIP(); err == nil && ip != "" {
			hostIP = ip
		}
	}

	log.Printf("🚀 Swagger UI сервер запущен")
	log.Printf("📖 Откройте в браузере:")
	log.Printf("   - http://localhost:%s/swagger-ui/", port)
	log.Printf("   - http://127.0.0.1:%s/swagger-ui/", port)
	if hostIP != "localhost" {
		log.Printf("   - http://%s:%s/swagger-ui/ (WSL IP)", hostIP, port)
	}
	log.Printf("📄 Swagger JSON: http://localhost:%s/swagger.json", port)

	// Используем 0.0.0.0:PORT для прослушивания на всех IPv4 интерфейсах
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}
}
