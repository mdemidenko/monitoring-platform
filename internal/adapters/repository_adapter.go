package adapters

import (
    "fmt"
    "log"

    "github.com/mdemidenko/monitoring-platform/internal/domain"
    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
)

// RepositoryAdapter адаптирует MemoryStorage к domain.NotificationRepository
type RepositoryAdapter struct {
    storage *repository.MemoryStorage
    logger  *log.Logger
}

// NewRepositoryAdapter создает новый адаптер для хранилища
func NewRepositoryAdapter(storage *repository.MemoryStorage, logger *log.Logger) *RepositoryAdapter {
    if logger == nil {
        logger = log.Default()
    }
    
    return &RepositoryAdapter{
        storage: storage,
        logger:  logger,
    }
}

// Store сохраняет сущность в хранилище
func (a *RepositoryAdapter) Store(entity interface{}) error {
    // Преобразуем доменную сущность в модель хранилища
    switch v := entity.(type) {
    case *domain.Notification:
        // Конвертируем domain.Notification в models.Notification
        model := &models.Notification{
            ChatID: v.ChatID,
            Text:   v.Text,
        }
        return a.storage.Store(model)
        
    case *domain.SentNotification:
        // Конвертируем domain.SentNotification в models.SentNotification
        model := &models.SentNotification{
            MessageID: v.MessageID,
            ChatID:    v.ChatID,
        }
        return a.storage.Store(model)
        
    default:
        return fmt.Errorf("unsupported entity type: %T", v)
    }
}

// GetNotifications возвращает все уведомления
func (a *RepositoryAdapter) GetNotifications() []*domain.Notification {
    // Получаем модели из хранилища
    modelNotifications := a.storage.GetNotifications()
    
    // Конвертируем в доменные сущности
    domainNotifications := make([]*domain.Notification, len(modelNotifications))
    for i, n := range modelNotifications {
        domainNotifications[i] = &domain.Notification{
            ChatID: n.ChatID,
            Text:   n.Text,
        }
    }
    
    return domainNotifications
}

// GetSentNotifications возвращает отправленные уведомления
func (a *RepositoryAdapter) GetSentNotifications() []*domain.SentNotification {
    // Получаем модели из хранилища
    modelSentNotifications := a.storage.GetSentNotifications()
    
    // Конвертируем в доменные сущности
    domainSentNotifications := make([]*domain.SentNotification, len(modelSentNotifications))
    for i, sn := range modelSentNotifications {
        domainSentNotifications[i] = &domain.SentNotification{
            MessageID: sn.MessageID,
            ChatID:    sn.ChatID,
        }
    }
    
    return domainSentNotifications
}

// MemoryStorageAdapter полная реализация адаптера с поддержкой domain.NotificationRepository
type MemoryStorageAdapter struct {
    *RepositoryAdapter
}

// NewMemoryStorageAdapter создает адаптер для MemoryStorage
func NewMemoryStorageAdapter(storage *repository.MemoryStorage, logger *log.Logger) domain.NotificationRepository {
    return &MemoryStorageAdapter{
        RepositoryAdapter: NewRepositoryAdapter(storage, logger),
    }
}