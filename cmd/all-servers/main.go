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
	"github.com/mdemidenko/monitoring-platform/internal/grpc"
	"github.com/mdemidenko/monitoring-platform/internal/logger"
	"github.com/mdemidenko/monitoring-platform/internal/notifier"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
	_ "github.com/mdemidenko/monitoring-platform/docs"
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

	log.Printf("🚀 Запуск всех серверов для %s v%s", cfg.App.Name, cfg.App.Version)

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

	// Запускаем HTTP сервер
	httpServer := api.NewServer(telegramService, storage, cfg)
	go func() {
		log.Printf("🚀 HTTP сервер запускается на %s:%s", cfg.Server.Host, cfg.Server.Port)
		httpServer.Start(cfg.Server.Port)
	}()

	// Запускаем gRPC сервер
	grpcServer, err := grpc.NewGRPCServer(cfg, telegramService, storage)
	if err != nil {
		log.Fatalf("❌ Не удалось создать gRPC сервер: %v", err)
	}
	
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

	log.Println("✅ Все серверы запущены")
	log.Printf("📡 HTTP endpoints: http://%s:%s/swagger/index.html", cfg.Server.Host, cfg.Server.Port)
	log.Printf("📡 gRPC endpoints: %s:%s", cfg.Server.Host, cfg.Server.GRPCPort)

	// Канал для получения сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал завершения
	<-sigChan
	log.Println("🚨 Получен сигнал завершения, начинаем graceful shutdown...")

	// Graceful shutdown серверов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.Timeout)*time.Second)
	defer shutdownCancel()
	
	// Останавливаем HTTP сервер
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("⚠️ Ошибка при остановке HTTP сервера: %v", err)
	}

	// Останавливаем gRPC сервер
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
	log.Println("👋 Все серверы завершены")
}

// printStorageStats выводит статистику хранилища
func printStorageStats(storage *repository.MemoryStorage) {
	log.Printf("\n=== СТАТИСТИКА ХРАНИЛИЩА ===")
	log.Printf("Созданных Notification: %d", len(storage.GetNotifications()))
	log.Printf("Отправленных SentNotification: %d", len(storage.GetSentNotifications()))
	log.Printf("Всего элементов: %d", len(storage.GetNotifications())+len(storage.GetSentNotifications()))
}