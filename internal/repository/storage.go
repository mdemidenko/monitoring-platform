package repository

import (
	"fmt"
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
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		notifications:     make([]*models.Notification, 0),
		sentNotifications: make([]*models.SentNotification, 0),
	}
}

func (m *MemoryStorage) Store(entity any) error {
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
	return m.notifications
}

func (m *MemoryStorage) GetSentNotifications() []*models.SentNotification {
	return m.sentNotifications
}