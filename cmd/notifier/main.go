package main

import (
	"context"
	"log"
	"os"
	"os/signal"
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
	storageLogger.Start(ctx)

	// Создаем сервис
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

	log.Printf("Начинаем обработку %d уведомлений с интервалами...", len(notifications))

	// Канал для получения сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем обработку уведомлений в отдельной горутине
	results := make(chan notifier.ProcessResult, 1)
	go func() {
		result := telegramService.ProcessWithIntervals(ctx, notifications, 2*time.Second, 2)
		results <- result
	}()

	// Ожидаем либо завершения обработки, либо сигнала ОС
	select {
	case <-sigChan:
		log.Println("🚨 Получен сигнал завершения, начинаем graceful shutdown...")
		cancel()
		
		// Даем время на graceful shutdown
		select {
		case result := <-results:
			printResults(result)
		case <-time.After(5 * time.Second):
			log.Println("⚠️  Таймаут graceful shutdown, принудительное завершение")
		}
	case result := <-results:
		printResults(result)
		log.Println("🔄 Завершаем логгер...")
		cancel()
		time.Sleep(300 * time.Millisecond)
	}

	// Выводим статистику хранилища
	printStorageStats(storage)
	log.Println("👋 Приложение завершено")
}

// printResults выводит итоги обработки
func printResults(result notifier.ProcessResult) {
	log.Printf("\n=== ИТОГИ ОБРАБОТКИ ===")
	log.Printf("Успешно отправлено: %d", result.SuccessCount)
	log.Printf("Ошибок: %d", result.ErrorCount)
}

// printStorageStats выводит статистику хранилища
func printStorageStats(storage *repository.MemoryStorage) {
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
}