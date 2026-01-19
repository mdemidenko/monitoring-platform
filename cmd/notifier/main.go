package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/api"
	"github.com/mdemidenko/monitoring-platform/internal/logger"
	"github.com/mdemidenko/monitoring-platform/internal/notifier"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
	_ "github.com/mdemidenko/monitoring-platform/docs"
)

// @title Monitoring Platform API
// @version 1.0
// @description API для отправки уведомлений через Telegram с JWT аутентификацией
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите JWT токен в формате: Bearer {your_jwt_token}

// @tag.name auth
// @tag.description Эндпоинты для аутентификации

// @tag.name health
// @tag.description Проверка состояния сервиса

// @tag.name notifications
// @tag.description Управление уведомлениями

// @tag.name status
// @tag.description Статус и статистика сервиса

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

	// Создаем и запускаем web-сервер
	server := api.NewServer(telegramService, storage, cfg)
	go server.Start(cfg.Server.Port)

	log.Println("🚀 Приложение запущено")
	log.Printf("📡 Web-сервер доступен на http://%s:%s", cfg.Server.Host, cfg.Server.Port)

	// Канал для получения сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал завершения
	<-sigChan
	log.Println("🚨 Получен сигнал завершения, начинаем graceful shutdown...")

	// Graceful shutdown web-сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.Timeout)*time.Second)
	defer shutdownCancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("⚠️ Ошибка при остановке сервера: %v", err)
	}

	// Отменяем контекст для остальных компонентов
	cancel()
	time.Sleep(300 * time.Millisecond)

	// Выводим статистику хранилища
	printStorageStats(storage)
	log.Println("👋 Приложение завершено")
}

// printStorageStats выводит статистику хранилища
func printStorageStats(storage *repository.MemoryStorage) {
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
}