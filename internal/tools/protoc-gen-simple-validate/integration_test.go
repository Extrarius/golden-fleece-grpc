package main

import (
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
