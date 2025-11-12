package repository

import (
	"fmt"
	"monitoring-platform/internal/models"
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
		return nil
	case *models.SentNotification:
		m.sentNotifications = append(m.sentNotifications, v)
		return nil
	default:
		return fmt.Errorf("unsupported entity type: %T", v)
	}
}

func (m *MemoryStorage) GetNotifications() []*models.Notification {
	return m.notifications
}

func (m *MemoryStorage) GetSentNotifications() []*models.SentNotification {
	return m.sentNotifications
}