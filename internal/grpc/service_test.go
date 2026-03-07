package grpc

import (
    "context"
    "testing"
    "time"
    "github.com/mdemidenko/monitoring-platform/internal/domain"
    "github.com/stretchr/testify/assert"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/emptypb"

    // Используем алиас для pb
    grpcPb "github.com/mdemidenko/monitoring-platform/pkg/grpc"
)

func TestMonitoringService_Login_Success(t *testing.T) {
    mockService := &MockNotificationService{}
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.LoginRequest{
        Username: "testuser",
        Password: "testpass123",
    }

    resp, err := service.Login(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.NotEmpty(t, resp.Token)
    assert.Equal(t, "Bearer", resp.TokenType)
    assert.NotEmpty(t, resp.ExpiresAt)
}

func TestMonitoringService_Login_InvalidUsername(t *testing.T) {
    mockService := &MockNotificationService{}
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.LoginRequest{
        Username: "wrong",
        Password: "testpass123",
    }

    _, err := service.Login(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
    assert.Contains(t, err.Error(), "invalid username or password")
}

func TestMonitoringService_Login_InvalidPassword(t *testing.T) {
    mockService := &MockNotificationService{}
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.LoginRequest{
        Username: "testuser",
        Password: "wrong",
    }

    _, err := service.Login(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
    assert.Contains(t, err.Error(), "invalid username or password")
}

func TestMonitoringService_CreateNotification_Success(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            _, ok := entity.(*domain.Notification)
            assert.True(t, ok, "expected *domain.Notification")
            return nil
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.Notification{
        ChatId: "123",
        Text:   "Test notification",
    }

    resp, err := service.CreateNotification(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, "Notification created successfully", resp.Message)
    assert.Equal(t, req, resp.Data)
}

func TestMonitoringService_CreateNotification_ProcessEntityError(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            return assert.AnError
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.Notification{
        ChatId: "123",
        Text:   "Test notification",
    }

    _, err := service.CreateNotification(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.Internal, status.Code(err))
    assert.Contains(t, err.Error(), "failed to store notification")
}

func TestMonitoringService_GetNotification_Found(t *testing.T) {
    mockService := &MockNotificationService{
        GetNotificationsFunc: func() []*domain.Notification {
            return []*domain.Notification{
                {ChatID: "123", Text: "Found message"},
            }
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.GetNotificationRequest{
        ChatId: "123",
    }

    resp, err := service.GetNotification(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, "123", resp.ChatId)
    assert.Equal(t, "Found message", resp.Text)
}

func TestMonitoringService_GetNotification_NotFound(t *testing.T) {
    mockService := &MockNotificationService{
        GetNotificationsFunc: func() []*domain.Notification {
            return []*domain.Notification{}
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.GetNotificationRequest{
        ChatId: "999",
    }

    _, err := service.GetNotification(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.NotFound, status.Code(err))
    assert.Contains(t, err.Error(), "notification not found")
}

func TestMonitoringService_ListNotifications(t *testing.T) {
    mockService := &MockNotificationService{
        GetNotificationsFunc: func() []*domain.Notification {
            return []*domain.Notification{
                {ChatID: "123", Text: "Msg1"},
                {ChatID: "456", Text: "Msg2"},
            }
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &emptypb.Empty{}

    resp, err := service.ListNotifications(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, int32(2), resp.Count)
    assert.Len(t, resp.Notifications, 2)
    assert.Equal(t, "Msg1", resp.Notifications[0].Text)
    assert.Equal(t, "Msg2", resp.Notifications[1].Text)
}

func TestMonitoringService_CreateSentNotification_Success(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            _, ok := entity.(*domain.SentNotification)
            assert.True(t, ok, "expected *domain.SentNotification")
            return nil
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.SentNotification{
        MessageId: 123,
        ChatId:    123,
    }

    resp, err := service.CreateSentNotification(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, "Sent notification created successfully", resp.Message)
    assert.Equal(t, req, resp.Data)
}

func TestMonitoringService_CreateSentNotification_Error(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            return assert.AnError
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.SentNotification{
        MessageId: 123,
        ChatId:    123,
    }

    _, err := service.CreateSentNotification(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.Internal, status.Code(err))
    assert.Contains(t, err.Error(), "failed to store sent notification")
}

func TestMonitoringService_GetSentNotification_Found(t *testing.T) {
    mockService := &MockNotificationService{
        GetSentNotificationsFunc: func() []*domain.SentNotification {
            return []*domain.SentNotification{
                {MessageID: 123, ChatID: 123},
            }
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.GetSentNotificationRequest{
        MessageId: 123,
    }

    resp, err := service.GetSentNotification(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, int64(123), resp.MessageId)
    // assert.Equal(t, "123", resp.ChatId)
}

func TestMonitoringService_GetSentNotification_NotFound(t *testing.T) {
    mockService := &MockNotificationService{
        GetSentNotificationsFunc: func() []*domain.SentNotification {
            return []*domain.SentNotification{}
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.GetSentNotificationRequest{
        MessageId: 999,
    }

    _, err := service.GetSentNotification(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.NotFound, status.Code(err))
    assert.Contains(t, err.Error(), "sent notification not found")
}

func TestMonitoringService_ListSentNotifications(t *testing.T) {
    mockService := &MockNotificationService{
        GetSentNotificationsFunc: func() []*domain.SentNotification {
            return []*domain.SentNotification{
                {MessageID: 123, ChatID: 123},
                {MessageID: 456, ChatID: 456},
            }
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &emptypb.Empty{}

    resp, err := service.ListSentNotifications(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, int32(2), resp.Count)
    assert.Len(t, resp.SentNotifications, 2)
    assert.Equal(t, int64(123), resp.SentNotifications[0].MessageId)
    assert.Equal(t, int64(456), resp.SentNotifications[1].MessageId)
}

func TestMonitoringService_CreateService_Success(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            return nil
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.Service{
        Id:             1,
        Name:           "Test Service",
        Tenant:         "Test Tenant",
        DeprecatedDate: "2025-01-01",
        BusinessLine:   "Test Line",
    }

    resp, err := service.CreateService(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, "Service created successfully (stub)", resp.Message)
    assert.Equal(t, req, resp.Data)
}

func TestMonitoringService_GetService(t *testing.T) {
    cfg := newTestConfig()
    service := NewMonitoringService(&MockNotificationService{}, cfg)

    req := &grpcPb.GetServiceRequest{Id: 1}

    resp, err := service.GetService(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, "Example Service", resp.Name)
}

func TestMonitoringService_ListServices(t *testing.T) {
    cfg := newTestConfig()
    service := NewMonitoringService(&MockNotificationService{}, cfg)

    req := &emptypb.Empty{}

    resp, err := service.ListServices(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, int32(1), resp.Count)
    assert.Len(t, resp.Services, 1)
    assert.Equal(t, "Service 1", resp.Services[0].Name)
}

func TestMonitoringService_CreateResult_Success(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            return nil
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.Result{
        Id:     1,
        Name:   "Test Result",
        Tenant: "Test Tenant",
    }

    resp, err := service.CreateResult(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, "Result created successfully (stub)", resp.Message)
    assert.Equal(t, req, resp.Data)
}

func TestMonitoringService_CreateResult_ProcessEntityError(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessEntityFunc: func(ctx context.Context, entity interface{}) error {
            return assert.AnError // имитируем ошибку сохранения
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.Result{
        Id:     1,
        Name:   "Test Result",
        Tenant: "Test Tenant",
    }

    resp, err := service.CreateResult(context.Background(), req)
    assert.NoError(t, err) // метод всё равно возвращает успех (stub)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, "Result created successfully (stub)", resp.Message)
}
func TestMonitoringService_GetResult(t *testing.T) {
    cfg := newTestConfig()
    service := NewMonitoringService(&MockNotificationService{}, cfg)

    req := &grpcPb.GetResultRequest{Id: 1}

    resp, err := service.GetResult(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, "Example Result", resp.Name)
}

func TestMonitoringService_ListResults(t *testing.T) {
    cfg := newTestConfig()
    service := NewMonitoringService(&MockNotificationService{}, cfg)

    req := &emptypb.Empty{}

    resp, err := service.ListResults(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, int32(1), resp.Count)
    assert.Len(t, resp.Results, 1)
    assert.Equal(t, "Result 1", resp.Results[0].Name)
}

func TestMonitoringService_SendNotification_Success(t *testing.T) {
    mockService := &MockNotificationService{
        SendNotificationFunc: func(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
            return &domain.SentNotification{
                MessageID: 123,
                ChatID:    123, // ✅ int64, а не chatID string
                SentAt:    time.Now(),
            }, nil
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.SendRequest{
        ChatId: "123",
        Text:   "Test message",
    }

    resp, err := service.SendNotification(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, "Notification sent successfully", resp.Message)
    assert.Equal(t, "123", resp.ChatId)
    assert.Equal(t, int64(123), resp.MessageId)
}

func TestMonitoringService_SendNotification_Error(t *testing.T) {
    mockService := &MockNotificationService{
        SendNotificationFunc: func(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
            return nil, assert.AnError
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.SendRequest{
        ChatId: "123",
        Text:   "Test message",
    }

    _, err := service.SendNotification(context.Background(), req)
    assert.Error(t, err)
    assert.Equal(t, codes.Internal, status.Code(err))
    assert.Contains(t, err.Error(), "failed to send notification")
}

func TestMonitoringService_BatchSend_Success(t *testing.T) {
    mockService := &MockNotificationService{
        ProcessWithIntervalsFunc: func(ctx context.Context, notifications []*domain.Notification, interval time.Duration, workers int) domain.ProcessResult {
            return domain.ProcessResult{
                SuccessCount: len(notifications),
                ErrorCount:   0,
            }
        },
    }
    cfg := newTestConfig()
    service := NewMonitoringService(mockService, cfg)

    req := &grpcPb.BatchSendRequest{
        Messages: []*grpcPb.SendRequest{
            {ChatId: "123", Text: "Msg1"},
            {ChatId: "456", Text: "Msg2"},
        },
        IntervalMs: 1000,
        Workers:    3,
    }

    resp, err := service.BatchSend(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Success)
    assert.Equal(t, int32(2), resp.Total)
    assert.Equal(t, int32(2), resp.SuccessCount)
    assert.Equal(t, int32(0), resp.ErrorCount)
}

func TestGRPCServer_GetServer(t *testing.T) {
    cfg := newTestConfig()
    mockService := &MockNotificationService{}
    
    server, err := NewGRPCServer(cfg, mockService)
    assert.NoError(t, err)
    assert.NotNil(t, server)
    
    // Проверяем, что GetServer возвращает *grpc.Server
    grpcServer := server.GetServer()
    assert.NotNil(t, grpcServer)
}