package main

import (
	"log"
	"monitoring-platform/config"
	"monitoring-platform/internal/models"
	"monitoring-platform/internal/notifier"
	"monitoring-platform/internal/repository"
)

func main() {
	
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	// Создаем репозиторий для слайсов
	storage := repository.NewMemoryStorage()

	// Создаем сервис и передаем в него репозиторий

	telegramService := notifier.NewTelegramService(cfg, storage)

	// Проверяем здоровье бота
	if err := telegramService.HealthCheck(); err != nil {
		log.Fatal(err)
	}

	// Предопределяем уведомления и отправляем
		notifications := []*models.Notification{
			{ChatID: cfg.Telegram.ChatID, Text: "🔔 Проверка системы!"},
			{ChatID: cfg.Telegram.ChatID, Text: "✅ Проверка прошла успешно"},
			{ChatID: cfg.Telegram.ChatID, Text: "⚠️ Предупреждение системы"},
			{ChatID: cfg.Telegram.ChatID, Text: "📊 Статистика работы"},
	}

	for i, notification := range notifications {
		log.Printf("--- Обработка уведомления %d ---", i+1)
		log.Printf("Текст: %s", notification.Text)
		
		if err := telegramService.ProcessEntity(notification); err != nil {
			log.Printf("Ошибка обработки: %v", err)
			continue
		}
		
		log.Printf("Уведомление успешно обработано и отправлено")
	}

	// Выводим статистку хранилища
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification в слайсе: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification в слайсе: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
}

