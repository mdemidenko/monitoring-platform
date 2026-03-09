package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/mdemidenko/monitoring-platform/config"
    "github.com/mdemidenko/monitoring-platform/internal/client"
    "github.com/mdemidenko/monitoring-platform/internal/monitor"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
    "github.com/mdemidenko/monitoring-platform/internal/loader"
)

func main() {
    // Устанавливаем формат логов: дата, время с микросекундами, файл:строка
    log.SetFlags(log.LstdFlags)

    // Загружаем конфигурацию
    appConfig, err := config.LoadConfig("")
    if err != nil {
        log.Fatalf("Не удалось загрузить конфиг: %v", err)
    }
    cfg := appConfig.File

    log.Printf("Конфигурация загружена: workers=%d, batch=%d, timeout=%v",
        cfg.Workers, cfg.BatchSize, cfg.ShutdownTimeout)
    log.Printf("MongoDB: uri=%s, db=%s, collection=%s",
        cfg.MongoDBURI, cfg.DBName, cfg.CollectionName)

    // Создаем контекст с graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Настраиваем обработку сигналов
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-stop
        log.Println("Получен сигнал остановки. Начинаем graceful shutdown...")
        cancel()

        // Второй сигнал — принудительный выход
        <-stop
        log.Println("Принудительный выход!")
        os.Exit(1)
    }()

    // === ШАГ 1: Подключаемся к MongoDB ===
    mongoRepo, err := repository.NewMongoDBRepository(cfg.MongoDBURI, cfg.DBName, cfg.CollectionName)
    if err != nil {
        log.Fatalf("Не удалось подключиться к MongoDB: %v", err)
    }
    collection := mongoRepo.GetCollection()

    // === ШАГ 2: Загрузка, фильтрация и сохранение ===
    log.Println("🔄 Загрузка и фильтрация сервисов...")
    processedCount, err := loader.LoadAndFilterServices(ctx, collection, cfg.InputFile)
    if err != nil {
        log.Fatalf("Ошибка загрузки с фильтрацией: %v", err)
    }
    log.Printf("✅ Отфильтровано и загружено: %d сервисов", processedCount)

    // === ШАГ 3: Обогащение — делаем запросы и обновляем кластеры ===
    log.Println("🚀 Запуск обогащения кластерами...")

    // Создаём HTTP-клиент для внешней системы
    passportClient := client.NewPassportClient("https://smesre.tcsgroup.io/passport/v2")

    // Создаём сервис для обогащения
    enricher := monitor.NewEnricher(mongoRepo, passportClient, collection)

    // Запускаем обогащение
    if err := enricher.EnrichServices(ctx, cfg.Workers); err != nil && err != context.Canceled {
        log.Printf("Ошибка обогащения: %v", err)
        os.Exit(1)
    }

    log.Println("✅ Приложение успешно завершено")
}