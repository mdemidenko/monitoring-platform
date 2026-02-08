package repository

import (
	"fmt"
	"sync"

	"github.com/mdemidenko/monitoring-platform/internal/models"
)

type Storage interface {
	Store(entity any) error
	GetNotifications() []*models.Notification
	GetSentNotifications() []*models.SentNotification
}

type MemoryStorage struct {
	notifications     []*models.Notification
	sentNotifications []*models.SentNotification
	mu                sync.RWMutex // Мьютекс для потокобезопасности
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		notifications:     make([]*models.Notification, 0),
		sentNotifications: make([]*models.SentNotification, 0),
	}
}

func (m *MemoryStorage) Store(entity any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch v := entity.(type) {
	case *models.Notification:
		m.notifications = append(m.notifications, v)
	case *models.SentNotification:
		m.sentNotifications = append(m.sentNotifications, v)
	default:
		return fmt.Errorf("unsupported entity type: %T", v)
	}

	return nil
}

func (m *MemoryStorage) GetNotifications() []*models.Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Возвращаем копию для безопасности
	result := make([]*models.Notification, len(m.notifications))
	copy(result, m.notifications)
	return result
}

func (m *MemoryStorage) GetSentNotifications() []*models.SentNotification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Возвращаем копию для безопасности
	result := make([]*models.SentNotification, len(m.sentNotifications))
	copy(result, m.sentNotifications)
	return result
}