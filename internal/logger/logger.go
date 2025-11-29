package logger

import (
	"context"
	"log"
	"time"

	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

// StorageLogger мониторит изменения в хранилище и логирует новые структуры
type StorageLogger struct {
	storage    *repository.MemoryStorage
	interval   time.Duration
}

// NewStorageLogger создает новый логгер хранилища
func NewStorageLogger(storage *repository.MemoryStorage, interval time.Duration) *StorageLogger {
	return &StorageLogger{
		storage:   storage,
		interval:  interval,
	}
}

// Start запускает логгер в отдельной горутине с поддержкой контекста
func (sl *StorageLogger) Start(ctx context.Context) {
	log.Printf("📊 Логгер хранилища запущен (интервал проверки: %v)", sl.interval)

	go sl.monitor(ctx)
}

// Stop останавливает логгер (оставляем для обратной совместимости)
func (sl *StorageLogger) Stop() {
	log.Printf("📊 Логгер хранилища остановлен")
}

// monitor осуществляет мониторинг изменений в хранилище
func (sl *StorageLogger) monitor(ctx context.Context) {
	// Состояние для отслеживания изменений
	lastNotifications := make([]*models.Notification, 0)
	lastSentNotifications := make([]*models.SentNotification, 0)
	lastNotificationCount := 0
	lastSentNotificationCount := 0

	ticker := time.NewTicker(sl.interval)
	defer ticker.Stop()

	log.Printf("📊 Мониторинг хранилища начат")

	for {
		select {
		case <-ctx.Done():
			// Контекст отменен - завершаем работу
			log.Printf("📊 Логгер хранилища завершает работу")
			return
		case <-ticker.C:
			sl.checkForChanges(&lastNotifications, &lastSentNotifications, 
				&lastNotificationCount, &lastSentNotificationCount)
		}
	}
}

// checkForChanges проверяет изменения в хранилище и логирует новые структуры
func (sl *StorageLogger) checkForChanges(
	lastNotifications *[]*models.Notification,
	lastSentNotifications *[]*models.SentNotification,
	lastNotificationCount *int,
	lastSentNotificationCount *int,
) {
	// Получаем текущее состояние хранилища
	currentNotifications := sl.storage.GetNotifications()
	currentSentNotifications := sl.storage.GetSentNotifications()
	currentNotificationCount := len(currentNotifications)
	currentSentNotificationCount := len(currentSentNotifications)

	hasChanges := false

	// Проверяем изменения в Notification
	if currentNotificationCount > *lastNotificationCount {
		newNotifications := currentNotifications[*lastNotificationCount:]
		for _, notification := range newNotifications {
			log.Printf("📝 НОВЫЙ Notification: ChatID=%s, Text='%s'", 
				notification.ChatID, notification.Text)
		}
		*lastNotifications = currentNotifications
		*lastNotificationCount = currentNotificationCount
		hasChanges = true
	}

	// Проверяем изменения в SentNotification
	if currentSentNotificationCount > *lastSentNotificationCount {
		newSentNotifications := currentSentNotifications[*lastSentNotificationCount:]
		for _, sentNotification := range newSentNotifications {
			log.Printf("📝 НОВЫЙ SentNotification: MessageID=%d, ChatID=%d", 
				sentNotification.MessageID, sentNotification.ChatID)
		}
		*lastSentNotifications = currentSentNotifications
		*lastSentNotificationCount = currentSentNotificationCount
		hasChanges = true
	}

	// Логируем общую статистику при изменениях
	if hasChanges {
		log.Printf("📊 Статистика: Notifications=%d, SentNotifications=%d", 
			currentNotificationCount, currentSentNotificationCount)
	}
}