package grpc

import (
    "context"
    "time"

    "github.com/mdemidenko/monitoring-platform/config"
    "github.com/mdemidenko/monitoring-platform/internal/domain"

)

// MockNotificationService — мок для core.NotificationService
type MockNotificationService struct {
    SendNotificationFunc           func(ctx context.Context, chatID, text string) (*domain.SentNotification, error)
    ProcessWithIntervalsFunc       func(ctx context.Context, notifications []*domain.Notification, interval time.Duration, workers int) domain.ProcessResult
    GetNotificationsFunc           func() []*domain.Notification
    GetSentNotificationsFunc       func() []*domain.SentNotification
    ProcessEntityFunc              func(ctx context.Context, entity interface{}) error
    HealthCheckFunc                func() error
    GetStatsFunc                   func() *domain.ServiceStats
}

func (m *MockNotificationService) SendNotification(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
    if m.SendNotificationFunc != nil {
        return m.SendNotificationFunc(ctx, chatID, text)
    }
    return &domain.SentNotification{
        MessageID: 123,
        ChatID:    456,
        SentAt:    time.Now(),
    }, nil
}


func (m *MockNotificationService) ProcessWithIntervals(ctx context.Context, notifications []*domain.Notification, interval time.Duration, workers int) domain.ProcessResult {
    if m.ProcessWithIntervalsFunc != nil {
        return m.ProcessWithIntervalsFunc(ctx, notifications, interval, workers)
    }
    return domain.ProcessResult{
        SuccessCount: len(notifications),
        ErrorCount:   0,
    }
}

func (m *MockNotificationService) GetNotifications() []*domain.Notification {
    if m.GetNotificationsFunc != nil {
        return m.GetNotificationsFunc()
    }
    return nil
}

func (m *MockNotificationService) GetSentNotifications() []*domain.SentNotification {
    if m.GetSentNotificationsFunc != nil {
        return m.GetSentNotificationsFunc()
    }
    return nil
}

func (m *MockNotificationService) ProcessEntity(ctx context.Context, entity interface{}) error {
    if m.ProcessEntityFunc != nil {
        return m.ProcessEntityFunc(ctx, entity)
    }
    return nil
}

func (m *MockNotificationService) HealthCheck() error {
    if m.HealthCheckFunc != nil {
        return m.HealthCheckFunc()
    }
    return nil
}

func (m *MockNotificationService) GetStats() *domain.ServiceStats {
    if m.GetStatsFunc != nil {
        return m.GetStatsFunc()
    }
    return &domain.ServiceStats{
        TotalNotifications:     0,
        TotalSentNotifications: 0,
        PendingNotifications:   0,
    }
}

// newTestConfig возвращает тестовую конфигурацию
func newTestConfig() *config.Config {
    return &config.Config{
        App: config.AppConfig{
            Name:        "test-app",
            Version:     "1.0.0",
            Environment: "test",
        },
        Auth: config.AuthConfig{
            Login:     "testuser",
            Password:  "testpass123",
            JWTSecret: "test-secret-key-for-jwt-signing-32-chars",
            JWTExpiration: 24,
        },
        Server: config.ServerConfig{
            Port:    "8080",
            Host:    "",
            GinMode: "test",
        },
        Telegram: config.TelegramConfig{
            ChatID: "123456",
        },
    }
}