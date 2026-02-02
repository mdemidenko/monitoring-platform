package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/grpc"
	"github.com/mdemidenko/monitoring-platform/internal/logger"
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

	log.Printf("🚀 Запуск gRPC сервера для %s v%s", cfg.App.Name, cfg.App.Version)

	// Создаем репозиторий для слайсов
	storage := repository.NewMemoryStorage()

	// Создаем и запускаем логгер хранилища с контекстом
	storageLogger := logger.NewStorageLogger(storage, 200*time.Millisecond)
	storageLogger.Start(ctx)

	// Создаем сервис Telegram
	telegramService := notifier.NewTelegramService(cfg, storage)

	// Проверяем здоровье бота
	if err := telegramService.HealthCheck(); err != nil {
		log.Fatal(err)
	}

	// Создаем gRPC сервер
	grpcServer, err := grpc.NewGRPCServer(cfg, telegramService, storage)
	if err != nil {
		log.Fatalf("❌ Не удалось создать gRPC сервер: %v", err)
	}
	
	// Запускаем gRPC сервер в отдельной горутине
	go func() {
		grpcPort := cfg.Server.GRPCPort
		if grpcPort == "" {
			grpcPort = "9090"
		}
		log.Printf("🚀 gRPC сервер запускается на %s:%s", cfg.Server.Host, grpcPort)
		
		if err := grpcServer.Start(grpcPort); err != nil {
			log.Fatalf("❌ Не удалось запустить gRPC сервер: %v", err)
		}
	}()

	// Канал для получения сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал завершения
	<-sigChan
	log.Println("🚨 Получен сигнал завершения, начинаем graceful shutdown...")

	// Graceful shutdown gRPC сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.Timeout)*time.Second)
	defer shutdownCancel()
	
	if grpcServer != nil {
		if err := grpcServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("⚠️ Ошибка при остановке gRPC сервера: %v", err)
		}
	}

	// Отменяем контекст для остальных компонентов
	cancel()
	time.Sleep(300 * time.Millisecond)

	// Выводим статистику хранилища
	printStorageStats(storage)
	log.Println("👋 gRPC сервер завершен")
}

// printStorageStats выводит статистику хранилища
func printStorageStats(storage *repository.MemoryStorage) {
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
}