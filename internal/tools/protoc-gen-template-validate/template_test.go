package main

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

// TestExecuteTemplate проверяет выполнение шаблонов.
func TestExecuteTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tmplStr string
		data    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name:    "simple template",
			tmplStr: `Hello {{.Name}}!`,
			data:    map[string]interface{}{"Name": "World"},
			want:    "Hello World!",
			wantErr: false,
		},
		{
			name:    "minLen check template",
			tmplStr: minLenCheckTemplate,
			data: map[string]interface{}{
				"Receiver":  "m",
				"FieldName": "Title",
				"Value":     uint64(5),
				"FmtErrorf": "fmt.Errorf",
			},
			want:    "\tif len(m.Title) < 5 {\n\t\treturn fmt.Errorf(\"field Title must be at least 5 characters\")\n\t}",
			wantErr: false,
		},
		{
			name:    "email check template",
			tmplStr: emailCheckTemplate,
			data: map[string]interface{}{
				"Receiver":  "m",
				"FieldName": "Email",
				"FmtErrorf": "fmt.Errorf",
			},
			want:    "\tif !isValidEmail(m.Email) {\n\t\treturn fmt.Errorf(\"field Email must be a valid email address\")\n\t}",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeTemplate(tt.tmplStr, tt.data)
			if !tt.wantErr && result == "" {
				t.Errorf("executeTemplate() returned empty string, expected: %s", tt.want)
			}
			if !tt.wantErr && !strings.Contains(result, tt.want) {
				t.Errorf("executeTemplate() = %q, want contains %q", result, tt.want)
			}
		})
	}
}

// TestBuildValidationChecks проверяет создание ValidationCheck из FieldValidation.
func TestBuildValidationChecks(t *testing.T) {
	tests := []struct {
		name     string
		field    FieldValidation
		receiver string
		wantLen  int
	}{
		{
			name: "minLen only",
			field: FieldValidation{
				FieldName: "Title",
				MinLen:    uint64Ptr(5),
			},
			receiver: "m",
			wantLen:  1,
		},
		{
			name: "minLen and maxLen",
			field: FieldValidation{
				FieldName: "Title",
				MinLen:    uint64Ptr(5),
				MaxLen:    uint64Ptr(100),
			},
			receiver: "m",
			wantLen:  2,
		},
		{
			name: "email",
			field: FieldValidation{
				FieldName: "Email",
				Email:     true,
			},
			receiver: "m",
			wantLen:  1,
		},
		{
			name: "pattern",
			field: FieldValidation{
				FieldName: "Code",
				Pattern:   "^[A-Z]{2}-[0-9]{4}$",
			},
			receiver: "m",
			wantLen:  1,
		},
		{
			name: "minItems and maxItems",
			field: FieldValidation{
				FieldName: "Tags",
				MinItems:  uint64Ptr(1),
				MaxItems:  uint64Ptr(10),
			},
			receiver: "m",
			wantLen:  2,
		},
		{
			name: "all validations",
			field: FieldValidation{
				FieldName: "Field",
				MinLen:    uint64Ptr(5),
				MaxLen:    uint64Ptr(100),
				Email:     true,
				Pattern:   "^[A-Z]",
				MinItems:  uint64Ptr(1),
				MaxItems:  uint64Ptr(10),
			},
			receiver: "m",
			wantLen:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := buildValidationChecks(tt.field, tt.receiver, "fmt.Errorf", "regexp.MustCompile")
			if len(checks) != tt.wantLen {
				t.Errorf("buildValidationChecks() returned %d checks, want %d", len(checks), tt.wantLen)
			}
			for _, check := range checks {
				if check.Code == "" {
					t.Errorf("buildValidationChecks() returned check with empty Code")
				}
				if check.FmtErrorf == "" {
					t.Errorf("buildValidationChecks() returned check with empty FmtErrorf")
				}
			}
		})
	}
}

// TestGetReceiverName проверяет функцию getReceiverName.
func TestGetReceiverName(t *testing.T) {
	tests := []struct {
		name       string
		goTypeName string
		want       string
	}{
		{"CreateNoteRequest", "CreateNoteRequest", "c"},
		{"User", "User", "u"},
		{"Product", "Product", "m"}, // "p" заменяется на "m"
		{"Message", "Message", "m"},
		{"", "", "m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getReceiverName(tt.goTypeName)
			if got != tt.want {
				t.Errorf("getReceiverName(%q) = %q, want %q", tt.goTypeName, got, tt.want)
			}
		})
	}
}

// TestTemplateParsing проверяет парсинг шаблонов.
func TestTemplateParsing(t *testing.T) {
	templates := []struct {
		name string
		tmpl string
	}{
		{"fileHeader", fileHeaderTemplate},
		{"validateMethod", validateMethodTemplate},
		{"minLenCheck", minLenCheckTemplate},
		{"maxLenCheck", maxLenCheckTemplate},
		{"emailCheck", emailCheckTemplate},
		{"patternCheck", patternCheckTemplate},
		{"minItemsCheck", minItemsCheckTemplate},
		{"maxItemsCheck", maxItemsCheckTemplate},
		{"isValidEmail", isValidEmailTemplate},
	}

	for _, tt := range templates {
		t.Run(tt.name, func(t *testing.T) {
			_, err := template.New(tt.name).Parse(tt.tmpl)
			if err != nil {
				t.Errorf("Failed to parse template %s: %v", tt.name, err)
			}
		})
	}
}

// TestValidateMethodTemplate проверяет шаблон метода Validate().
func TestValidateMethodTemplate(t *testing.T) {
	data := ValidateMethodData{
		MessageName:  "TestMessage",
		ReceiverName: "t",
		Fields: []FieldValidationData{
			{
				FieldName: "Title",
				Validations: []ValidationCheck{
					{
						Code: "\tif len(t.Title) < 5 {\n\t\treturn fmt.Errorf(\"field Title must be at least 5 characters\")\n\t}",
					},
				},
			},
		},
		FmtErrorf:         "fmt.Errorf",
		RegexpMustCompile: "regexp.MustCompile",
	}

	tmpl := template.Must(template.New("validateMethod").Parse(validateMethodTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "func (t *TestMessage) Validate() error") {
		t.Errorf("Template result doesn't contain function signature")
	}
	if !strings.Contains(result, "return nil") {
		t.Errorf("Template result doesn't contain return nil")
	}
}

// uint64Ptr возвращает указатель на uint64.
func uint64Ptr(v uint64) *uint64 {
	return &v
}
