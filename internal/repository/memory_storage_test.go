package repository

import (
	"sync"
	"testing"

	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()

	if storage == nil {
		t.Fatal("Expected MemoryStorage instance, got nil")
	}

	// Проверяем через публичные методы
	notifications := storage.GetNotifications()
	if notifications == nil {
		t.Error("GetNotifications should not return nil")
	}
	if len(notifications) != 0 {
		t.Errorf("Expected empty notifications slice, got %d items", len(notifications))
	}

	sentNotifications := storage.GetSentNotifications()
	if sentNotifications == nil {
		t.Error("GetSentNotifications should not return nil")
	}
	if len(sentNotifications) != 0 {
		t.Errorf("Expected empty sentNotifications slice, got %d items", len(sentNotifications))
	}
}

func TestMemoryStorage_Store_Notification(t *testing.T) {
	tests := []struct {
		name         string
		notifications []*models.Notification
		wantCount    int
	}{
		{
			name: "store single notification",
			notifications: []*models.Notification{
				{ChatID: "123", Text: "Test 1"},
			},
			wantCount: 1,
		},
		{
			name: "store multiple notifications",
			notifications: []*models.Notification{
				{ChatID: "1", Text: "Test 1"},
				{ChatID: "2", Text: "Test 2"},
				{ChatID: "3", Text: "Test 3"},
			},
			wantCount: 3,
		},
		{
			name:         "store empty notifications list",
			notifications: []*models.Notification{},
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMemoryStorage()

			for _, n := range tt.notifications {
				err := storage.Store(n)
				if err != nil {
					t.Errorf("Failed to store notification: %v", err)
				}
			}

			notifications := storage.GetNotifications()
			if len(notifications) != tt.wantCount {
				t.Errorf("Expected %d notifications, got %d", tt.wantCount, len(notifications))
			}

			// Проверяем что порядок сохранения сохранился
			for i, n := range notifications {
				expected := tt.notifications[i]
				if n.ChatID != expected.ChatID || n.Text != expected.Text {
					t.Errorf("Notification %d mismatch: expected %+v, got %+v", i, expected, n)
				}
			}
		})
	}
}

func TestMemoryStorage_Store_SentNotification(t *testing.T) {
	tests := []struct {
		name             string
		sentNotifications []*models.SentNotification
		wantCount       int
	}{
		{
			name: "store single sent notification",
			sentNotifications: []*models.SentNotification{
				{MessageID: 1, ChatID: 123},
			},
			wantCount: 1,
		},
		{
			name: "store multiple sent notifications",
			sentNotifications: []*models.SentNotification{
				{MessageID: 1, ChatID: 123},
				{MessageID: 2, ChatID: 456},
				{MessageID: 3, ChatID: 789},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMemoryStorage()

			for _, sn := range tt.sentNotifications {
				err := storage.Store(sn)
				if err != nil {
					t.Errorf("Failed to store sent notification: %v", err)
				}
			}

			sentNotifications := storage.GetSentNotifications()
			if len(sentNotifications) != tt.wantCount {
				t.Errorf("Expected %d sent notifications, got %d", tt.wantCount, len(sentNotifications))
			}

			for i, sn := range sentNotifications {
				expected := tt.sentNotifications[i]
				if sn.MessageID != expected.MessageID || sn.ChatID != expected.ChatID {
					t.Errorf("SentNotification %d mismatch: expected %+v, got %+v", i, expected, sn)
				}
			}
		})
	}
}

func TestMemoryStorage_Store_MixedTypes(t *testing.T) {
	storage := NewMemoryStorage()

	// Сохраняем оба типа
	notification := &models.Notification{ChatID: "123", Text: "Hello"}
	sentNotification := &models.SentNotification{MessageID: 1, ChatID: 456}

	err := storage.Store(notification)
	if err != nil {
		t.Errorf("Failed to store notification: %v", err)
	}

	err = storage.Store(sentNotification)
	if err != nil {
		t.Errorf("Failed to store sent notification: %v", err)
	}

	// Проверяем раздельное хранение
	notifications := storage.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}

	sentNotifications := storage.GetSentNotifications()
	if len(sentNotifications) != 1 {
		t.Errorf("Expected 1 sent notification, got %d", len(sentNotifications))
	}

	// Проверяем что типы не смешались
	if notifications[0].ChatID != "123" || notifications[0].Text != "Hello" {
		t.Errorf("Notification corrupted: %+v", notifications[0])
	}

	if sentNotifications[0].MessageID != 1 || sentNotifications[0].ChatID != 456 {
		t.Errorf("SentNotification corrupted: %+v", sentNotifications[0])
	}
}

func TestMemoryStorage_Store_UnsupportedType(t *testing.T) {
	storage := NewMemoryStorage()

	// Пытаемся сохранить неподдерживаемый тип
	unsupported := "this is not a valid entity"

	err := storage.Store(unsupported)

	if err == nil {
		t.Error("Expected error for unsupported entity type, got nil")
	}

	expectedError := "unsupported entity type: string"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%v'", expectedError, err)
	}

	// Проверяем что хранилище осталось пустым
	if len(storage.GetNotifications()) != 0 {
		t.Error("Storage should be empty after unsupported store attempt")
	}

	if len(storage.GetSentNotifications()) != 0 {
		t.Error("Storage should be empty after unsupported store attempt")
	}
}

func TestMemoryStorage_Store_NilEntity(t *testing.T) {
	storage := NewMemoryStorage()

	// Пытаемся сохранить nil
	err := storage.Store(nil)

	if err == nil {
		t.Error("Expected error for nil entity, got nil")
	}

	expectedError := "unsupported entity type: <nil>"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%v'", expectedError, err)
	}
}

func TestMemoryStorage_Store_DuplicateReferences(t *testing.T) {
	storage := NewMemoryStorage()

	// Создаем одну сущность
	notification := &models.Notification{ChatID: "123", Text: "Original"}

	// Сохраняем её дважды
	err := storage.Store(notification)
	if err != nil {
		t.Errorf("First store failed: %v", err)
	}

	err = storage.Store(notification)
	if err != nil {
		t.Errorf("Second store failed: %v", err)
	}

	// Должно быть 2 ссылки на один объект
	notifications := storage.GetNotifications()
	if len(notifications) != 2 {
		t.Errorf("Expected 2 notifications, got %d", len(notifications))
	}

	// Обе ссылки указывают на один объект
	if notifications[0] != notifications[1] {
		t.Error("Expected both entries to reference the same object")
	}

	// Изменяем оригинал
	notification.Text = "Modified"

	// Обе ссылки должны отражать изменения
	if notifications[0].Text != "Modified" || notifications[1].Text != "Modified" {
		t.Error("Both stored references should reflect changes to original")
	}
}

func TestMemoryStorage_GetNotifications_ReturnsCopy(t *testing.T) {
	storage := NewMemoryStorage()

	notification := &models.Notification{ChatID: "123", Text: "Test"}
	require.NoError(t, storage.Store(notification), "Storage should not fail on Store")

	// Получаем список
	notifications := storage.GetNotifications()

	// Модифицируем возвращенный слайс (не должен влиять на хранилище)
	notifications = append(notifications, &models.Notification{ChatID: "456", Text: "Extra"})

	// Проверяем что хранилище не изменилось
	storageNotifications := storage.GetNotifications()
	if len(storageNotifications) != 1 {
		t.Errorf("Storage should still have 1 notification, got %d", len(storageNotifications))
	}
}

func TestMemoryStorage_GetSentNotifications_ReturnsCopy(t *testing.T) {
	storage := NewMemoryStorage()

	sentNotification := &models.SentNotification{MessageID: 1, ChatID: 123}
	require.NoError(t, storage.Store(sentNotification), "Storage should not fail on Store")

	// Получаем список
	sentNotifications := storage.GetSentNotifications()

	// Модифицируем возвращенный слайс
	sentNotifications = append(sentNotifications, &models.SentNotification{MessageID: 2, ChatID: 456})

	// Проверяем что хранилище не изменилось
	storageSentNotifications := storage.GetSentNotifications()
	if len(storageSentNotifications) != 1 {
		t.Errorf("Storage should still have 1 sent notification, got %d", len(storageSentNotifications))
	}
}

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()
	const numGoroutines = 100
	const numItemsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numItemsPerGoroutine; j++ {
				notification := &models.Notification{
					ChatID: string(rune(id + 65)), // A, B, C...
					Text:   string(rune(j + 97)),  // a, b, c...
				}
				err := storage.Store(notification)
				if err != nil {
					t.Errorf("Goroutine %d failed to store: %v", id, err)
				}
			}
		}(i)
	}

	// Ждем завершения всех горутин
	wg.Wait()

	// Проверяем что все элементы сохранены
	totalNotifications := len(storage.GetNotifications())
	expectedTotal := numGoroutines * numItemsPerGoroutine
	
	if totalNotifications != expectedTotal {
		t.Errorf("Expected %d notifications, got %d", expectedTotal, totalNotifications)
	}

	// Дополнительная проверка: убедимся что нет дубликатов (по ChatID + Text)
	// Это не идеальная проверка, но помогает убедиться в целостности
	notifications := storage.GetNotifications()
	seen := make(map[string]bool)
	for _, n := range notifications {
		key := n.ChatID + ":" + n.Text
		if seen[key] {
			t.Errorf("Duplicate notification found: %s", key)
		}
		seen[key] = true
	}

	// Проверяем конкурентный доступ к GetNotifications
	var wg2 sync.WaitGroup
	wg2.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg2.Done()
			notifications := storage.GetNotifications()
			if len(notifications) != expectedTotal {
				t.Errorf("Goroutine %d: Expected %d notifications, got %d", 
					id, expectedTotal, len(notifications))
			}
		}(i)
	}
	wg2.Wait()
}

func TestMemoryStorage_ConcurrentMixedOperations(t *testing.T) {
	storage := NewMemoryStorage()
	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Каждая горутина делает и Store и Get

	for i := 0; i < numGoroutines; i++ {
		// Горутина для записи
		go func(id int) {
			defer wg.Done()
			notification := &models.Notification{
				ChatID: string(rune(id + 65)),
				Text:   "Notification from goroutine",
			}
			err := storage.Store(notification)
			if err != nil {
				t.Errorf("Write goroutine %d failed: %v", id, err)
			}
		}(i)

		// Горутина для чтения
		go func(id int) {
			defer wg.Done()
			notifications := storage.GetNotifications()
			sentNotifications := storage.GetSentNotifications()
			_ = notifications // Используем переменные чтобы избежать предупреждений
			_ = sentNotifications
		}(i)
	}

	wg.Wait()

	// Проверяем что все записи сохранены
	notifications := storage.GetNotifications()
	if len(notifications) != numGoroutines {
		t.Errorf("Expected %d notifications, got %d", numGoroutines, len(notifications))
	}
}

func TestMemoryStorage_InterfaceImplementation(t *testing.T) {
	// Проверяем что MemoryStorage реализует интерфейс Storage
	var _ Storage = (*MemoryStorage)(nil)
	
	storage := NewMemoryStorage()
	
	// Проверяем что методы работают через интерфейс
	var s Storage = storage
	
	err := s.Store(&models.Notification{ChatID: "123", Text: "Test"})
	if err != nil {
		t.Errorf("Store via interface failed: %v", err)
	}
	
	notifications := s.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification via interface, got %d", len(notifications))
	}
}