# protoc-gen-template-validate

Protoc плагин для автоматической генерации методов валидации для protobuf сообщений на основе аннотаций `validate.rules` из `protoc-gen-validate`.

**Отличие от `protoc-gen-simple-validate`:** использует `text/template` для генерации кода вместо прямого использования `g.P()`. Это позволяет:
- Разделить логику генерации и шаблоны кода
- Легче модифицировать формат генерируемого кода
- Использовать шаблоны для различных частей кода (заголовок, методы, вспомогательные функции)

**Важно:** Оба плагина могут работать одновременно, генерируя файлы с разными суффиксами:
- `protoc-gen-simple-validate` → `*.pb.validate.go`
- `protoc-gen-template-validate` → `*.pb.validate.go` (тот же суффикс, что и у simple-плагина)

## Назначение

Плагин `protoc-gen-template-validate` автоматически генерирует методы `Validate() error` для protobuf сообщений, используя правила валидации, определенные в `.proto` файлах через аннотации `validate.rules`.

Генерация кода происходит через шаблоны `text/template`, что делает код плагина более структурированным и легким для поддержки.

## Формат использования

### Аннотации `validate.rules`

Плагин использует аннотации из пакета `protoc-gen-validate` (`github.com/envoyproxy/protoc-gen-validate`). 

#### Пример proto файла:

```protobuf
syntax = "proto3";

package notes.v1;

option go_package = "notes/v1;notesv1";

import "validate/validate.proto";

message CreateNoteRequest {
  string title = 1 [
    (validate.rules).string = {
      min_len: 5,
      max_len: 255
    }
  ];
  
  string content = 2 [
    (validate.rules).string.min_len = 10
  ];
  
  string email = 3 [
    (validate.rules).string.email = true
  ];
  
  repeated string tags = 4 [
    (validate.rules).repeated = {
      min_items: 1,
      max_items: 10
    }
  ];
}
```

### Поддерживаемые правила валидации

#### Строковые правила (`string`):
- `min_len` - минимальная длина строки
- `max_len` - максимальная длина строки
- `pattern` - регулярное выражение для проверки формата
- `email` - проверка формата email адреса

#### Правила для repeated полей (`repeated`):
- `min_items` - минимальное количество элементов
- `max_items` - максимальное количество элементов

#### Числовые правила (`int32`, `int64`, `float`, `double`):
- `gte` - больше или равно
- `lte` - меньше или равно
- `gt` - больше
- `lt` - меньше

## Примеры использования

### Пример 1: Базовые валидации строк

```protobuf
message User {
  string username = 1 [
    (validate.rules).string = {
      min_len: 3,
      max_len: 20
    }
  ];
  
  string email = 2 [
    (validate.rules).string.email = true
  ];
}
```

**Сгенерированный код:**

```go
func (u *User) Validate() error {
	if len(u.Username) < 3 {
		return fmt.Errorf("field Username must be at least 3 characters")
	}
	if len(u.Username) > 20 {
		return fmt.Errorf("field Username must be at most 20 characters")
	}
	if !isValidEmail(u.Email) {
		return fmt.Errorf("field Email must be a valid email address")
	}
	return nil
}
```

### Пример 2: Валидация repeated полей

```protobuf
message Product {
  repeated string tags = 1 [
    (validate.rules).repeated = {
      min_items: 1,
      max_items: 5
    }
  ];
}
```

**Сгенерированный код:**

```go
func (p *Product) Validate() error {
	if len(p.Tags) < 1 {
		return fmt.Errorf("field Tags must have at least 1 items")
	}
	if len(p.Tags) > 5 {
		return fmt.Errorf("field Tags must have at most 5 items")
	}
	return nil
}
```

### Пример 3: Pattern валидация

```protobuf
message Code {
  string code = 1 [
    (validate.rules).string.pattern = "^[A-Z]{2}-[0-9]{4}$"
  ];
}
```

**Сгенерированный код:**

```go
func (c *Code) Validate() error {
	patternCode := regexp.MustCompile(`^[A-Z]{2}-[0-9]{4}$`)
	if !patternCode.MatchString(c.Code) {
		return fmt.Errorf("field Code does not match required pattern")
	}
	return nil
}
```

## Генерируемые методы

### Метод `Validate() error`

Для каждого protobuf сообщения, содержащего поля с аннотациями `validate.rules`, плагин генерирует метод:

```go
func (receiver *MessageName) Validate() error {
	// Проверки валидации для каждого поля
	// ...
	return nil // если все проверки прошли успешно
}
```

**Особенности:**
- Метод возвращает `error` - `nil` если валидация прошла успешно, или ошибку с описанием проблемы
- Проверки выполняются последовательно, при первой ошибке возвращается ошибка
- Для сообщений без валидаций генерируется метод, который всегда возвращает `nil`

### Вспомогательная функция `isValidEmail()`

Если в сообщении есть поля с правилом `email: true`, плагин автоматически генерирует вспомогательную функцию:

```go
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
```

## Установка и интеграция

### 1. Установка плагина

#### Через `go install`:

```bash
go install ./internal/tools/protoc-gen-template-validate
```

#### Через Taskfile:

```bash
task install-template-validate
```

Плагин будет установлен в `bin/protoc-gen-template-validate`.

### 2. Установка зависимостей

Убедиться, что установлен пакет `protoc-gen-validate`:

```bash
go get github.com/envoyproxy/protoc-gen-validate/validate
```

### 3. Использование через `easyp`

Добавьте плагин в `easyp.yaml`:

```yaml
generate:
  inputs:
    - directory: "proto"
  plugins:
    - path: bin/protoc-gen-template-validate
      out: pkg/proto
      opt:
        - paths=source_relative
```

Запустите генерацию:

```bash
easyp generate --path proto
```

### 4. Использование через `protoc` напрямую

```bash
PROTOC_GEN_VALIDATE_PATH=$(go list -m -f '{{.Dir}}' github.com/envoyproxy/protoc-gen-validate)

protoc \
  -Iproto \
  -I$PROTOC_GEN_VALIDATE_PATH \
  --plugin=protoc-gen-template-validate=./bin/protoc-gen-template-validate \
  --template-validate_out=paths=source_relative:pkg/proto \
  proto/notes/v1/notes.proto
```

### 5. Использование в коде

После генерации используйте методы валидации в вашем коде:

```go
package main

import (
	"fmt"
	notesv1 "your-project/pkg/proto/notes/v1"
)

func main() {
	req := &notesv1.CreateNoteRequest{
		Title:   "Short", // Ошибка: меньше 5 символов
		Content: "Content",
		Email:   "invalid-email",
	}
	
	if err := req.Validate(); err != nil {
		fmt.Printf("Ошибка валидации: %v\n", err)
		// Вывод: Ошибка валидации: field Title must be at least 5 characters
	}
}
```

## Структура проекта

```
internal/tools/protoc-gen-template-validate/
├── main.go              # Главный файл плагина (точка входа, извлечение данных, генерация через шаблоны)
├── templates.go         # Шаблоны Go кода (text/template)
├── types.go             # Структуры данных для шаблонов (FileInfo, MessageInfo, ValidationCheck)
├── template_test.go     # Модульные тесты (парсинг шаблонов, преобразование данных)
├── integration_test.go  # Интеграционные тесты (работа методов валидации, golden test)
└── README.md            # Документация плагина
```

## Использование шаблонов

Плагин использует `text/template` для генерации кода. Шаблоны определены в `templates.go`:

- `fileHeaderTemplate` - заголовок файла с комментарием и package
- `validateMethodTemplate` - метод `Validate()` для сообщения
- `minLenCheckTemplate`, `maxLenCheckTemplate` - проверки длины строки
- `emailCheckTemplate` - проверка формата email
- `patternCheckTemplate` - проверка регулярного выражения
- `minItemsCheckTemplate`, `maxItemsCheckTemplate` - проверки для repeated полей
- `isValidEmailTemplate` - вспомогательная функция для проверки email

Все шаблоны используют квалифицированные имена импортов (`{{.FmtErrorf}}`, `{{.RegexpMustCompile}}`), которые передаются через структуры данных, чтобы protogen корректно управлял импортами.

## Тестирование

### Модульные тесты

```bash
go test ./internal/tools/protoc-gen-template-validate/... -run TestTemplate -v
```

Тесты проверяют:
- Парсинг всех шаблонов (`TestTemplateParsing`)
- Выполнение шаблонов с данными (`TestExecuteTemplate`)
- Преобразование данных в структуры для шаблонов (`TestBuildValidationChecks`)
- Генерацию метода Validate() через шаблон (`TestValidateMethodTemplate`)

### Интеграционные тесты

```bash
go test ./internal/tools/protoc-gen-template-validate/... -run TestValidate -v
```

Тесты проверяют:
- Генерацию кода для различных типов валидаций (`TestValidate_TestMessage` - 7 тест-кейсов)
- Корректность работы методов валидации
- Обработку сообщений без валидаций (`TestValidate_EmptyMessage`)
- Сравнение с простым плагином (`TestCompareWithSimplePlugin` - golden test)

### Пример тестового proto файла

См. `proto/test/v1/test.proto` для примеров различных типов валидаций:
- MinLen/MaxLen для строк
- Email валидация
- Pattern валидация
- MinItems/MaxItems для repeated полей
- Сообщение без валидаций (EmptyMessage)

## Ограничения

- Плагин генерирует только статическую валидацию на основе правил из proto файлов
- Не поддерживается динамическая валидация в runtime
- Для сложных бизнес-правил рекомендуется использовать дополнительную валидацию в коде приложения

## См. также

- [protoc-gen-validate](https://github.com/envoyproxy/protoc-gen-validate) - исходный пакет с правилами валидации
- [protogen documentation](https://pkg.go.dev/google.golang.org/protobuf/compiler/protogen) - документация по protogen API
