package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"github.com/mdemidenko/monitoring-platform/internal/domain"

	"github.com/mdemidenko/monitoring-platform/config"
    "github.com/mdemidenko/monitoring-platform/internal/core"
    "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// MockNotificationRepository — мок для domain.NotificationRepository
type MockNotificationRepository struct {
    mock.Mock
}

func (m *MockNotificationRepository) Store(entity interface{}) error {
    args := m.Called(entity)
    return args.Error(0)
}

func (m *MockNotificationRepository) GetNotifications() []*domain.Notification {
    args := m.Called()
    return args.Get(0).([]*domain.Notification)
}

func (m *MockNotificationRepository) GetSentNotifications() []*domain.SentNotification {
    args := m.Called()
    return args.Get(0).([]*domain.SentNotification)
}

func (m *MockNotificationRepository) GetStats() *domain.ServiceStats {
    args := m.Called()
    return args.Get(0).(*domain.ServiceStats)
}

type MockNotificationSender struct {
    mock.Mock
}

func (m *MockNotificationSender) Send(ctx context.Context, notification *domain.Notification) (*domain.SentNotification, error) {
    args := m.Called(ctx, notification)
    return args.Get(0).(*domain.SentNotification), args.Error(1)
}

func (m *MockNotificationSender) HealthCheck() error {
    args := m.Called()
    return args.Error(0)
}
func TestNewServer(t *testing.T) {
	cfg := newTestConfig()
	mockService := &TestNotificationService{}
	
	// Создаем сервер с моком
	server := NewServer(mockService, cfg)
	
	if server == nil {
		t.Fatal("Expected server instance, got nil")
	}
	
	if server.router == nil {
		t.Error("Expected router to be initialized")
	}
	
	if server.handler == nil {
		t.Error("Expected handler to be initialized")
	}
	
	if server.cfg != cfg {
		t.Error("Expected config to be set")
	}
}

func TestServer_setupMiddleware(t *testing.T) {
	cfg := newTestConfig()
	mockService := &TestNotificationService{
		HealthCheckFunc: func() error { return nil },
		GetStatsFunc: func() *domain.ServiceStats {
			return &domain.ServiceStats{
				TotalNotifications:     5,
				TotalSentNotifications: 3,
				PendingNotifications:   2,
			}
		},
	}
	
	server := NewServer(mockService, cfg)
	
	// Проверяем что middleware настроены
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	
	server.router.ServeHTTP(w, req)
	
	// Должен быть 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestServer_Routes(t *testing.T) {
	cfg := newTestConfig()
	mockService := &TestNotificationService{
		HealthCheckFunc: func() error { return nil },
		GetStatsFunc: func() *domain.ServiceStats {
			return &domain.ServiceStats{
				TotalNotifications:     5,
				TotalSentNotifications: 3,
				PendingNotifications:   2,
			}
		},
	}
	
	server := NewServer(mockService, cfg)
	
	tests := []struct {
		method string
		path   string
		wantStatus int
	}{
		{"GET", "/", http.StatusOK},
		{"GET", "/api/health", http.StatusOK},
		{"POST", "/api/auth/login", http.StatusBadRequest}, // Нет тела
		{"POST", "/api/send", http.StatusUnauthorized}, // Нет аутентификации
		{"POST", "/api/batch", http.StatusUnauthorized}, // Нет аутентификации
		{"GET", "/api/notifications", http.StatusUnauthorized}, // Нет аутентификации
		{"GET", "/api/notifications/sent", http.StatusUnauthorized}, // Нет аутентификации
		{"GET", "/api/status", http.StatusUnauthorized}, // Нет аутентификации
		{"GET", "/swagger/index.html", http.StatusOK}, // Swagger redirect
		{"GET", "/nonexistent", http.StatusNotFound}, // 404
	}
	
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.method == "POST" && (tt.path == "/api/auth/login" || tt.path == "/api/send" || tt.path == "/api/batch") {
				req.Header.Set("Content-Type", "application/json")
			}
			
			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)
			
			if w.Code != tt.wantStatus {
				t.Errorf("For %s %s: expected status %d, got %d", tt.method, tt.path, tt.wantStatus, w.Code)
			}
		})
	}
}

func TestServer_Shutdown(t *testing.T) {
	cfg := newTestConfig()
	mockService := &TestNotificationService{}
	
	server := NewServer(mockService, cfg)
	
	// Проверяем что Shutdown не паникует
	ctx := context.Background()
	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown should not return error for test server, got: %v", err)
	}
}

func TestServer_RootHandler(t *testing.T) {
	cfg := newTestConfig()
	mockService := &TestNotificationService{}
	
	server := NewServer(mockService, cfg)
	
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	server.router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	expectedFields := []string{"service", "version", "status", "docs", "api"}
	for _, field := range expectedFields {
		if _, ok := response[field]; !ok {
			t.Errorf("Missing field in root response: %s", field)
		}
	}
}

func TestServer_setGinMode(t *testing.T) {
    tests := []struct {
        name     string
        ginMode  string
        expected string
    }{
        {"release", "release", gin.ReleaseMode},
        {"test", "test", gin.TestMode},
        {"debug", "debug", gin.DebugMode},
        {"empty", "", gin.DebugMode},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := newTestConfig()
            cfg.Server.GinMode = tt.ginMode

            origMode := gin.Mode()
            defer gin.SetMode(origMode) // восстанавливаем режим

            setGinMode(cfg)

            assert.Equal(t, tt.expected, gin.Mode())
        })
    }
}

func TestServer_Shutdown_WithoutServer(t *testing.T) {
    server := &Server{} // httpServer == nil
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    err := server.Shutdown(ctx)
    assert.NoError(t, err)
}

func TestServer_Start_InitializesHTTPServer(t *testing.T) {
    cfg := &config.Config{
        Server: config.ServerConfig{
            Host: "127.0.0.1",
            Port: "8081",
        },
    }

    // Моки
    mockRepo := &MockNotificationRepository{}
    mockSender := &MockNotificationSender{}

    // Ожидания
    mockRepo.On("GetNotifications").Return([]*domain.Notification{})
    mockRepo.On("GetSentNotifications").Return([]*domain.SentNotification{})
    mockSender.On("HealthCheck").Return(nil)

    // Service
    notificationService := core.NewNotificationService(mockRepo, mockSender, nil)

    // Server
    server := NewServer(notificationService, cfg)

    // Запуск
    started := make(chan bool, 1)
    go func() {
        server.Start(cfg.Server.Port)
        close(started)
    }()

    // Даем время на инициализацию
    time.Sleep(100 * time.Millisecond)

    // Проверяем эндпоинт
    resp, err := http.Get("http://127.0.0.1:8081/api/health")
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    resp.Body.Close()

    // Останавливаем
    err = server.Shutdown(context.Background())
    require.NoError(t, err)

    // Ждём
    <-started

    // Проверяем моки
    mockRepo.AssertExpectations(t)
    mockSender.AssertExpectations(t)
}