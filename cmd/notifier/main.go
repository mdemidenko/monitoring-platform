package main

import (
	
	"log"
	"monitoring-platform/config"
	"monitoring-platform/internal/notifier"
	
)

func main() {
	
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	// Создаем сервис

	telegramService := notifier.NewTelegramService(cfg)

	// Проверяем здоровье бота
	if err := telegramService.HealthCheck(); err != nil {
		log.Fatal(err)
	}

	// Отправляем уведомления
	result, err := telegramService.SendNotification("🔔 Проверка системы!")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Уведомление отправлено! ID: %d", result.MessageID)

	// Еще одно уведомление
	result, err = telegramService.SendNotification("✅ Проверка прошла успешно")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Уведомление отправлено! ID: %d", result.MessageID)
}
