package main

// FieldValidation хранит извлечённые правила валидации для одного поля сообщения.
//
// Используется на этапе извлечения данных из protogen.Field через extractFieldValidation().
// Затем преобразуется в ValidationCheck через buildValidationChecks() для передачи в шаблоны.
//
// Пример:
//
//	FieldValidation{
//	    FieldName: "Title",
//	    FieldType: "string",
//	    MinLen: &uint64(5),
//	    MaxLen: &uint64(255),
//	}
type FieldValidation struct {
	FieldName  string // Go имя поля (например Title)
	FieldType  string // Тип поля (string, int32, etc.) - protobuf kind как строка
	IsRepeated bool   // Является ли поле repeated (слайсом)
	IsOptional bool   // Является ли поле optional (указателем)

	// String rules
	MinLen  *uint64 // Минимальная длина строки (в protobuf это uint64, но при генерации используем как int32)
	MaxLen  *uint64 // Максимальная длина строки
	Pattern string  // Регулярное выражение для строки
	Email   bool    // Проверка email формата

	// Repeated rules
	MinItems *uint64 // Минимальное количество элементов (для repeated)
	MaxItems *uint64 // Максимальное количество элементов (для repeated)

	// Примечание: Поля Required, Min, Max удалены, так как они не реализованы
	// ни в protoc-gen-simple-validate, ни в protoc-gen-template-validate.
	// Сейчас поддерживаются только: string (min_len, max_len, pattern, email) + repeated (min_items, max_items)
}

// MessageInfo хранит информацию о protobuf message, нужную для генерации.
//
// Используется на этапе извлечения данных из protogen.Message через extractMessageInfo().
// Содержит список полей с валидациями (FieldValidation).
//
// Пример:
//
//	MessageInfo{
//	    GoName: "CreateNoteRequest",
//	    GoPackage: "notesv1",
//	    ReceiverName: "c",
//	    Fields: []FieldValidation{...},
//	}
type MessageInfo struct {
	GoName       string            // Go имя типа сообщения
	GoPackage    string            // Go пакет
	Fields       []FieldValidation // Список полей с валидациями
	ReceiverName string            // Имя receiver (обычно первая буква в нижнем регистре)
}

// FileInfo хранит информацию о proto файле для передачи в шаблон файла.
//
// Используется в generateCodeWithTemplates() для генерации заголовка файла через fileHeaderTemplate.
// Содержит список всех сообщений (MessageInfo) и квалифицированные имена импортов.
//
// Важно: FmtErrorf и RegexpMustCompile должны быть получены через g.QualifiedGoIdent(),
// чтобы protogen корректно управлял импортами в сгенерированном коде.
//
// Пример:
//
//	FileInfo{
//	    PackageName: "notesv1",
//	    SourcePath: "proto/notes/v1/notes.proto",
//	    Messages: []MessageInfo{...},
//	    NeedsEmail: true,
//	    FmtErrorf: "fmt.Errorf",
//	    RegexpMustCompile: "regexp.MustCompile",
//	}
type FileInfo struct {
	PackageName       string        // Имя пакета Go
	SourcePath        string        // Путь к исходному proto файлу
	Messages          []MessageInfo // Список сообщений с валидациями
	NeedsEmail        bool          // Нужна ли функция isValidEmail()
	FmtErrorf         string        // Квалифицированное имя fmt.Errorf (через g.QualifiedGoIdent)
	RegexpMustCompile string        // Квалифицированное имя regexp.MustCompile (через g.QualifiedGoIdent)
}

// ValidateMethodData хранит данные для шаблона метода Validate().
//
// Используется в generateCodeWithTemplates() для генерации метода Validate() через validateMethodTemplate.
// Содержит список полей с их проверками валидации (FieldValidationData).
//
// Пример:
//
//	ValidateMethodData{
//	    MessageName: "CreateNoteRequest",
//	    ReceiverName: "c",
//	    Fields: []FieldValidationData{
//	        {
//	            FieldName: "Title",
//	            Validations: []ValidationCheck{
//	                {Type: "minLen", Code: "if len(c.Title) < 5 { return fmt.Errorf(...) }"},
//	            },
//	        },
//	    },
//	    FmtErrorf: "fmt.Errorf",
//	    RegexpMustCompile: "regexp.MustCompile",
//	}
type ValidateMethodData struct {
	MessageName       string                // Go имя типа сообщения
	ReceiverName      string                // Имя receiver
	Fields            []FieldValidationData // Список полей с валидациями
	FmtErrorf         string                // Квалифицированное имя fmt.Errorf
	RegexpMustCompile string                // Квалифицированное имя regexp.MustCompile
}

// FieldValidationData хранит данные о валидациях одного поля для шаблона.
//
// Используется в ValidateMethodData для передачи в validateMethodTemplate.
// Содержит список проверок валидации (ValidationCheck) для одного поля.
//
// Пример:
//
//	FieldValidationData{
//	    FieldName: "Title",
//	    Validations: []ValidationCheck{
//	        {Type: "minLen", Code: "if len(c.Title) < 5 { return fmt.Errorf(...) }"},
//	        {Type: "maxLen", Code: "if len(c.Title) > 255 { return fmt.Errorf(...) }"},
//	    },
//	}
type FieldValidationData struct {
	FieldName   string            // Go имя поля
	Validations []ValidationCheck // Список проверок валидации
}

// ValidationCheck хранит данные об одной проверке валидации для шаблона.
//
// Используется в FieldValidationData для передачи в validateMethodTemplate.
// Поле Code содержит уже сгенерированный код проверки (через executeTemplate()),
// который вставляется в шаблон метода Validate().
//
// Типы проверок:
//   - "minLen": минимальная длина строки
//   - "maxLen": максимальная длина строки
//   - "email": проверка формата email
//   - "pattern": проверка регулярного выражения
//   - "minItems": минимальное количество элементов в repeated поле
//   - "maxItems": максимальное количество элементов в repeated поле
//
// Пример:
//
//	ValidationCheck{
//	    Type: "minLen",
//	    Value: uint64(5),
//	    FieldName: "Title",
//	    Receiver: "c",
//	    ErrorMsg: "field Title must be at least 5 characters",
//	    Code: "if len(c.Title) < 5 {\n\treturn fmt.Errorf(\"field Title must be at least 5 characters\")\n}",
//	    FmtErrorf: "fmt.Errorf",
//	}
type ValidationCheck struct {
	Type              string      // Тип проверки: "minLen", "maxLen", "email", "pattern", "minItems", "maxItems"
	Value             interface{} // Значение для проверки (uint64 для minLen/maxLen/minItems/maxItems, string для pattern)
	FieldName         string      // Go имя поля
	Receiver          string      // Имя receiver
	ErrorMsg          string      // Сообщение об ошибке
	Code              string      // Сгенерированный код проверки (для вставки в шаблон)
	FmtErrorf         string      // Квалифицированное имя fmt.Errorf (для использования в шаблонах проверок)
	RegexpMustCompile string      // Квалифицированное имя regexp.MustCompile (для использования в шаблонах проверок)
	Pattern           string      // Экранированный pattern для regexp (используется только для pattern проверок)
}
