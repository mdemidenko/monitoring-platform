package logger

import (
    "context"
    "testing"
    "time"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
    "github.com/stretchr/testify/assert"
)

// TestStorageLogger_New проверяет создание логгера
func TestStorageLogger_New(t *testing.T) {
    storage := &repository.MemoryStorage{}
    interval := 1 * time.Second

    logger := NewStorageLogger(storage, interval)

    assert.NotNil(t, logger)
    assert.Equal(t, storage, logger.storage)
    assert.Equal(t, interval, logger.interval)
}

// TestStorageLogger_Start_Stop проверяет вызов Start и Stop
func TestStorageLogger_Start_Stop(t *testing.T) {
    storage := &repository.MemoryStorage{}
    logger := NewStorageLogger(storage, 100*time.Millisecond)

    // Start запускает горутину
    ctx, cancel := context.WithCancel(context.Background())
    logger.Start(ctx)
    defer cancel()

    // Даём время на запуск
    time.Sleep(50 * time.Millisecond)

    // Stop — просто вызов
    logger.Stop()
}

// TestStorageLogger_monitor_ContextCancel проверяет остановку по контексту
func TestStorageLogger_monitor_ContextCancel(t *testing.T) {
    storage := &repository.MemoryStorage{}
    logger := NewStorageLogger(storage, 10*time.Millisecond)

    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan bool, 1)

    // Запускаем monitor в горутине
    go func() {
        logger.monitor(ctx)
        done <- true
    }()

    // Даём время на старт
    time.Sleep(30 * time.Millisecond)

    // Останавливаем
    cancel()

    // Ждём завершения
    select {
    case <-done:
        // OK: monitor завершился
    case <-time.After(200 * time.Millisecond):
        t.Fatal("monitor не завершился после отмены контекста")
    }
}

// TestStorageLogger_checkForChanges_NewNotifications проверяет обнаружение новых Notification
func TestStorageLogger_checkForChanges_NewNotifications(t *testing.T) {
    storage := &repository.MemoryStorage{}
    logger := NewStorageLogger(storage, 100*time.Millisecond)

    // Добавляем уведомления
    notification1 := &models.Notification{ChatID: "123", Text: "Hello"}
    notification2 := &models.Notification{ChatID: "456", Text: "World"}
    storage.Store(notification1)
    storage.Store(notification2)

    // Инициализируем состояние
    lastNotifications := make([]*models.Notification, 0)
    lastSentNotifications := make([]*models.SentNotification, 0)
    lastNotificationCount := 0
    lastSentNotificationCount := 0

    // Проверяем изменения
    logger.checkForChanges(&lastNotifications, &lastSentNotifications,
        &lastNotificationCount, &lastSentNotificationCount)

    // Проверяем, что состояние обновилось
    assert.Equal(t, 2, len(lastNotifications))
    assert.Equal(t, 2, lastNotificationCount)
    assert.Equal(t, notification1.ChatID, lastNotifications[0].ChatID)
    assert.Equal(t, notification2.ChatID, lastNotifications[1].ChatID)
}

// TestStorageLogger_checkForChanges_NewSentNotifications проверяет обнаружение новых SentNotification
func TestStorageLogger_checkForChanges_NewSentNotifications(t *testing.T) {
    storage := &repository.MemoryStorage{}
    logger := NewStorageLogger(storage, 100*time.Millisecond)

    // Добавляем отправленные уведомления
    sent1 := &models.SentNotification{MessageID: 1, ChatID: 123}
    sent2 := &models.SentNotification{MessageID: 2, ChatID: 456}
    storage.Store(sent1)
    storage.Store(sent2)

    // Инициализируем состояние
    lastNotifications := make([]*models.Notification, 0)
    lastSentNotifications := make([]*models.SentNotification, 0)
    lastNotificationCount := 0
    lastSentNotificationCount := 0

    // Проверяем изменения
    logger.checkForChanges(&lastNotifications, &lastSentNotifications,
        &lastNotificationCount, &lastSentNotificationCount)

    // Проверяем, что состояние обновилось
    assert.Equal(t, 2, len(lastSentNotifications))
    assert.Equal(t, 2, lastSentNotificationCount)
    assert.Equal(t, sent1.MessageID, lastSentNotifications[0].MessageID)
    assert.Equal(t, sent2.MessageID, lastSentNotifications[1].MessageID)
}

// TestStorageLogger_checkForChanges_NoChanges проверяет отсутствие изменений
func TestStorageLogger_checkForChanges_NoChanges(t *testing.T) {
    storage := &repository.MemoryStorage{}
    logger := NewStorageLogger(storage, 100*time.Millisecond)

    // Добавляем уведомления
    notification := &models.Notification{ChatID: "123", Text: "Hello"}
    storage.Store(notification)

    // Инициализируем состояние
    lastNotifications := make([]*models.Notification, 0)
    lastSentNotifications := make([]*models.SentNotification, 0)
    lastNotificationCount := 0
    lastSentNotificationCount := 0

    // Первый вызов — должен обновить состояние
    logger.checkForChanges(&lastNotifications, &lastSentNotifications,
        &lastNotificationCount, &lastSentNotificationCount)

    // Сохраняем состояние
    prevNotificationCount := lastNotificationCount
    prevLen := len(lastNotifications)

    // Второй вызов — без изменений
    logger.checkForChanges(&lastNotifications, &lastSentNotifications,
        &lastNotificationCount, &lastSentNotificationCount)

    // Состояние не должно измениться
    assert.Equal(t, prevNotificationCount, lastNotificationCount)
    assert.Equal(t, prevLen, len(lastNotifications))
}

// TestStorageLogger_checkForChanges_BothChanges проверяет обновление обоих типов
func TestStorageLogger_checkForChanges_BothChanges(t *testing.T) {
    storage := &repository.MemoryStorage{}
    logger := NewStorageLogger(storage, 100*time.Millisecond)

    // Добавляем оба типа
    notification := &models.Notification{ChatID: "123", Text: "Hello"}
    sent := &models.SentNotification{MessageID: 1, ChatID: 123}
    storage.Store(notification)
    storage.Store(sent)

    // Инициализируем состояние
    lastNotifications := make([]*models.Notification, 0)
    lastSentNotifications := make([]*models.SentNotification, 0)
    lastNotificationCount := 0
    lastSentNotificationCount := 0

    // Проверяем изменения
    logger.checkForChanges(&lastNotifications, &lastSentNotifications,
        &lastNotificationCount, &lastSentNotificationCount)

    // Проверяем обновление обоих списков
    assert.Equal(t, 1, len(lastNotifications))
    assert.Equal(t, 1, lastNotificationCount)
    assert.Equal(t, "123", lastNotifications[0].ChatID)

    assert.Equal(t, 1, len(lastSentNotifications))
    assert.Equal(t, 1, lastSentNotificationCount)
    assert.Equal(t, int64(1), lastSentNotifications[0].MessageID)
}