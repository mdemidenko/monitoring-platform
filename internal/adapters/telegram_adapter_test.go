package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/domain"
)

func TestTelegramAdapter_Send(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.TelegramConfig
		notification   *domain.Notification
		responseBody   string
		responseStatus int
		roundTripErr   error
		wantErr        bool
		errCode        string
		wantMessageID  int64
	}{
		{
			name: "successful send with notification chat_id",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "123456789",
				Text:   "Hello, World!",
			},
			responseBody:   `{"ok": true, "result": {"message_id": 123, "chat": {"id": 123456789}}}`,
			responseStatus: http.StatusOK,
			wantMessageID:  123,
		},
		{
			name: "successful send with config chat_id",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				ChatID:   "987654321",
				Timeout:  5,
			},
			notification: &domain.Notification{
				Text: "Hello, World!",
			},
			responseBody:   `{"ok": true, "result": {"message_id": 456, "chat": {"id": 987654321}}}`,
			responseStatus: http.StatusOK,
			wantMessageID:  456,
		},
		{
			name: "empty text validation",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				ChatID:   "test-chat-id",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "123",
				Text:   "",
			},
			wantErr: true,
			errCode: domain.ErrValidation,
		},
		{
			name: "empty chat_id validation",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "",
				Text:   "Valid text",
			},
			wantErr: true,
			errCode: domain.ErrValidation,
		},
		{
			name: "telegram API error",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				ChatID:   "test-chat-id",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "123",
				Text:   "Test",
			},
			responseBody:   `{"ok": false, "description": "Bad Request: chat not found"}`,
			responseStatus: http.StatusOK,
			wantErr:        true,
			errCode:        domain.ErrExternalService,
		},
		{
			name: "HTTP client error",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				ChatID:   "test-chat-id",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "123",
				Text:   "Test",
			},
			roundTripErr: errors.New("network error"),
			wantErr:      true,
			errCode:      domain.ErrExternalService,
		},
		{
			name: "JSON marshal error",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				ChatID:   "test-chat-id",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "123",
				Text:   string([]byte{0xff, 0xfe, 0xfd}), // Invalid UTF-8
			},
			wantErr: true,
			errCode: domain.ErrExternalService,
		},
		{
			name: "HTTP status not OK",
			config: &config.TelegramConfig{
				BotToken: "test-bot-token",
				ChatID:   "test-chat-id",
				Timeout:  5,
			},
			notification: &domain.Notification{
				ChatID: "123",
				Text:   "Test",
			},
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"ok": false}`,
			wantErr:        true,
			errCode:        domain.ErrExternalService,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Используем newTelegramAdapterWithMock из mocks_test.go
			adapter := newTelegramAdapterWithMock(tt.config, func(req *http.Request) (*http.Response, error) {
				if tt.roundTripErr != nil {
					return nil, tt.roundTripErr
				}

				return &http.Response{
					StatusCode: tt.responseStatus,
					Body:       io.NopCloser(bytes.NewBufferString(tt.responseBody)),
					Header:     make(http.Header),
				}, nil
			})

			// Вызываем тестируемый метод
			ctx := context.Background()
			result, err := adapter.Send(ctx, tt.notification)

			// Проверяем результаты
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}

				// Проверяем тип ошибки
				var domainErr *domain.DomainError
				if errors.As(err, &domainErr) {
					if domainErr.Code != tt.errCode {
						t.Errorf("Expected error code %s, got %s", tt.errCode, domainErr.Code)
					}
				} else if tt.errCode != "" {
					t.Errorf("Expected DomainError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if result == nil {
					t.Error("Expected result, got nil")
					return
				}
				if result.MessageID != tt.wantMessageID {
					t.Errorf("Expected MessageID %d, got %d", tt.wantMessageID, result.MessageID)
				}
				if result.SentAt.IsZero() {
					t.Error("Expected SentAt to be set")
				}
			}
		})
	}
}

func TestTelegramAdapter_Send_RequestValidation(t *testing.T) {
	// Тест на корректность формирования запроса
	adapter := newTelegramAdapterWithMock(newTestTelegramConfig(), func(req *http.Request) (*http.Response, error) {
		// Проверяем URL
		expectedURL := "https://api.telegram.org/bottest-bot-token/sendMessage"
		if req.URL.String() != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, req.URL.String())
		}

		// Проверяем метод
		if req.Method != "POST" {
			t.Errorf("Expected POST method, got %s", req.Method)
		}

		// Проверяем заголовки
		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json")
		}

		// Читаем тело и проверяем его
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		var requestBody map[string]interface{}
		if err := json.Unmarshal(body, &requestBody); err != nil {
			t.Fatalf("Failed to unmarshal request body: %v", err)
		}

		if requestBody["chat_id"] != "123456" {
			t.Errorf("Expected chat_id 123456, got %v", requestBody["chat_id"])
		}
		if requestBody["text"] != "Test message" {
			t.Errorf("Expected text 'Test message', got %v", requestBody["text"])
		}

		// Возвращаем успешный ответ
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(bytes.NewBufferString(
				`{"ok": true, "result": {"message_id": 1, "chat": {"id": 123456}}}`,
			)),
			Header: make(http.Header),
		}, nil
	})

	_, err := adapter.Send(context.Background(), &domain.Notification{
		ChatID: "123456",
		Text:   "Test message",
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestTelegramAdapter_HealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		roundTripErr   error
		wantErr        bool
		errContains    string
	}{
		{
			name:           "health check successful",
			responseBody:   `{"ok": true, "result": {"id": 123, "is_bot": true, "first_name": "TestBot"}}`,
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "health check failed - HTTP 401 error",
			responseBody:   `{"ok": false, "description": "Unauthorized"}`,
			responseStatus: http.StatusUnauthorized,
			wantErr:        true,
			errContains:    "status: 401",
		},
		{
			name:           "health check failed - HTTP 500 error",
			responseBody:   `{"error": "Internal server error"}`,
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "status: 500",
		},
		{
			name:         "health check failed - network error",
			roundTripErr: errors.New("network timeout"),
			wantErr:      true,
			errContains:  "Telegram API unavailable",
		},
		{
			name:           "health check failed - telegram API error (ok: false)",
			responseBody:   `{"ok": false, "description": "Unauthorized"}`,
			responseStatus: http.StatusOK,
			wantErr:        true,
			errContains:    "Telegram API error: Unauthorized",
		},
		{
			name:           "health check failed - invalid JSON",
			responseBody:   `{invalid json}`,
			responseStatus: http.StatusOK,
			wantErr:        true,
			errContains:    "failed to parse response",
		},
		{
			name:           "health check failed - empty response",
			responseBody:   "", // Пустое тело при статусе 200
			responseStatus: http.StatusOK,
			wantErr:        true,
			errContains:    "empty response",
		},
		{
			name:           "health check failed - HTTP error with empty body",
			responseBody:   "", // Пустое тело при ошибке
			responseStatus: http.StatusBadRequest,
			wantErr:        true,
			errContains:    "status: 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.TelegramConfig{
				BotToken: "test-token",
				Timeout:  5,
				Debug:    false, // Отключаем debug для чистого вывода
			}

			adapter := newTelegramAdapterWithMock(config, func(req *http.Request) (*http.Response, error) {
				if tt.roundTripErr != nil {
					return nil, tt.roundTripErr
				}

				return &http.Response{
					StatusCode: tt.responseStatus,
					Body:       io.NopCloser(bytes.NewBufferString(tt.responseBody)),
					Header:     make(http.Header),
				}, nil
			})

			err := adapter.HealthCheck()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Test '%s': Expected error, got nil", tt.name)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Test '%s': Expected error containing '%s', got: %v", tt.name, tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s': Unexpected error: %v", tt.name, err)
				}
			}
		})
	}
}

func TestNewTelegramAdapter(t *testing.T) {
	cfg := &config.TelegramConfig{
		BotToken: "token",
		Timeout:  10,
	}

	adapter := NewTelegramAdapter(cfg, nil)

	if adapter == nil {
		t.Fatal("Expected adapter, got nil")
	}

	if adapter.config != cfg {
		t.Error("Config not set properly")
	}

	if adapter.client == nil {
		t.Error("HTTP client not initialized")
	}

	// Проверяем таймаут клиента
	expectedTimeout := time.Duration(cfg.Timeout) * time.Second
	if adapter.client.Timeout != expectedTimeout {
		t.Errorf("Expected timeout %v, got %v", expectedTimeout, adapter.client.Timeout)
	}
}