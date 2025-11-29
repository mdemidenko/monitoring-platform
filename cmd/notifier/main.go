package main

import (
	"log"
	"sync"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/logger"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/notifier"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	// Создаем репозиторий для слайсов
	storage := repository.NewMemoryStorage()

	// Создаем и запускаем логгер хранилища
	storageLogger := logger.NewStorageLogger(storage, 200*time.Millisecond)
	storageLogger.Start()
	defer storageLogger.Stop()

	// Создаем сервис и передаем в него репозиторий
	telegramService := notifier.NewTelegramService(cfg, storage)

	// Проверяем здоровье бота
	if err := telegramService.HealthCheck(); err != nil {
		log.Fatal(err)
	}

	// Предопределяем уведомления
	notifications := []*models.Notification{
		{ChatID: cfg.Telegram.ChatID, Text: "🔔 Проверка системы!"},
		{ChatID: cfg.Telegram.ChatID, Text: "✅ Проверка прошла успешно"},
		{ChatID: cfg.Telegram.ChatID, Text: "⚠️ Предупреждение системы"},
		{ChatID: cfg.Telegram.ChatID, Text: "📊 Статистика работы"},
	}

	log.Printf("Начинаем параллельную обработку %d уведомлений...", len(notifications))

	// Параллельная обработка с горутинами и каналами
	successCount, errorCount := processNotificationsParallel(telegramService, notifications)

	// Даем время логгеру обработать последние изменения
	time.Sleep(300 * time.Millisecond)

	log.Printf("\n=== ИТОГИ ПАРАЛЛЕЛЬНОЙ ОБРАБОТКИ ===")
	log.Printf("Успешно отправлено: %d", successCount)
	log.Printf("Ошибок: %d", errorCount)

	// Выводим статистику хранилища
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification в слайсе: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification в слайсе: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
}

// processNotificationsParallel обрабатывает уведомления параллельно с использованием горутин и каналов
func processNotificationsParallel(service *notifier.TelegramService, notifications []*models.Notification) (successCount, errorCount int) {
	// Создаем каналы для коммуникации
	jobs := make(chan *models.Notification, len(notifications))    // Канал для заданий
	results := make(chan *processResult, len(notifications))       // Канал для результатов
	done := make(chan bool)                                        // Канал для сигнала завершения

	var wg sync.WaitGroup

	// Запускаем worker'ы (горутины)
	numWorkers := 2
	
	// Можно настроить количество параллельных worker'ов
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go notificationWorker(i+1, &wg, jobs, results, service)
	}

	// Отправляем уведомления в канал jobs
	go func() {
		for i, notification := range notifications {
			log.Printf("📨 Постановка в очередь уведомления %d: %s", i+1, notification.Text)
			jobs <- notification
		}
		close(jobs) // Закрываем канал после отправки всех заданий
	}()

	// Ждем завершения всех worker'ов и закрываем канал results
	go func() {
		wg.Wait()
		close(results)
		done <- true
	}()

	// Обрабатываем результаты из канала results
	successCount = 0
	errorCount = 0

	// Читаем результаты пока канал не закроется
	for result := range results {
		if result.Error != nil {
			log.Printf("❌ Ошибка обработки уведомления '%s': %v", result.Text, result.Error)
			errorCount++
		} else {
			log.Printf("✅ Уведомление успешно обработано: %s", result.Text)
			successCount++
		}
	}

	// Ждем сигнал завершения
	<-done

	return successCount, errorCount
}

// processResult результат обработки уведомления
type processResult struct {
	Text  string
	Error error
}

// notificationWorker обрабатывает уведомления из канала jobs
func notificationWorker(workerID int, wg *sync.WaitGroup, jobs <-chan *models.Notification, results chan<- *processResult, service *notifier.TelegramService) {
	defer wg.Done()

	log.Printf("Worker %d запущен", workerID)

	// Читаем уведомления из канала пока он не закроется
	for notification := range jobs {
		log.Printf("Worker %d обрабатывает: %s", workerID, notification.Text)

		// Обрабатываем уведомление
		err := service.ProcessEntity(notification)

		// Отправляем результат в канал results
		results <- &processResult{
			Text:  notification.Text,
			Error: err,
		}
	}

	log.Printf("👷 Worker %d завершил работу", workerID)
}