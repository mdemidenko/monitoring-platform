package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/notifier"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
	"github.com/mdemidenko/monitoring-platform/pkg/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MonitoringService реализует gRPC сервис мониторинга
type MonitoringService struct {
	grpc.UnimplementedMonitoringServiceServer
	telegramService *notifier.TelegramService
	storage         *repository.MemoryStorage
	cfg             *config.Config
}

// NewMonitoringService создает новый экземпляр сервиса
func NewMonitoringService(
	telegramService *notifier.TelegramService,
	storage *repository.MemoryStorage,
	cfg *config.Config,
) *MonitoringService {
	return &MonitoringService{
		telegramService: telegramService,
		storage:         storage,
		cfg:             cfg,
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

// CreateNotification создает новое уведомление
func (s *MonitoringService) CreateNotification(ctx context.Context, req *grpc.Notification) (*grpc.NotificationResponse, error) {
	// Конвертируем из protobuf в модель
	notification := &models.Notification{
		ChatID: req.ChatId,
		Text:   req.Text,
	}
	
	// Сохраняем в хранилище
	if err := s.storage.Store(notification); err != nil {
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
	notifications := s.storage.GetNotifications()
	
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
	notifications := s.storage.GetNotifications()
	
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
	// Конвертируем из protobuf в модель
	sentNotification := &models.SentNotification{
		MessageID: req.MessageId,
		ChatID:    req.ChatId,
	}
	
	// Сохраняем в хранилище
	if err := s.storage.Store(sentNotification); err != nil {
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
	sentNotifications := s.storage.GetSentNotifications()
	
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
	sentNotifications := s.storage.GetSentNotifications()
	
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
	// Конвертируем из protobuf в модель
	service := &models.Service{
		ID:             int(req.Id),
		Name:           req.Name,
		Tenant:         req.Tenant,
		DeprecatedDate: req.DeprecatedDate,
		BusinessLine:   req.BusinessLine,
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
	// Конвертируем из protobuf в модель
	result := &models.Result{
		ID:     int(req.Id),
		Name:   req.Name,
		Tenant: req.Tenant,
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
	
	// Создаем уведомление
	notification := models.NewNotification(chatID, req.Text)
	
	// Сохраняем в хранилище
	if err := s.storage.Store(notification); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to store notification: %v", err))
	}
	
	// Отправляем через сервис Telegram
	sentNotification, err := s.telegramService.SendNotification(ctx, req.Text)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send notification: %v", err))
	}
	
	// Сохраняем отправленное уведомление
	if sentNotification != nil {
		if err := s.storage.Store(sentNotification); err != nil {
			log.Printf("Failed to store sent notification: %v", err)
		}
	}
	
	// Формируем ответ
	response := &grpc.SendResponse{
		Success:   true,
		Message:   "Notification sent successfully",
		ChatId:    chatID,
	}
	
	if sentNotification != nil {
		response.MessageId = sentNotification.MessageID
	}
	
	return response, nil
}

// BatchSend отправляет несколько уведомлений пакетно
func (s *MonitoringService) BatchSend(ctx context.Context, req *grpc.BatchSendRequest) (*grpc.BatchSendResponse, error) {
	// Преобразуем запрос в формат для обработки
	notifications := make([]*models.Notification, 0, len(req.Messages))
	
	for _, msg := range req.Messages {
		chatID := msg.ChatId
		if chatID == "" {
			chatID = s.cfg.Telegram.ChatID
		}
		notifications = append(notifications, models.NewNotification(chatID, msg.Text))
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
	
	// Запускаем обработку
	result := s.telegramService.ProcessWithIntervals(ctx, notifications, interval, workers)
	
	return &grpc.BatchSendResponse{
		Success:      true,
		Message:      "Batch processing completed",
		Total:        int32(len(notifications)),
		SuccessCount: int32(result.SuccessCount),
		ErrorCount:   int32(result.ErrorCount),
	}, nil
}