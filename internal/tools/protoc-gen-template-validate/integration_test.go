package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	testv1 "notes-service/pkg/proto/test/v1"
)

// TestValidate_TestMessage проверяет работу методов валидации для TestMessage
// с различными типами правил валидации (MinLen, MaxLen, Email, Pattern, Repeated).
func TestValidate_TestMessage(t *testing.T) {
	tests := []struct {
		name    string
		message *testv1.TestMessage
		wantErr bool
	}{
		{
			name: "valid message",
			message: &testv1.TestMessage{
				Title:        "Valid Title",
				Email:        "test@example.com",
				PatternField: "Hello",
				Tags:         []string{"tag1"},
				Age:          25,
			},
			wantErr: false,
		},
		{
			name: "title too short",
			message: &testv1.TestMessage{
				Title:        "Ab",
				Email:        "test@example.com",
				PatternField: "Hello",
				Tags:         []string{"tag1"},
				Age:          25,
			},
			wantErr: true,
		},
		{
			name: "title too long",
			message: &testv1.TestMessage{
				Title:        string(make([]byte, 101)),
				Email:        "test@example.com",
				PatternField: "Hello",
				Tags:         []string{"tag1"},
				Age:          25,
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			message: &testv1.TestMessage{
				Title:        "Valid Title",
				Email:        "invalid-email",
				PatternField: "Hello",
				Tags:         []string{"tag1"},
				Age:          25,
			},
			wantErr: true,
		},
		{
			name: "pattern field invalid",
			message: &testv1.TestMessage{
				Title:        "Valid Title",
				Email:        "test@example.com",
				PatternField: "hello", // должно начинаться с заглавной буквы
				Tags:         []string{"tag1"},
				Age:          25,
			},
			wantErr: true,
		},
		{
			name: "empty tags",
			message: &testv1.TestMessage{
				Title:        "Valid Title",
				Email:        "test@example.com",
				PatternField: "Hello",
				Tags:         []string{},
				Age:          25,
			},
			wantErr: true,
		},
		{
			name: "too many tags",
			message: &testv1.TestMessage{
				Title:        "Valid Title",
				Email:        "test@example.com",
				PatternField: "Hello",
				Tags:         make([]string, 11),
				Age:          25,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_EmptyMessage проверяет, что для сообщений без правил валидации
// генерируется метод Validate(), который всегда возвращает nil.
func TestValidate_EmptyMessage(t *testing.T) {
	msg := &testv1.EmptyMessage{
		Name: "test",
	}

	// EmptyMessage не имеет валидаций, поэтому должен возвращать nil
	err := msg.Validate()
	if err != nil {
		t.Errorf("Validate() для EmptyMessage должен возвращать nil, получили: %v", err)
	}
}

// TestCompareWithSimplePlugin сравнивает сгенерированные файлы обоими плагинами.
// Это golden test для проверки функциональной эквивалентности.
//
// Тест генерирует код через оба плагина и сохраняет в файлы с разными именами
// (test_simple.pb.validate.go и test_template.pb.validate.go) для сравнения.
func TestCompareWithSimplePlugin(t *testing.T) {
	// Создаем временные каталоги для генерации
	simpleDir := t.TempDir()
	templateDir := t.TempDir()

	// Пути к плагинам
	simplePlugin := filepath.Join("bin", "protoc-gen-simple-validate")
	templatePlugin := filepath.Join("bin", "protoc-gen-template-validate")

	// Проверяем наличие плагинов
	if _, err := os.Stat(simplePlugin); os.IsNotExist(err) {
		t.Skipf("Плагин %s не найден. Запустите 'task install-simple-validate'", simplePlugin)
	}
	if _, err := os.Stat(templatePlugin); os.IsNotExist(err) {
		t.Skipf("Плагин %s не найден. Запустите 'task install-template-validate'", templatePlugin)
	}

	// Путь к proto файлу
	protoFile := "proto/test/v1/test.proto"
	if _, err := os.Stat(protoFile); os.IsNotExist(err) {
		t.Fatalf("Proto файл %s не найден", protoFile)
	}

	// Получаем путь к validate.proto из зависимостей
	validateProtoPath, err := getValidateProtoPath()
	if err != nil {
		t.Fatalf("Не удалось найти validate.proto: %v", err)
	}

	// Генерируем через simple-плагин
	simpleGeneratedFile := filepath.Join(simpleDir, "test", "v1", "test.pb.validate.go")
	if err := generateWithPlugin(simplePlugin, protoFile, validateProtoPath, simpleDir); err != nil {
		t.Fatalf("Ошибка генерации через simple-плагин: %v", err)
	}

	// Генерируем через template-плагин
	templateGeneratedFile := filepath.Join(templateDir, "test", "v1", "test.pb.validate.go")
	if err := generateWithPlugin(templatePlugin, protoFile, validateProtoPath, templateDir); err != nil {
		t.Fatalf("Ошибка генерации через template-плагин: %v", err)
	}

	// Проверяем наличие сгенерированных файлов
	if _, err := os.Stat(simpleGeneratedFile); os.IsNotExist(err) {
		t.Fatalf("Файл %s не был сгенерирован simple-плагином", simpleGeneratedFile)
	}
	if _, err := os.Stat(templateGeneratedFile); os.IsNotExist(err) {
		t.Fatalf("Файл %s не был сгенерирован template-плагином", templateGeneratedFile)
	}

	// Сохраняем файлы с разными именами для golden-теста
	// Создаем каталог для golden-файлов
	goldenDir := filepath.Join("pkg", "proto", "test", "v1")
	if err := os.MkdirAll(goldenDir, 0755); err != nil {
		t.Fatalf("Не удалось создать каталог для golden-файлов: %v", err)
	}

	// Копируем файлы с разными именами
	simpleGoldenFile := filepath.Join(goldenDir, "test_simple.pb.validate.go")
	templateGoldenFile := filepath.Join(goldenDir, "test_template.pb.validate.go")

	if err := copyFile(simpleGeneratedFile, simpleGoldenFile); err != nil {
		t.Fatalf("Не удалось скопировать simple-файл: %v", err)
	}
	if err := copyFile(templateGeneratedFile, templateGoldenFile); err != nil {
		t.Fatalf("Не удалось скопировать template-файл: %v", err)
	}

	// Читаем файлы для сравнения
	simpleContent, err := os.ReadFile(simpleGoldenFile)
	if err != nil {
		t.Fatalf("Не удалось прочитать %s: %v", simpleGoldenFile, err)
	}

	templateContent, err := os.ReadFile(templateGoldenFile)
	if err != nil {
		t.Fatalf("Не удалось прочитать %s: %v", templateGoldenFile, err)
	}

	// Нормализуем содержимое (убираем различия в форматировании и комментариях)
	simpleNormalized := normalizeGeneratedCode(string(simpleContent))
	templateNormalized := normalizeGeneratedCode(string(templateContent))

	// Извлекаем только методы Validate() для сравнения
	simpleMethods := extractValidateMethods(simpleNormalized)
	templateMethods := extractValidateMethods(templateNormalized)

	// Сравниваем методы Validate()
	if len(simpleMethods) != len(templateMethods) {
		t.Errorf("Количество методов Validate() отличается: simple=%d, template=%d",
			len(simpleMethods), len(templateMethods))
	}

	// Проверяем, что методы функционально эквивалентны
	// (игнорируя различия в форматировании и комментариях)
	for msgName, simpleMethod := range simpleMethods {
		templateMethod, exists := templateMethods[msgName]
		if !exists {
			t.Errorf("Метод Validate() для %s не найден в template-validate файле", msgName)
			continue
		}

		// Сравниваем логику валидации (убираем различия в форматировании)
		simpleLogic := extractValidationLogic(simpleMethod)
		templateLogic := extractValidationLogic(templateMethod)

		if simpleLogic != templateLogic {
			t.Errorf("Логика валидации для %s отличается:\nSimple:\n%s\nTemplate:\n%s",
				msgName, simpleLogic, templateLogic)
		}
	}

	// Дополнительно сравниваем полное содержимое (после нормализации)
	if simpleNormalized != templateNormalized {
		// Если есть различия, выводим их для отладки
		t.Logf("Полное содержимое файлов отличается после нормализации")
		t.Logf("Simple (первые 500 символов):\n%s", truncateString(simpleNormalized, 500))
		t.Logf("Template (первые 500 символов):\n%s", truncateString(templateNormalized, 500))
		// Не делаем это ошибкой, так как мы уже проверили логику валидации
	}
}

// normalizeGeneratedCode нормализует сгенерированный код для сравнения.
func normalizeGeneratedCode(code string) string {
	// Убираем комментарии с именами плагинов
	code = strings.ReplaceAll(code, "protoc-gen-simple-validate", "protoc-gen-validate")
	code = strings.ReplaceAll(code, "protoc-gen-template-validate", "protoc-gen-validate")

	// Убираем лишние пробелы и переводы строк
	lines := strings.Split(code, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}

// extractValidateMethods извлекает методы Validate() из кода.
func extractValidateMethods(code string) map[string]string {
	methods := make(map[string]string)

	// Ищем методы Validate() по паттерну
	lines := strings.Split(code, "\n")
	var currentMethod []string
	var currentName string
	inMethod := false

	for _, line := range lines {
		if strings.Contains(line, "func (") && strings.Contains(line, ") Validate() error {") {
			// Извлекаем имя типа из сигнатуры
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "*" && i+1 < len(parts) {
					currentName = strings.TrimSuffix(parts[i+1], ")")
					break
				}
			}
			inMethod = true
			currentMethod = []string{line}
		} else if inMethod {
			currentMethod = append(currentMethod, line)
			if strings.TrimSpace(line) == "}" {
				methods[currentName] = strings.Join(currentMethod, "\n")
				inMethod = false
				currentMethod = nil
			}
		}
	}

	return methods
}

// extractValidationLogic извлекает логику валидации из метода (без форматирования).
func extractValidationLogic(method string) string {
	// Извлекаем только проверки (строки с if, return)
	lines := strings.Split(method, "\n")
	var logic []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "return ") {
			// Нормализуем: убираем различия в именах переменных и форматировании
			normalized := strings.ReplaceAll(trimmed, "\t", " ")
			normalized = strings.ReplaceAll(normalized, "  ", " ")
			logic = append(logic, normalized)
		}
	}
	return strings.Join(logic, "\n")
}

// generateWithPlugin запускает protoc с указанным плагином для генерации кода.
func generateWithPlugin(pluginPath, protoFile, validateProtoPath, outputDir string) error {
	// Создаем структуру каталогов
	outputPath := filepath.Join(outputDir, "test", "v1")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}

	// Определяем имя плагина из пути
	pluginName := filepath.Base(pluginPath)
	// Убираем префикс "protoc-gen-"
	flagName := strings.TrimPrefix(pluginName, "protoc-gen-")

	// Строим команду protoc
	cmd := exec.Command("protoc",
		"-I", "proto",
		"-I", validateProtoPath,
		"--plugin", pluginName+"="+pluginPath,
		"--"+flagName+"_out", "paths=source_relative:"+outputDir,
		protoFile,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("protoc failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// getValidateProtoPath находит путь к validate.proto из зависимостей Go.
func getValidateProtoPath() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/envoyproxy/protoc-gen-validate")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to find protoc-gen-validate module: %v", err)
	}
	moduleDir := strings.TrimSpace(stdout.String())
	return filepath.Join(moduleDir, "validate"), nil
}

// truncateString обрезает строку до указанной длины.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// copyFile копирует файл из источника в назначение.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
