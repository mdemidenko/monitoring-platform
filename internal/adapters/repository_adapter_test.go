package adapters

import (
	"strings"
	"testing"

	"github.com/mdemidenko/monitoring-platform/internal/domain"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

func TestRepositoryAdapter_Store_Integration(t *testing.T) {
	tests := []struct {
		name        string
		entity      interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:    "store domain notification",
			entity:  &domain.Notification{ChatID: "123", Text: "Test"},
			wantErr: false,
		},
		{
			name:    "store domain sent notification",
			entity:  &domain.SentNotification{MessageID: 1, ChatID: 123},
			wantErr: false,
		},
		{
			name:        "store unsupported entity type",
			entity:      "invalid entity",
			wantErr:     true,
			errContains: "unsupported entity type",
		},
		{
			name:        "store nil entity",
			entity:      nil,
			wantErr:     true,
			errContains: "unsupported entity type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Используем реальное хранилище
			storage := repository.NewMemoryStorage()
			adapter := NewRepositoryAdapter(storage, nil)

			err := adapter.Store(tt.entity)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRepositoryAdapter_StoreAndRetrieve_Integration(t *testing.T) {
	// Комплексный тест: сохраняем и получаем данные
	storage := repository.NewMemoryStorage()
	adapter := NewRepositoryAdapter(storage, nil)

	// Сохраняем несколько уведомлений
	notifications := []*domain.Notification{
		{ChatID: "chat1", Text: "Message 1"},
		{ChatID: "chat2", Text: "Message 2"},
		{ChatID: "chat3", Text: "Message 3"},
	}

	for _, n := range notifications {
		err := adapter.Store(n)
		if err != nil {
			t.Fatalf("Failed to store notification: %v", err)
		}
	}

	// Сохраняем отправленные уведомления
	sentNotifications := []*domain.SentNotification{
		{MessageID: 1, ChatID: 1001},
		{MessageID: 2, ChatID: 1002},
	}

	for _, sn := range sentNotifications {
		err := adapter.Store(sn)
		if err != nil {
			t.Fatalf("Failed to store sent notification: %v", err)
		}
	}

	// Получаем и проверяем уведомления
	retrievedNotifications := adapter.GetNotifications()
	if len(retrievedNotifications) != len(notifications) {
		t.Errorf("Expected %d notifications, got %d", len(notifications), len(retrievedNotifications))
	}

	// Проверяем каждое уведомление
	for i, expected := range notifications {
		actual := retrievedNotifications[i]
		if actual.ChatID != expected.ChatID || actual.Text != expected.Text {
			t.Errorf("Notification %d mismatch: expected %+v, got %+v", i, expected, actual)
		}
	}

	// Получаем и проверяем отправленные уведомления
	retrievedSentNotifications := adapter.GetSentNotifications()
	if len(retrievedSentNotifications) != len(sentNotifications) {
		t.Errorf("Expected %d sent notifications, got %d", len(sentNotifications), len(retrievedSentNotifications))
	}

	for i, expected := range sentNotifications {
		actual := retrievedSentNotifications[i]
		if actual.MessageID != expected.MessageID || actual.ChatID != expected.ChatID {
			t.Errorf("SentNotification %d mismatch: expected %+v, got %+v", i, expected, actual)
		}
	}
}

func TestRepositoryAdapter_GetNotifications_Empty(t *testing.T) {
	storage := repository.NewMemoryStorage()
	adapter := NewRepositoryAdapter(storage, nil)

	notifications := adapter.GetNotifications()
	if notifications == nil {
		t.Error("GetNotifications should not return nil")
	}
	if len(notifications) != 0 {
		t.Errorf("Expected 0 notifications, got %d", len(notifications))
	}
}

func TestRepositoryAdapter_GetSentNotifications_Empty(t *testing.T) {
	storage := repository.NewMemoryStorage()
	adapter := NewRepositoryAdapter(storage, nil)

	sentNotifications := adapter.GetSentNotifications()
	if sentNotifications == nil {
		t.Error("GetSentNotifications should not return nil")
	}
	if len(sentNotifications) != 0 {
		t.Errorf("Expected 0 sent notifications, got %d", len(sentNotifications))
	}
}

func TestMemoryStorageAdapter_Interface(t *testing.T) {
	storage := repository.NewMemoryStorage()
	adapter := NewMemoryStorageAdapter(storage, nil)

	// Проверяем что адаптер реализует интерфейс domain.NotificationRepository
	var _ domain.NotificationRepository = adapter

	// Базовый тест работы
	notification := &domain.Notification{
		ChatID: "test-chat",
		Text:   "Test message",
	}

	err := adapter.Store(notification)
	if err != nil {
		t.Errorf("Store failed: %v", err)
	}

	notifications := adapter.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}
}