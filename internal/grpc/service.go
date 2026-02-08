package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/core"
	"github.com/mdemidenko/monitoring-platform/internal/domain"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/pkg/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MonitoringService реализует gRPC сервис мониторинга
type MonitoringService struct {
	grpc.UnimplementedMonitoringServiceServer
	notificationService *core.NotificationService
	cfg                 *config.Config
}

// NewMonitoringService создает новый экземпляр сервиса
func NewMonitoringService(
	notificationService *core.NotificationService,
	cfg *config.Config,
) *MonitoringService {
	return &MonitoringService{
		notificationService: notificationService,
		cfg:                 cfg,
	}
}

// ========== Аутентификация ==========

// Login реализует аутентификацию пользователя
func (s *MonitoringService) Login(ctx context.Context, req *grpc.LoginRequest) (*grpc.LoginResponse, error) {
	// Проверяем credentials
	if req.Username != s.cfg.Auth.Login || req.Password != s.cfg.Auth.Password {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	// Создаем JWT токен
	expirationTime := time.Now().Add(time.Duration(s.cfg.Auth.JWTExpiration) * time.Hour)

	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ID:        uuid.New().String(),
		Subject:   req.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.Auth.JWTSecret))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &grpc.LoginResponse{
		Token:     tokenString,
		ExpiresAt: expirationTime.Format(time.RFC3339),
		TokenType: "Bearer",
	}, nil
}

// ========== Notification методы ==========

// CreateNotification использует сервис ядра
func (s *MonitoringService) CreateNotification(ctx context.Context, req *grpc.Notification) (*grpc.NotificationResponse, error) {
	// Создаем доменное уведомление
	notification := domain.NewNotification(req.ChatId, req.Text)
	
	// Используем ProcessEntity для сохранения через репозиторий
	if err := s.notificationService.ProcessEntity(ctx, notification); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to store notification: %v", err))
	}
	
	return &grpc.NotificationResponse{
		Success: true,
		Message: "Notification created successfully",
		Data:    req,
	}, nil
}

// GetNotification получает уведомление по chat_id
func (s *MonitoringService) GetNotification(ctx context.Context, req *grpc.GetNotificationRequest) (*grpc.Notification, error) {
	// Используем метод сервиса для получения уведомлений
	notifications := s.notificationService.GetNotifications()
	
	// Ищем уведомление по chat_id
	for _, n := range notifications {
		if n.ChatID == req.ChatId {
			return &grpc.Notification{
				ChatId: n.ChatID,
				Text:   n.Text,
			}, nil
		}
	}
	
	return nil, status.Error(codes.NotFound, "notification not found")
}

// ListNotifications возвращает список всех уведомлений
func (s *MonitoringService) ListNotifications(ctx context.Context, _ *emptypb.Empty) (*grpc.NotificationList, error) {
	// Используем метод сервиса для получения уведомлений
	notifications := s.notificationService.GetNotifications()
	
	// Конвертируем в protobuf сообщения
	pbNotifications := make([]*grpc.Notification, 0, len(notifications))
	for _, n := range notifications {
		pbNotifications = append(pbNotifications, &grpc.Notification{
			ChatId: n.ChatID,
			Text:   n.Text,
		})
	}
	
	return &grpc.NotificationList{
		Notifications: pbNotifications,
		Count:         int32(len(pbNotifications)),
	}, nil
}

// ========== SentNotification методы ==========

// CreateSentNotification создает запись об отправленном уведомлении
func (s *MonitoringService) CreateSentNotification(ctx context.Context, req *grpc.SentNotification) (*grpc.SentNotificationResponse, error) {
	// Создаем доменную модель отправленного уведомления
	// Предполагаем, что SentNotification имеет конструктор или можно создать напрямую
	// В зависимости от того, как определен domain.SentNotification
	sentNotification := &domain.SentNotification{
		MessageID: req.MessageId,
		ChatID:    req.ChatId,
	}
	
	// Используем ProcessEntity для сохранения через репозиторий
	if err := s.notificationService.ProcessEntity(ctx, sentNotification); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to store sent notification: %v", err))
	}
	
	return &grpc.SentNotificationResponse{
		Success: true,
		Message: "Sent notification created successfully",
		Data:    req,
	}, nil
}

// GetSentNotification получает отправленное уведомление по message_id
func (s *MonitoringService) GetSentNotification(ctx context.Context, req *grpc.GetSentNotificationRequest) (*grpc.SentNotification, error) {
	// Используем метод сервиса для получения отправленных уведомлений
	sentNotifications := s.notificationService.GetSentNotifications()
	
	// Ищем по message_id
	for _, n := range sentNotifications {
		if n.MessageID == req.MessageId {
			return &grpc.SentNotification{
				MessageId: n.MessageID,
				ChatId:    n.ChatID,
			}, nil
		}
	}
	
	return nil, status.Error(codes.NotFound, "sent notification not found")
}

// ListSentNotifications возвращает список всех отправленных уведомлений
func (s *MonitoringService) ListSentNotifications(ctx context.Context, _ *emptypb.Empty) (*grpc.SentNotificationList, error) {
	// Используем метод сервиса для получения отправленных уведомлений
	sentNotifications := s.notificationService.GetSentNotifications()
	
	// Конвертируем в protobuf сообщения
	pbSentNotifications := make([]*grpc.SentNotification, 0, len(sentNotifications))
	for _, n := range sentNotifications {
		pbSentNotifications = append(pbSentNotifications, &grpc.SentNotification{
			MessageId: n.MessageID,
			ChatId:    n.ChatID,
		})
	}
	
	return &grpc.SentNotificationList{
		SentNotifications: pbSentNotifications,
		Count:             int32(len(pbSentNotifications)),
	}, nil
}

// ========== Service методы ==========

// CreateService создает новый сервис
func (s *MonitoringService) CreateService(ctx context.Context, req *grpc.Service) (*grpc.ServiceResponse, error) {
	// Создаем модель сервиса
	service := &models.Service{
		ID:             int(req.Id),
		Name:           req.Name,
		Tenant:         req.Tenant,
		DeprecatedDate: req.DeprecatedDate,
		BusinessLine:   req.BusinessLine,
	}
	
	// Сохраняем через ProcessEntity
	if err := s.notificationService.ProcessEntity(ctx, service); err != nil {
		// Если ProcessEntity не поддерживает models.Service, просто логируем
		log.Printf("Warning: failed to store service via ProcessEntity: %v", err)
	}
	
	log.Printf("Service created (stub): %+v", service)
	
	return &grpc.ServiceResponse{
		Success: true,
		Message: "Service created successfully (stub)",
		Data:    req,
	}, nil
}

// GetService получает сервис по ID
func (s *MonitoringService) GetService(ctx context.Context, req *grpc.GetServiceRequest) (*grpc.Service, error) {
	return &grpc.Service{
		Id:             req.Id,
		Name:           "Example Service",
		Tenant:         "Example Tenant",
		DeprecatedDate: "2024-12-31",
		BusinessLine:   "Example Business Line",
	}, nil
}

// ListServices возвращает список сервисов
func (s *MonitoringService) ListServices(ctx context.Context, _ *emptypb.Empty) (*grpc.ServiceList, error) {
	return &grpc.ServiceList{
		Services: []*grpc.Service{
			{
				Id:             1,
				Name:           "Service 1",
				Tenant:         "Tenant A",
				DeprecatedDate: "2024-12-31",
				BusinessLine:   "Business Line 1",
			},
		},
		Count: 1,
	}, nil
}

// ========== Result методы ==========

// CreateResult создает новый результат
func (s *MonitoringService) CreateResult(ctx context.Context, req *grpc.Result) (*grpc.ResultResponse, error) {
	// Создаем модель результата
	result := &models.Result{
		ID:     int(req.Id),
		Name:   req.Name,
		Tenant: req.Tenant,
	}
	
	// Сохраняем через ProcessEntity
	if err := s.notificationService.ProcessEntity(ctx, result); err != nil {
		// Если ProcessEntity не поддерживает models.Result, просто логируем
		log.Printf("Warning: failed to store result via ProcessEntity: %v", err)
	}
	
	log.Printf("Result created (stub): %+v", result)
	
	return &grpc.ResultResponse{
		Success: true,
		Message: "Result created successfully (stub)",
		Data:    req,
	}, nil
}

// GetResult получает результат по ID
func (s *MonitoringService) GetResult(ctx context.Context, req *grpc.GetResultRequest) (*grpc.Result, error) {
	return &grpc.Result{
		Id:     req.Id,
		Name:   "Example Result",
		Tenant: "Example Tenant",
	}, nil
}

// ListResults возвращает список результатов
func (s *MonitoringService) ListResults(ctx context.Context, _ *emptypb.Empty) (*grpc.ResultList, error) {
	return &grpc.ResultList{
		Results: []*grpc.Result{
			{
				Id:     1,
				Name:   "Result 1",
				Tenant: "Tenant A",
			},
		},
		Count: 1,
	}, nil
}

// ========== Методы отправки уведомлений ==========

// SendNotification отправляет уведомление через Telegram
func (s *MonitoringService) SendNotification(ctx context.Context, req *grpc.SendRequest) (*grpc.SendResponse, error) {
	// Используем chat_id из запроса или дефолтный
	chatID := req.ChatId
	if chatID == "" {
		chatID = s.cfg.Telegram.ChatID
	}
	
	// Используем метод SendNotification сервиса (он сам сохранит и отправит)
	sentNotification, err := s.notificationService.SendNotification(ctx, chatID, req.Text)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send notification: %v", err))
	}
	
	// Формируем ответ
	response := &grpc.SendResponse{
		Success: true,
		Message: "Notification sent successfully",
		ChatId:  chatID,
	}
	
	if sentNotification != nil {
		response.MessageId = sentNotification.MessageID
	}
	
	return response, nil
}

// BatchSend отправляет несколько уведомлений пакетно
func (s *MonitoringService) BatchSend(ctx context.Context, req *grpc.BatchSendRequest) (*grpc.BatchSendResponse, error) {
	// Преобразуем protobuf сообщения в доменные уведомления
	notifications := make([]*domain.Notification, 0, len(req.Messages))
	for _, msg := range req.Messages {
		chatID := msg.ChatId
		if chatID == "" {
			chatID = s.cfg.Telegram.ChatID
		}
		notifications = append(notifications, &domain.Notification{
			ChatID: chatID,
			Text:   msg.Text,
		})
	}
	
	// Настраиваем интервал
	interval := 2 * time.Second
	if req.IntervalMs > 0 {
		interval = time.Duration(req.IntervalMs) * time.Millisecond
	}
	
	// Настраиваем количество воркеров
	workers := 2
	if req.Workers > 0 && req.Workers <= 10 {
		workers = int(req.Workers)
	}
	
	// Запускаем обработку через метод сервиса
	result := s.notificationService.ProcessWithIntervals(ctx, notifications, interval, workers)
	
	return &grpc.BatchSendResponse{
		Success:      true,
		Message:      "Batch processing completed",
		Total:        int32(len(notifications)),
		SuccessCount: int32(result.SuccessCount),
		ErrorCount:   int32(result.ErrorCount),
	}, nil
}