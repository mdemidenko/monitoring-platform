package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/logger"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/notifier"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

func main() {
	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	// Создаем репозиторий для слайсов
	storage := repository.NewMemoryStorage()

	// Создаем и запускаем логгер хранилища с контекстом
	storageLogger := logger.NewStorageLogger(storage, 200*time.Millisecond)
	storageLogger.Start(ctx) // передаем контекст
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

	log.Printf("Начинаем параллельную обработку %d уведомлений с интервалами...", len(notifications))

	// Канал для получения сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем обработку уведомлений с интервалами в отдельной горутине
	results := make(chan processResult, 1)
	go func() {
		successCount, errorCount := processNotificationsWithIntervals(ctx, telegramService, notifications, 2*time.Second)
		results <- processResult{successCount: successCount, errorCount: errorCount}
	}()

	// Ожидаем либо завершения обработки, либо сигнала ОС
	select {
	case <-sigChan:
		log.Println("🚨 Получен сигнал завершения, начинаем graceful shutdown...")
		cancel() // Отменяем контекст - уведомляем все горутины о завершении
		
		// Даем время на graceful shutdown
		select {
		case result := <-results:
			log.Printf("\n=== ИТОГИ ПАРАЛЛЕЛЬНОЙ ОБРАБОТКИ ===")
			log.Printf("Успешно отправлено: %d", result.successCount)
			log.Printf("Ошибок: %d", result.errorCount)
		case <-time.After(5 * time.Second):
			log.Println("⚠️  Таймаут graceful shutdown, принудительное завершение")
		}
	case result := <-results:
		log.Printf("\n=== ИТОГИ ПАРАЛЛЕЛЬНОЙ ОБРАБОТКИ ===")
		log.Printf("Успешно отправлено: %d", result.successCount)
		log.Printf("Ошибок: %d", result.errorCount)
	}

	// Даем время логгеру обработать последние изменения
	time.Sleep(300 * time.Millisecond)

	// Выводим статистику хранилища
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification в слайсе: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification в слайсе: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
	
	log.Println("👋 Приложение завершено")
}

// processResult результат обработки всех уведомлений
type processResult struct {
	successCount int
	errorCount   int
}

// processNotificationsWithIntervals обрабатывает уведомления с интервалами между отправками
func processNotificationsWithIntervals(ctx context.Context, service *notifier.TelegramService, notifications []*models.Notification, interval time.Duration) (successCount, errorCount int) {
	// Создаем каналы для коммуникации
	jobs := make(chan *models.Notification, len(notifications)) // Канал для заданий
	results := make(chan *workerResult, len(notifications))     // Канал для результатов
	done := make(chan bool)                                     // Канал для сигнала завершения

	var wg sync.WaitGroup

	// Запускаем worker'ы (горутины)
	numWorkers := 2

	// Можно настроить количество параллельных worker'ов
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go notificationWorker(ctx, i+1, &wg, jobs, results, service)
	}

	// Отправляем уведомления в канал jobs с интервалами
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		sentCount := 0

		for {
			select {
			case <-ctx.Done():
				log.Println("⏹️  Прерывание отправки уведомлений по сигналу")
				close(jobs)
				return
			case <-ticker.C:
				if sentCount >= len(notifications) {
					// Все уведомления отправлены в канал
					close(jobs)
					log.Printf("✅ Все %d уведомлений поставлены в очередь", sentCount)
					return
				}

				notification := notifications[sentCount]
				log.Printf("📨 Постановка в очередь уведомления %d: %s", sentCount+1, notification.Text)
				
				select {
				case <-ctx.Done():
					log.Println("⏹️  Прерывание отправки уведомлений по сигналу")
					close(jobs)
					return
				case jobs <- notification:
					sentCount++
					log.Printf("⏰ Следующее уведомление через %v", interval)
				}
			}
		}
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
	for {
		select {
		case <-ctx.Done():
			log.Println("⏹️  Прерывание обработки результатов по сигналу")
			// Ждем завершения воркеров с таймаутом
			select {
			case <-done:
				log.Println("✅ Все воркеры завершили работу")
			case <-time.After(2 * time.Second):
				log.Println("⚠️  Таймаут ожидания завершения воркеров")
			}
			return successCount, errorCount
		case result, ok := <-results:
			if !ok {
				// Канал results закрыт, все результаты обработаны
				<-done
				return successCount, errorCount
			}
			if result.Error != nil {
				log.Printf("❌ Ошибка обработки уведомления '%s': %v", result.Text, result.Error)
				errorCount++
			} else {
				log.Printf("✅ Уведомление успешно обработано: %s", result.Text)
				successCount++
			}
		}
	}
}

// workerResult результат обработки уведомления воркером
type workerResult struct {
	Text  string
	Error error
}

// notificationWorker обрабатывает уведомления из канала jobs
func notificationWorker(ctx context.Context, workerID int, wg *sync.WaitGroup, jobs <-chan *models.Notification, results chan<- *workerResult, service *notifier.TelegramService) {
	defer wg.Done()

	log.Printf("Worker %d запущен", workerID)
	defer log.Printf("👷 Worker %d завершил работу", workerID)

	// Читаем уведомления из канала пока он не закроется
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d получил сигнал завершения", workerID)
			return
		case notification, ok := <-jobs:
			if !ok {
				// Канал jobs закрыт, выходим
				return
			}

			log.Printf("Worker %d обрабатывает: %s", workerID, notification.Text)

			// Обрабатываем уведомление с контекстом
			err := service.ProcessEntity(ctx, notification)

			// Отправляем результат в канал results
			select {
			case <-ctx.Done():
				log.Printf("Worker %d прерван при отправке результата", workerID)
				return
			case results <- &workerResult{
				Text:  notification.Text,
				Error: err,
			}:
				// Результат успешно отправлен
			}
		}
	}
}