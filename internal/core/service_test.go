package core

import (
    "context"
    "testing"
    "time"

    "github.com/mdemidenko/monitoring-platform/internal/domain"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// MockRepository — мок для domain.NotificationRepository
type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) Store(entity interface{}) error {
    args := m.Called(entity)
    return args.Error(0)
}

func (m *MockRepository) GetNotifications() []*domain.Notification {
    args := m.Called()
    return args.Get(0).([]*domain.Notification)
}

func (m *MockRepository) GetSentNotifications() []*domain.SentNotification {
    args := m.Called()
    return args.Get(0).([]*domain.SentNotification)
}

// MockSender — мок для domain.NotificationSender
type MockSender struct {
    mock.Mock
}

func (m *MockSender) Send(ctx context.Context, notification *domain.Notification) (*domain.SentNotification, error) {
    args := m.Called(ctx, notification)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*domain.SentNotification), args.Error(1)
}

func (m *MockSender) HealthCheck() error {
    args := m.Called()
    return args.Error(0)
}

// TestNewNotificationService проверяет создание сервиса
func TestNewNotificationService(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    service := NewNotificationService(repo, sender, nil)

    assert.NotNil(t, service)
    assert.Equal(t, repo, service.repo)
    assert.Equal(t, sender, service.sender)
}

// TestNotificationService_SendNotification проверяет отправку уведомления
func TestNotificationService_SendNotification(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    notification := &domain.Notification{
        ChatID: "123",
        Text:   "Test message",
    }

    sentNotification := &domain.SentNotification{
        MessageID: 123,
        ChatID:    456,
        SentAt:    time.Now(),
    }

    // Ожидания
    repo.On("Store", notification).Return(nil)
    sender.On("Send", mock.Anything, notification).Return(sentNotification, nil)
    repo.On("Store", sentNotification).Return(nil)

    service := NewNotificationService(repo, sender, nil)
    result, err := service.SendNotification(context.Background(), "123", "Test message")

    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, sentNotification.MessageID, result.MessageID)
    assert.Equal(t, sentNotification.ChatID, result.ChatID)

    // Проверяем, что моки вызваны
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

// TestNotificationService_SendNotification_ValidationError проверяет валидацию
func TestNotificationService_SendNotification_ValidationError(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    service := NewNotificationService(repo, sender, nil)

    _, err := service.SendNotification(context.Background(), "", "Test")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "chat_id cannot be empty")

    _, err = service.SendNotification(context.Background(), "123", "")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "text cannot be empty")
}

// TestNotificationService_ProcessEntity проверяет обработку Notification
func TestNotificationService_ProcessEntity(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    notification := &domain.Notification{
        ChatID: "123",
        Text:   "Test notification",
    }

    sentNotification := &domain.SentNotification{
        MessageID: 123,
        ChatID:    456,
    }

    // Ожидания
    repo.On("Store", notification).Return(nil)
    sender.On("Send", mock.Anything, notification).Return(sentNotification, nil)
    repo.On("Store", sentNotification).Return(nil)

    service := NewNotificationService(repo, sender, nil)
    err := service.ProcessEntity(context.Background(), notification)

    assert.NoError(t, err)
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

// TestNotificationService_ProcessEntity_SentNotification проверяет обработку SentNotification
func TestNotificationService_ProcessEntity_SentNotification(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    entity := &domain.SentNotification{
        MessageID: 123,
        ChatID:    456,
    }

    repo.On("Store", entity).Return(nil)

    service := NewNotificationService(repo, sender, nil)
    err := service.ProcessEntity(context.Background(), entity)

    assert.NoError(t, err)
    repo.AssertExpectations(t)
    sender.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)
}

// TestNotificationService_ProcessWithIntervals проверяет пакетную отправку
func TestNotificationService_ProcessWithIntervals(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    notifications := []*domain.Notification{
        {ChatID: "123", Text: "Msg1"},
        {ChatID: "456", Text: "Msg2"},
    }

    sentNotification := &domain.SentNotification{
        MessageID: 123,
        ChatID:    456,
        SentAt:    time.Now(),
    }

    // Ожидания
    repo.On("Store", mock.AnythingOfType("*domain.Notification")).Return(nil).Twice()
    sender.On("Send", mock.Anything, mock.AnythingOfType("*domain.Notification")).Return(sentNotification, nil).Twice()
    repo.On("Store", mock.AnythingOfType("*domain.SentNotification")).Return(nil).Twice()

    service := NewNotificationService(repo, sender, nil)
    result := service.ProcessWithIntervals(context.Background(), notifications, 10*time.Millisecond, 2)

    assert.Equal(t, 2, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)
}

// TestNotificationService_HealthCheck проверяет здоровье
func TestNotificationService_HealthCheck(t *testing.T) {
    repo := &MockRepository{}
    sender := &MockSender{}

    sender.On("HealthCheck").Return(nil)

    service := NewNotificationService(repo, sender, nil)
    err := service.HealthCheck()

    assert.NoError(t, err)
    sender.AssertExpectations(t)
}