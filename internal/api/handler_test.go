package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"strings"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/mdemidenko/monitoring-platform/internal/domain"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	
	// Тестовые маршруты (без middleware)
	router.GET("/api/health", handler.HealthHandler)
	router.POST("/api/auth/login", handler.LoginHandler)
	router.POST("/api/send", handler.SendHandler)
	router.POST("/api/batch", handler.BatchHandler)
	router.GET("/api/notifications", handler.NotificationsHandler)
	router.GET("/api/notifications/sent", handler.SentNotificationsHandler)
	router.GET("/api/status", handler.StatusHandler)
	
	return router
}

func TestHandler_HealthHandler(t *testing.T) {
	tests := []struct {
		name           string
		healthCheckErr error
		wantStatus     int
		wantSuccess    bool
	}{
		{
			name:        "health check successful",
			wantStatus:  http.StatusOK,
			wantSuccess: true,
		},
		{
			name:           "health check failed",
			healthCheckErr: domain.NewDomainError(domain.ErrExternalService, "Telegram API unavailable", nil),
			wantStatus:     http.StatusServiceUnavailable,
			wantSuccess:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем мок сервиса
			mockService := &TestNotificationService{
				HealthCheckFunc: func() error {
					return tt.healthCheckErr
				},
				GetStatsFunc: func() *domain.ServiceStats {
					return &domain.ServiceStats{
						TotalNotifications:     5,
						TotalSentNotifications: 3,
						PendingNotifications:   2,
					}
				},
			}

			// Создаем хендлер и роутер
			cfg := newTestConfig()
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			// Создаем запрос
			req := httptest.NewRequest("GET", "/api/health", nil)
			w := httptest.NewRecorder()
			
			// Выполняем запрос
			router.ServeHTTP(w, req)

			// Проверяем статус код
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Парсим ответ
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Для успешного ответа проверяем структуру
			if tt.wantSuccess {
				if status, ok := response["status"].(string); !ok || status != "ok" {
					t.Errorf("Expected status 'ok', got %v", response["status"])
				}
				
				// Проверяем наличие полей
				requiredFields := []string{"status", "timestamp", "app", "version", "storage"}
				for _, field := range requiredFields {
					if _, ok := response[field]; !ok {
						t.Errorf("Missing required field: %s", field)
					}
				}
			}
		})
	}
}

func TestHandler_LoginHandler(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
		wantToken  bool
	}{
		{
			name:       "successful login",
			username:   "testuser",
			password:   "testpass123",
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name:       "wrong username",
			username:   "wronguser",
			password:   "testpass123",
			wantStatus: http.StatusUnauthorized,
			wantToken:  false,
		},
		{
			name:       "wrong password",
			username:   "testuser",
			password:   "wrongpass",
			wantStatus: http.StatusUnauthorized,
			wantToken:  false,
		},
		{
			name:       "empty username",
			username:   "",
			password:   "testpass123",
			wantStatus: http.StatusBadRequest,
			wantToken:  false,
		},
		{
			name:       "empty password",
			username:   "testuser",
			password:   "",
			wantStatus: http.StatusBadRequest,
			wantToken:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			mockService := &TestNotificationService{}
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			// Подготавливаем запрос
			loginData := map[string]string{
				"username": tt.username,
				"password": tt.password,
			}
			body, _ := json.Marshal(loginData)
			
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Проверяем статус код
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Для успешного логина проверяем наличие токена
			if tt.wantToken {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if token, ok := response["token"].(string); !ok || token == "" {
					t.Error("Expected JWT token in response")
				}
				if tokenType, ok := response["token_type"].(string); !ok || tokenType != "Bearer" {
					t.Error("Expected token_type 'Bearer'")
				}
			}
		})
	}
}

func TestHandler_SendHandler(t *testing.T) {
	tests := []struct {
		name        string
		requestBody map[string]interface{}
		mockError   error
		wantStatus  int
	}{
		{
			name: "successful send with chat_id",
			requestBody: map[string]interface{}{
				"chat_id": "123456789",
				"text":    "Test message",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "successful send without chat_id (uses config)",
			requestBody: map[string]interface{}{
				"text": "Test message",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "empty text validation",
			requestBody: map[string]interface{}{
				"chat_id": "123",
				"text":    "",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing text field",
			requestBody: map[string]interface{}{
				"chat_id": "123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validation error from service",
			requestBody: map[string]interface{}{
				"text": "Test",
			},
			mockError:  domain.NewDomainError(domain.ErrValidation, "text too long", nil),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "external service error",
			requestBody: map[string]interface{}{
				"text": "Test",
			},
			mockError:  domain.NewDomainError(domain.ErrExternalService, "Telegram API error", nil),
			wantStatus: http.StatusBadGateway,
		},
		{
			name: "internal service error",
			requestBody: map[string]interface{}{
				"text": "Test",
			},
			mockError:  domain.NewDomainError(domain.ErrRepository, "storage error", nil),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем мок сервиса
			mockService := &TestNotificationService{
				SendNotificationFunc: func(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return &domain.SentNotification{
						MessageID: 123,
						ChatID:    456,
						SentAt:    time.Now(),
					}, nil
				},
			}

			cfg := newTestConfig()
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			// Подготавливаем запрос
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/send", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Проверяем статус код
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Для успешного ответа проверяем структуру
			if w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if success, ok := response["success"].(bool); !ok || !success {
					t.Error("Expected success: true")
				}
			}
		})
	}
}

func TestHandler_BatchHandler(t *testing.T) {
	tests := []struct {
		name        string
		requestBody map[string]interface{}
		wantStatus  int
	}{
		{
			name: "successful batch with all parameters",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"chat_id": "123", "text": "Message 1"},
					{"chat_id": "456", "text": "Message 2"},
				},
				"interval_ms": 1000,
				"workers":     2,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "successful batch without optional parameters",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"text": "Message 1"},
					{"text": "Message 2"},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "empty messages array",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing messages field",
			requestBody: map[string]interface{}{
				"interval_ms": 1000,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "message with empty text",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"text": ""},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid interval (negative)",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"text": "Test"},
				},
				"interval_ms": -100,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid workers (zero)",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"text": "Test"},
				},
				"workers": 0,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "too many workers",
			requestBody: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"text": "Test"},
				},
				"workers": 15,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &TestNotificationService{
				ProcessWithIntervalsFunc: func(ctx context.Context, notifications []*domain.Notification, interval time.Duration, workers int) domain.ProcessResult {
					return domain.ProcessResult{
						SuccessCount: len(notifications),
						ErrorCount:   0,
					}
				},
			}

			cfg := newTestConfig()
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/batch", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if success, ok := response["success"].(bool); !ok || !success {
					t.Error("Expected success: true")
				}
			}
		})
	}
}

func TestHandler_NotificationsHandler(t *testing.T) {
	tests := []struct {
		name         string
		mockNotifications []*domain.Notification
		wantStatus   int
		wantCount    int
	}{
		{
			name: "successful get with notifications",
			mockNotifications: []*domain.Notification{
				{ChatID: "123", Text: "Message 1"},
				{ChatID: "456", Text: "Message 2"},
				{ChatID: "789", Text: "Message 3"},
			},
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
		{
			name:         "successful get with empty list",
			mockNotifications: []*domain.Notification{},
			wantStatus:   http.StatusOK,
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &TestNotificationService{
				GetNotificationsFunc: func() []*domain.Notification {
					return tt.mockNotifications
				},
			}

			cfg := newTestConfig()
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			req := httptest.NewRequest("GET", "/api/notifications", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				// Проверяем структуру ответа
				if success, ok := response["success"].(bool); !ok || !success {
					t.Error("Expected success: true")
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected data field")
				}

				if count, ok := data["count"].(float64); !ok || int(count) != tt.wantCount {
					t.Errorf("Expected count %d, got %v", tt.wantCount, count)
				}
			}
		})
	}
}

func TestHandler_SentNotificationsHandler(t *testing.T) {
	tests := []struct {
		name         string
		mockSentNotifications []*domain.SentNotification
		wantStatus   int
		wantCount    int
	}{
		{
			name: "successful get with sent notifications",
			mockSentNotifications: []*domain.SentNotification{
				{MessageID: 1, ChatID: 123},
				{MessageID: 2, ChatID: 456},
				{MessageID: 3, ChatID: 789},
			},
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
		{
			name:         "successful get with empty list",
			mockSentNotifications: []*domain.SentNotification{},
			wantStatus:   http.StatusOK,
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &TestNotificationService{
				GetSentNotificationsFunc: func() []*domain.SentNotification {
					return tt.mockSentNotifications
				},
			}

			cfg := newTestConfig()
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			req := httptest.NewRequest("GET", "/api/notifications/sent", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if success, ok := response["success"].(bool); !ok || !success {
					t.Error("Expected success: true")
				}
			}
		})
	}
}

func TestHandler_StatusHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockStats  *domain.ServiceStats
		wantStatus int
	}{
		{
			name: "successful status with data",
			mockStats: &domain.ServiceStats{
				TotalNotifications:     10,
				TotalSentNotifications: 7,
				PendingNotifications:   3,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "successful status with zero data",
			mockStats: &domain.ServiceStats{
				TotalNotifications:     0,
				TotalSentNotifications: 0,
				PendingNotifications:   0,
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &TestNotificationService{
				GetStatsFunc: func() *domain.ServiceStats {
					return tt.mockStats
				},
			}

			cfg := newTestConfig()
			handler := newTestHandler(mockService, cfg)
			router := setupTestRouter(handler)

			req := httptest.NewRequest("GET", "/api/status", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if success, ok := response["success"].(bool); !ok || !success {
					t.Error("Expected success: true")
				}

				// Проверяем наличие статистики
				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected data field")
				}

				stats, ok := data["stats"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected stats field")
				}

				expectedFields := []string{"total_notifications", "total_sent_notifications", "pending_notifications"}
				for _, field := range expectedFields {
					if _, ok := stats[field]; !ok {
						t.Errorf("Missing stat field: %s", field)
					}
				}
			}
		})
	}
}

func TestHandler_SendHandler_InternalServerError(t *testing.T) {
    mockService := &TestNotificationService{
        SendNotificationFunc: func(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
            return nil, errors.New("unexpected error") // не domain.Error → вызовет InternalServerError
        },
    }
    cfg := newTestConfig()
    handler := newTestHandler(mockService, cfg)
    router := setupTestRouter(handler)

    body := `{"text": "test"}`
    req := httptest.NewRequest("POST", "/api/send", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusInternalServerError, w.Code)
    var resp ErrorResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, "Internal Server Error", resp.ErrorType)
}

func TestHandler_SendHandler_BadGatewayError(t *testing.T) {
    mockService := &TestNotificationService{
        SendNotificationFunc: func(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
            return nil, domain.NewDomainError(domain.ErrExternalService, "telegram api down", nil)
        },
    }
    cfg := newTestConfig()
    handler := newTestHandler(mockService, cfg)
    router := setupTestRouter(handler)

    body := `{"text": "test"}`
    req := httptest.NewRequest("POST", "/api/send", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusBadGateway, w.Code)
    var resp ErrorResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, "Bad Gateway", resp.ErrorType)
}

func TestInternalServerError(t *testing.T) {
    err := InternalServerError("test error")
    assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
    assert.Equal(t, "Internal Server Error", err.ErrorType)
}

func TestBadGatewayError(t *testing.T) {
    err := BadGatewayError("service down")
    assert.Equal(t, http.StatusBadGateway, err.StatusCode)
    assert.Equal(t, "Bad Gateway", err.ErrorType)
}