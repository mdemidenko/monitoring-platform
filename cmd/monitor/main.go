package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/monitor"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

func main() {
    // Загружаем конфигурацию
    cfg := config.FileLoadConfig()
    log.Printf("Конфигурация загружена: workers=%d, batch=%d, timeout=%v", 
        cfg.Workers, cfg.BatchSize, cfg.ShutdownTimeout)

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
        
        // Второй сигнал - принудительный выход
        <-stop
        log.Println("Принудительный выход!")
        os.Exit(1)
    }()

    // Инициализация зависимостей
    repo := repository.NewRepository(cfg.InputFile, cfg.OutputFile)
    svc := monitor.New(repo)

    // Запускаем обработку
    if err := processWithContext(ctx, svc, repo, cfg); err != nil {
        log.Printf("Ошибка: %v", err)
        os.Exit(1)
    }

    log.Println("Приложение успешно завершено")
}

func processWithContext(ctx context.Context, svc monitor.Service, repo repository.Repository, cfg config.FileConfig) error {
    log.Println("Начало обработки...")
    startTime := time.Now()

    // Создаем каналы для конвейера
    resultsChan, procErrChan := svc.FilterServicesBatch(ctx, cfg.Workers)
    
    // Счетчик обработанных результатов
    var resultCount int32
    
    // Канал для сбора результатов
    collectedResults := make(chan models.Result, 100)
    
    // Горутина для сбора и подсчета результатов
    go func() {
        defer close(collectedResults)
        for result := range resultsChan {
            atomic.AddInt32(&resultCount, 1)
            
            // Выводим прогресс каждые 100 записей
            if atomic.LoadInt32(&resultCount)%10 == 0 {
                log.Printf("Обработано: %d записей", atomic.LoadInt32(&resultCount))
            }
            
            select {
            case <-ctx.Done():
                return
            case collectedResults <- result:
            }
        }
    }()
    
    // Сохраняем результаты
    saveErrChan := repo.SaveResults(ctx, collectedResults)
    
    // Ожидаем завершения и проверяем ошибки
    var saveErr, procErr error
    
    select {
    case saveErr = <-saveErrChan:
    case <-ctx.Done():
        return ctx.Err()
    }
    
    select {
    case procErr = <-procErrChan:
    default:
    }
    
    // Обрабатываем ошибки
    if procErr != nil && procErr != context.Canceled {
        return fmt.Errorf("ошибка обработки: %w", procErr)
    }
    
    if saveErr != nil && saveErr != context.Canceled {
        return fmt.Errorf("ошибка сохранения: %w", saveErr)
    }
    
    // Выводим итоговую статистику
    finalCount := atomic.LoadInt32(&resultCount)
    elapsed := time.Since(startTime)
    
    log.Printf("========================================")
    log.Printf("ОБРАБОТКА ЗАВЕРШЕНА")
    log.Printf("Всего времени: %v", elapsed)
    log.Printf("Найдено подходящих сервисов: %d", finalCount)
    log.Printf("Скорость обработки: %.2f записей/сек", 
        float64(finalCount)/elapsed.Seconds())
    log.Printf("========================================")
    
    return nil
}