// Package main содержит protoc плагин protoc-gen-template-validate для генерации
// методов валидации для protobuf сообщений на основе аннотаций validate.rules.
//
// Плагин генерирует методы Validate() error для каждого сообщения, содержащего
// поля с правилами валидации из protoc-gen-validate.
//
// Использование text/template для генерации кода.
//
// Использование:
//
//	# Установка плагина
//	go install ./internal/tools/protoc-gen-template-validate
//
//	# Или через easyp
//	easyp generate --path proto
//
// Формат входных данных:
//
//	Используются аннотации validate.rules в proto файлах:
//
//	message CreateNoteRequest {
//	  string title = 1 [
//	    (validate.rules).string = {
//	      min_len: 5,
//	      max_len: 255
//	    }
//	  ];
//	}
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"

	validate "github.com/envoyproxy/protoc-gen-validate/validate"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

// main является точкой входа protoc плагина.
//
// Плагин читает FileDescriptorSet из stdin (передается protoc через protogen),
// обрабатывает все proto файлы и генерирует файлы с методами валидации.
//
// Формат вызова плагина:
//
//	protoc --plugin=protoc-gen-template-validate=./bin/protoc-gen-template-validate \
//	       --template-validate_out=paths=source_relative:pkg/proto \
//	       proto/notes/v1/notes.proto
func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f)
		}
		return nil
	})
}

// generateFileWithTemplate обрабатывает один proto файл и генерирует файл с методами валидации через шаблоны.
func generateFileWithTemplate(gen *protogen.Plugin, f *protogen.File) {
	// Проверяем наличие сообщений в файле (исключая map entry типы)
	if !hasMessages(f) {
		return
	}

	// Создаем файл (тот же суффикс, что и у simple-плагина для одинакового результата)
	filename := f.GeneratedFilenamePrefix + ".pb.validate.go"
	g := gen.NewGeneratedFile(filename, f.GoImportPath)

	// Получаем квалифицированные имена импортов заранее
	fmtErrorf := g.QualifiedGoIdent(protogen.GoImportPath("fmt").Ident("Errorf"))
	regexpMustCompile := g.QualifiedGoIdent(protogen.GoImportPath("regexp").Ident("MustCompile"))

	// Извлекаем информацию о файле для шаблона
	fileInfo := extractFileInfo(f, fmtErrorf, regexpMustCompile)

	// Генерируем код через шаблоны
	generateCodeWithTemplates(g, fileInfo)
}

// generateFile обрабатывает один proto файл и генерирует файл с методами валидации.
// Оставлено для обратной совместимости, вызывает generateFileWithTemplate.
func generateFile(gen *protogen.Plugin, f *protogen.File) {
	generateFileWithTemplate(gen, f)
}

// hasMessages проверяет, есть ли в файле сообщения (исключая map entry типы).
func hasMessages(f *protogen.File) bool {
	for _, msg := range f.Messages {
		if !msg.Desc.IsMapEntry() {
			return true
		}
	}
	return false
}

// extractFileInfo извлекает информацию о proto файле для передачи в шаблон.
//
// Параметры:
//   - f: protogen.File - proto файл для обработки
//   - fmtErrorf: квалифицированное имя fmt.Errorf (получено через g.QualifiedGoIdent)
//   - regexpMustCompile: квалифицированное имя regexp.MustCompile (получено через g.QualifiedGoIdent)
//
// Возвращает:
//   - FileInfo: структура с информацией о файле для передачи в шаблон fileHeaderTemplate
//
// Пример использования:
//
//	fmtErrorf := g.QualifiedGoIdent(protogen.GoImportPath("fmt").Ident("Errorf"))
//	regexpMustCompile := g.QualifiedGoIdent(protogen.GoImportPath("regexp").Ident("MustCompile"))
//	fileInfo := extractFileInfo(f, fmtErrorf, regexpMustCompile)
func extractFileInfo(f *protogen.File, fmtErrorf, regexpMustCompile string) FileInfo {
	var messages []MessageInfo
	needsEmail := false

	for _, msg := range f.Messages {
		// Пропускаем map entry типы
		if msg.Desc.IsMapEntry() {
			continue
		}

		msgInfo := extractMessageInfo(msg)
		messages = append(messages, msgInfo)

		// Проверяем, нужна ли функция isValidEmail
		for _, field := range msgInfo.Fields {
			if field.Email {
				needsEmail = true
				break
			}
		}
	}

	return FileInfo{
		PackageName:       string(f.GoPackageName),
		SourcePath:        f.Desc.Path(),
		Messages:          messages,
		NeedsEmail:        needsEmail,
		FmtErrorf:         fmtErrorf,
		RegexpMustCompile: regexpMustCompile,
	}
}

// extractMessageInfo извлекает информацию о сообщении для шаблона.
//
// Параметры:
//   - msg: protogen.Message - protobuf сообщение
//
// Возвращает:
//   - MessageInfo: структура с информацией о сообщении и его полях с валидациями
//
// Проходит по всем полям сообщения и извлекает правила валидации через extractFieldValidation.
func extractMessageInfo(msg *protogen.Message) MessageInfo {
	receiver := getReceiverName(msg.GoIdent.GoName)
	var fields []FieldValidation

	for _, field := range msg.Fields {
		fieldValidation := extractFieldValidation(field)
		if fieldValidation != nil {
			fields = append(fields, *fieldValidation)
		}
	}

	return MessageInfo{
		GoName:       msg.GoIdent.GoName,
		GoPackage:    string(msg.GoIdent.GoImportPath),
		Fields:       fields,
		ReceiverName: receiver,
	}
}

// extractFieldValidation извлекает правила валидации из поля protobuf.
//
// Параметры:
//   - field: protogen.Field - поле protobuf сообщения
//
// Возвращает:
//   - *FieldValidation: структура с правилами валидации или nil, если правил нет
//
// Проверяет наличие расширения validate.rules в опциях поля и извлекает:
//   - Строковые правила: MinLen, MaxLen, Pattern, Email
//   - Правила для repeated: MinItems, MaxItems
//
// Пример:
//
//	fieldValidation := extractFieldValidation(field)
//	if fieldValidation != nil && fieldValidation.MinLen != nil {
//	    // Поле имеет правило min_len
//	}
func extractFieldValidation(field *protogen.Field) *FieldValidation {
	opts := field.Desc.Options()
	if opts == nil {
		return nil
	}

	// Проверяем наличие расширения validate.rules
	if !proto.HasExtension(opts, validate.E_Rules) {
		return nil
	}

	// Извлекаем правила валидации из расширения
	ext := proto.GetExtension(opts, validate.E_Rules)
	rules, ok := ext.(*validate.FieldRules)
	if !ok || rules == nil {
		return nil
	}

	validation := &FieldValidation{
		FieldName:  field.GoName,
		FieldType:  field.Desc.Kind().String(),
		IsRepeated: field.Desc.IsList(),
		IsOptional: field.Desc.HasOptionalKeyword(),
	}

	// Обрабатываем строковые правила валидации
	if s := rules.GetString_(); s != nil {
		if s.MinLen != nil {
			v := s.GetMinLen()
			validation.MinLen = &v
		}
		if s.MaxLen != nil {
			v := s.GetMaxLen()
			validation.MaxLen = &v
		}
		if s.Pattern != nil {
			validation.Pattern = s.GetPattern()
		}
		if s.GetEmail() {
			validation.Email = true
		}
	}

	// Обрабатываем правила валидации для repeated полей
	if r := rules.GetRepeated(); r != nil {
		if r.MinItems != nil {
			v := r.GetMinItems()
			validation.MinItems = &v
		}
		if r.MaxItems != nil {
			v := r.GetMaxItems()
			validation.MaxItems = &v
		}
	}

	return validation
}

// addValidationCheck добавляет ValidationCheck в список проверок, если code не пустой.
//
// Параметры:
//   - checks: указатель на список проверок валидации
//   - checkType: тип проверки ("minLen", "maxLen", "email", "pattern", "minItems", "maxItems")
//   - fieldName: Go имя поля
//   - receiver: имя receiver для метода Validate()
//   - code: сгенерированный код проверки (если пустой, проверка не добавляется)
//   - fmtErrorf: квалифицированное имя fmt.Errorf
//   - errorMsg: сообщение об ошибке
//   - value: значение для проверки (может быть nil)
//   - regexpMustCompile: квалифицированное имя regexp.MustCompile (опционально, для pattern)
//   - pattern: экранированный pattern для regexp (опционально, для pattern)
//
// Если code пустой, проверка не добавляется в список.
func addValidationCheck(checks *[]ValidationCheck, checkType, fieldName, receiver, code, fmtErrorf, errorMsg string, value interface{}, regexpMustCompile, pattern string) {
	if code == "" {
		return
	}
	check := ValidationCheck{
		Type:      checkType,
		FieldName: fieldName,
		Receiver:  receiver,
		ErrorMsg:  errorMsg,
		Code:      code,
		FmtErrorf: fmtErrorf,
	}
	if value != nil {
		check.Value = value
	}
	if regexpMustCompile != "" {
		check.RegexpMustCompile = regexpMustCompile
	}
	if pattern != "" {
		check.Pattern = pattern
	}
	*checks = append(*checks, check)
}

// buildValidationChecks создает список ValidationCheck из FieldValidation, генерируя код через шаблоны.
//
// Параметры:
//   - field: FieldValidation - правила валидации для поля
//   - receiver: имя receiver для метода Validate() (например "m", "c")
//   - fmtErrorf: квалифицированное имя fmt.Errorf
//   - regexpMustCompile: квалифицированное имя regexp.MustCompile
//
// Возвращает:
//   - []ValidationCheck: список проверок валидации с сгенерированным кодом
//
// Для каждого правила валидации (minLen, maxLen, email, pattern, minItems, maxItems):
//  1. Выполняет соответствующий шаблон через executeTemplate()
//  2. Создает ValidationCheck с сгенерированным кодом
//  3. Добавляет в список проверок
//
// Пример:
//
//	checks := buildValidationChecks(fieldValidation, "m", "fmt.Errorf", "regexp.MustCompile")
//	// checks содержит ValidationCheck с Code, содержащим сгенерированный код проверки
func buildValidationChecks(field FieldValidation, receiver, fmtErrorf, regexpMustCompile string) []ValidationCheck {
	var checks []ValidationCheck

	// Строковые проверки
	if field.MinLen != nil {
		code := executeTemplate(minLenCheckTemplate, map[string]interface{}{
			"Receiver":  receiver,
			"FieldName": field.FieldName,
			"Value":     *field.MinLen,
			"FmtErrorf": fmtErrorf,
		})
		addValidationCheck(&checks, "minLen", field.FieldName, receiver, code, fmtErrorf,
			fmt.Sprintf("field %s must be at least %d characters", field.FieldName, *field.MinLen),
			*field.MinLen, "", "")
	}

	if field.MaxLen != nil {
		code := executeTemplate(maxLenCheckTemplate, map[string]interface{}{
			"Receiver":  receiver,
			"FieldName": field.FieldName,
			"Value":     *field.MaxLen,
			"FmtErrorf": fmtErrorf,
		})
		addValidationCheck(&checks, "maxLen", field.FieldName, receiver, code, fmtErrorf,
			fmt.Sprintf("field %s must be at most %d characters", field.FieldName, *field.MaxLen),
			*field.MaxLen, "", "")
	}

	if field.Pattern != "" {
		// Безопасное экранирование pattern через %q
		patternEscaped := fmt.Sprintf("%q", field.Pattern)
		code := executeTemplate(patternCheckTemplate, map[string]interface{}{
			"Receiver":          receiver,
			"FieldName":         field.FieldName,
			"Pattern":           patternEscaped,
			"FmtErrorf":         fmtErrorf,
			"RegexpMustCompile": regexpMustCompile,
		})
		addValidationCheck(&checks, "pattern", field.FieldName, receiver, code, fmtErrorf,
			fmt.Sprintf("field %s does not match required pattern", field.FieldName),
			field.Pattern, regexpMustCompile, patternEscaped)
	}

	if field.Email {
		code := executeTemplate(emailCheckTemplate, map[string]interface{}{
			"Receiver":  receiver,
			"FieldName": field.FieldName,
			"FmtErrorf": fmtErrorf,
		})
		addValidationCheck(&checks, "email", field.FieldName, receiver, code, fmtErrorf,
			fmt.Sprintf("field %s must be a valid email address", field.FieldName),
			nil, "", "")
	}

	// Repeated проверки
	if field.MinItems != nil {
		code := executeTemplate(minItemsCheckTemplate, map[string]interface{}{
			"Receiver":  receiver,
			"FieldName": field.FieldName,
			"Value":     *field.MinItems,
			"FmtErrorf": fmtErrorf,
		})
		addValidationCheck(&checks, "minItems", field.FieldName, receiver, code, fmtErrorf,
			fmt.Sprintf("field %s must have at least %d items", field.FieldName, *field.MinItems),
			*field.MinItems, "", "")
	}

	if field.MaxItems != nil {
		code := executeTemplate(maxItemsCheckTemplate, map[string]interface{}{
			"Receiver":  receiver,
			"FieldName": field.FieldName,
			"Value":     *field.MaxItems,
			"FmtErrorf": fmtErrorf,
		})
		addValidationCheck(&checks, "maxItems", field.FieldName, receiver, code, fmtErrorf,
			fmt.Sprintf("field %s must have at most %d items", field.FieldName, *field.MaxItems),
			*field.MaxItems, "", "")
	}

	return checks
}

// buildFieldValidations преобразует поля сообщения в FieldValidationData для шаблона.
//
// Параметры:
//   - msgInfo: MessageInfo - информация о сообщении с полями
//   - fmtErrorf: квалифицированное имя fmt.Errorf
//   - regexpMustCompile: квалифицированное имя regexp.MustCompile
//
// Возвращает:
//   - []FieldValidationData: список полей с их проверками валидации
//
// Для каждого поля с валидациями создает FieldValidationData, содержащую список ValidationCheck.
// Используется для передачи данных в шаблон validateMethodTemplate.
func buildFieldValidations(msgInfo MessageInfo, fmtErrorf, regexpMustCompile string) []FieldValidationData {
	var result []FieldValidationData

	for _, field := range msgInfo.Fields {
		checks := buildValidationChecks(field, msgInfo.ReceiverName, fmtErrorf, regexpMustCompile)
		if len(checks) > 0 {
			result = append(result, FieldValidationData{
				FieldName:   field.FieldName,
				Validations: checks,
			})
		}
	}

	return result
}

// executeTemplate выполняет шаблон с данными и возвращает результат как строку.
//
// Параметры:
//   - tmplStr: строка с шаблоном text/template
//   - data: данные для подстановки в шаблон (map[string]interface{} или структура)
//
// Возвращает:
//   - string: результат выполнения шаблона (сгенерированный код)
//
// Используется для генерации кода проверок валидации через шаблоны из templates.go.
// В случае ошибки логирует её в os.Stderr и возвращает пустую строку.
//
// Пример:
//
//	code := executeTemplate(minLenCheckTemplate, map[string]interface{}{
//	    "Receiver": "m",
//	    "FieldName": "Title",
//	    "Value": uint64(5),
//	    "FmtErrorf": "fmt.Errorf",
//	})
func executeTemplate(tmplStr string, data interface{}) string {
	tmpl := template.Must(template.New("").Parse(tmplStr))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// Логируем ошибку в stderr, чтобы она была видна при генерации
		fmt.Fprintf(os.Stderr, "template execution failed: %v\n", err)
		return ""
	}
	return buf.String()
}

// generateCodeWithTemplates генерирует код через шаблоны и записывает в GeneratedFile.
//
// Параметры:
//   - g: protogen.GeneratedFile - файл для записи сгенерированного кода
//   - fileInfo: FileInfo - информация о proto файле для передачи в шаблоны
//
// Процесс генерации:
//  1. Генерирует заголовок файла через fileHeaderTemplate
//  2. Для каждого сообщения генерирует метод Validate() через validateMethodTemplate
//  3. Генерирует функцию isValidEmail() через isValidEmailTemplate (если нужно)
//  4. Форматирует результат через go/format
//  5. Записывает в GeneratedFile
//
// ВАЖНО: Плагин генерирует тот же результат, что и protoc-gen-simple-validate,
// но использует text/template вместо прямых вызовов g.P(). Это позволяет
// переключаться между плагинами без изменения сгенерированного кода.
//
// Использует квалифицированные имена импортов из fileInfo (FmtErrorf, RegexpMustCompile),
// чтобы protogen корректно управлял импортами.
func generateCodeWithTemplates(g *protogen.GeneratedFile, fileInfo FileInfo) {
	var buf bytes.Buffer

	// Генерируем заголовок файла (используем готовый шаблон, инициализированный в init())
	if err := fileHeaderTmpl.Execute(&buf, fileInfo); err != nil {
		return
	}

	// Генерируем методы Validate() для каждого сообщения (используем готовый шаблон)
	for _, msgInfo := range fileInfo.Messages {
		fieldValidations := buildFieldValidations(msgInfo, fileInfo.FmtErrorf, fileInfo.RegexpMustCompile)

		methodData := ValidateMethodData{
			MessageName:       msgInfo.GoName,
			ReceiverName:      msgInfo.ReceiverName,
			Fields:            fieldValidations,
			FmtErrorf:         fileInfo.FmtErrorf,
			RegexpMustCompile: fileInfo.RegexpMustCompile,
		}

		if err := validateMethodTmpl.Execute(&buf, methodData); err != nil {
			continue
		}
	}

	// Генерируем isValidEmail, если нужно (используем готовый шаблон)
	if fileInfo.NeedsEmail {
		emailData := map[string]interface{}{
			"RegexpMustCompile": fileInfo.RegexpMustCompile,
		}
		if err := isValidEmailTmpl.Execute(&buf, emailData); err != nil {
			// Пропускаем ошибку
		}
	}

	// Форматируем код через go/format
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Fallback на исходный код, если форматирование не удалось
		formatted = buf.Bytes()
	}

	// Записываем в GeneratedFile
	g.Write(formatted)
}

// getReceiverName определяет имя receiver для метода валидации.
//
// Параметры:
//   - goTypeName: Go имя типа сообщения (например, "CreateNoteRequest")
//
// Возвращает:
//   - Имя receiver (обычно первая буква в нижнем регистре, например "c" для "CreateNoteRequest")
//   - Если имя начинается с "p", возвращается "m" (чтобы избежать конфликта с package)
//
// Примеры:
//   - "CreateNoteRequest" -> "c"
//   - "User" -> "u"
//   - "Product" -> "m" (если бы начиналось с "p")
func getReceiverName(goTypeName string) string {
	if goTypeName == "" {
		return "m"
	}
	first := strings.ToLower(string(goTypeName[0]))
	if first == "p" { // часто "p" занято под package в примерах
		return "m"
	}
	return first
}
