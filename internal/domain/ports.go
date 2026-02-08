package domain

import (
    "context"
    "time"
)

// NotificationSender - порт для отправки уведомлений
type NotificationSender interface {
    // Send отправляет уведомление
    Send(ctx context.Context, notification *Notification) (*SentNotification, error)
    
    // HealthCheck проверяет доступность сервиса отправки
    HealthCheck() error
}

// NotificationRepository - порт для работы с хранилищем
type NotificationRepository interface {
    // Store сохраняет сущность
    Store(entity interface{}) error
    
    // GetNotifications возвращает все уведомления
    GetNotifications() []*Notification
    
    // GetSentNotifications возвращает отправленные уведомления
    GetSentNotifications() []*SentNotification
}

// NotificationService - порт для бизнес-логики
type NotificationService interface {
    // SendNotification отправляет одно уведомление
    SendNotification(ctx context.Context, chatID, text string) (*SentNotification, error)
    
    // ProcessWithIntervals обрабатывает уведомления с интервалами
    ProcessWithIntervals(ctx context.Context, notifications []*Notification, interval time.Duration, workers int) ProcessResult
    
    // ProcessEntity обрабатывает сущность (бизнес-логика)
    ProcessEntity(ctx context.Context, entity interface{}) error
    
    // HealthCheck проверяет здоровье системы
    HealthCheck() error
    
    // GetNotifications возвращает все уведомления
    GetNotifications() []*Notification
    
    // GetSentNotifications возвращает отправленные уведомления
    GetSentNotifications() []*SentNotification
    
    // GetStats возвращает статистику сервиса
    GetStats() *ServiceStats
}