package api

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/mdemidenko/monitoring-platform/config"
    "github.com/mdemidenko/monitoring-platform/internal/domain"
    "github.com/mdemidenko/monitoring-platform/internal/middleware"
    "github.com/stretchr/testify/assert"
)

// TestNotificationService — мок для domain.NotificationService
type TestNotificationService struct {
    SendNotificationFunc         func(ctx context.Context, chatID, text string) (*domain.SentNotification, error)
    ProcessWithIntervalsFunc     func(ctx context.Context, notifications []*domain.Notification, interval time.Duration, workers int) domain.ProcessResult
    HealthCheckFunc              func() error
    GetNotificationsFunc         func() []*domain.Notification
    GetSentNotificationsFunc     func() []*domain.SentNotification
    GetStatsFunc                 func() *domain.ServiceStats
    ProcessEntityFunc            func(ctx context.Context, entity interface{}) error
}

func (s *TestNotificationService) SendNotification(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
    if s.SendNotificationFunc != nil {
        return s.SendNotificationFunc(ctx, chatID, text)
    }
    return nil, nil
}

func (s *TestNotificationService) ProcessWithIntervals(ctx context.Context, notifications []*domain.Notification, interval time.Duration, workers int) domain.ProcessResult {
    if s.ProcessWithIntervalsFunc != nil {
        return s.ProcessWithIntervalsFunc(ctx, notifications, interval, workers)
    }
    return domain.ProcessResult{
        SuccessCount: len(notifications),
        ErrorCount:   0,
    }
}

func (s *TestNotificationService) HealthCheck() error {
    if s.HealthCheckFunc != nil {
        return s.HealthCheckFunc()
    }
    return nil
}

func (s *TestNotificationService) GetNotifications() []*domain.Notification {
    if s.GetNotificationsFunc != nil {
        return s.GetNotificationsFunc()
    }
    return nil
}

func (s *TestNotificationService) GetSentNotifications() []*domain.SentNotification {
    if s.GetSentNotificationsFunc != nil {
        return s.GetSentNotificationsFunc()
    }
    return nil
}

func (s *TestNotificationService) GetStats() *domain.ServiceStats {
    if s.GetStatsFunc != nil {
        return s.GetStatsFunc()
    }
    return nil
}

func (s *TestNotificationService) ProcessEntity(ctx context.Context, entity interface{}) error {
    if s.ProcessEntityFunc != nil {
        return s.ProcessEntityFunc(ctx, entity)
    }
    return nil
}

// Проверка на этапе компиляции, что TestNotificationService реализует domain.NotificationService
var _ domain.NotificationService = (*TestNotificationService)(nil)

// newTestConfig возвращает тестовую конфигурацию
func newTestConfig() *config.Config {
    return &config.Config{
        App: config.AppConfig{
            Name:        "test-app",
            Version:     "1.0.0",
            Environment: "test",
        },
        Auth: config.AuthConfig{
            JWTSecret: "test-secret-key-for-jwt-signing-32-chars", // 32+ символов
            Login:  "testuser",
            Password:  "testpass123",
        },
        Server: config.ServerConfig{
            Port:         "8080",
            Host:         "",
            GinMode:      "test",
            EnableCORS:   false,
            TrustedProxies: []string{},
        },
        Telegram: config.TelegramConfig{
            ChatID: "123456",
        },
    }
}

// newTestHandler создаёт Handler с моком
func newTestHandler(service *TestNotificationService, cfg *config.Config) *Handler {
    return &Handler{
        notificationService: service,
        cfg:                 cfg,
    }
}

// setupAuthRouter настраивает роутер с аутентификацией
func setupAuthRouter(handler *Handler) *gin.Engine {
    gin.SetMode(gin.TestMode)
    router := gin.New()
    router.Use(gin.Recovery())

    // Публичные роуты
    router.POST("/api/auth/login", handler.LoginHandler)
    router.GET("/api/health", handler.HealthHandler)

    // Защищённые роуты
    protected := router.Group("/api")
    protected.Use(middleware.AuthMiddleware(handler.cfg.Auth.JWTSecret))
    {
        protected.POST("/send", handler.SendHandler)
        protected.POST("/batch", handler.BatchHandler)
        protected.GET("/notifications", handler.NotificationsHandler)
        protected.GET("/notifications/sent", handler.SentNotificationsHandler)
        protected.GET("/status", handler.StatusHandler)
    }

    return router
}

// getAuthToken получает JWT токен через /api/auth/login
func getAuthToken(t *testing.T, router *gin.Engine, username, password string) string {
    loginData := map[string]string{
        "username": username,
        "password": password,
    }
    body, _ := json.Marshal(loginData)

    req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var response map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &response)
    assert.NoError(t, err)

    token, ok := response["token"].(string)
    assert.True(t, ok, "token should be present in response")
    assert.NotEmpty(t, token, "token should not be empty")

    return token
}