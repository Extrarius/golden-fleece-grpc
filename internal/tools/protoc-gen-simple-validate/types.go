package main

// FieldValidation хранит извлечённые правила валидации для одного поля сообщения.
type FieldValidation struct {
	FieldName  string // Go имя поля (например Title)
	FieldType  string // Тип поля (string, int32, etc.) - protobuf kind как строка
	IsRepeated bool   // Является ли поле repeated (слайсом)
	IsOptional bool   // Является ли поле optional (указателем)

	Required bool // Обязательное поле

	// String rules
	MinLen  *uint64 // Минимальная длина строки (в protobuf это uint64, но при генерации используем как int32)
	MaxLen  *uint64 // Максимальная длина строки
	Pattern string  // Регулярное выражение для строки
	Email   bool    // Проверка email формата

	// Repeated rules
	MinItems *uint64 // Минимальное количество элементов (для repeated)
	MaxItems *uint64 // Максимальное количество элементов (для repeated)

	// Number rules
	Min *float64 // Минимальное значение числа
	Max *float64 // Максимальное значение числа
}

// MessageInfo хранит информацию о protobuf message, нужную для генерации.
type MessageInfo struct {
	GoName       string            // Go имя типа сообщения
	GoPackage    string            // Go пакет
	Fields       []FieldValidation // Список полей с валидациями
	ReceiverName string            // Имя receiver (обычно первая буква в нижнем регистре)
}
