package core

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/mdemidenko/monitoring-platform/internal/domain"
)

// NotificationService реализует бизнес-логику уведомлений
type NotificationService struct {
    repo   domain.NotificationRepository
    sender domain.NotificationSender
    logger *log.Logger
}

// NewNotificationService создает новый сервис нотификаций
func NewNotificationService(
    repo domain.NotificationRepository,
    sender domain.NotificationSender,
    logger *log.Logger,
) *NotificationService {
    if logger == nil {
        logger = log.Default()
    }
    
    return &NotificationService{
        repo:   repo,
        sender: sender,
        logger: logger,
    }
}

// SendNotification отправляет одно уведомление
// Реализует domain.NotificationService.SendNotification
func (s *NotificationService) SendNotification(ctx context.Context, chatID, text string) (*domain.SentNotification, error) {
    // 1. Валидация входных данных (бизнес-правила)
    if text == "" {
        return nil, domain.NewDomainError(domain.ErrValidation, "text cannot be empty", nil)
    }
    
    if chatID == "" {
        return nil, domain.NewDomainError(domain.ErrValidation, "chat_id cannot be empty", nil)
    }
    
    // 2. Создание доменной сущности
    notification := domain.NewNotification(chatID, text)
    
    // 3. Сохранение уведомления (входящий запрос)
    if err := s.repo.Store(notification); err != nil {
        return nil, domain.NewDomainError(domain.ErrRepository, "failed to store notification", err)
    }
    
    s.logger.Printf("Notification stored: chatID=%s, text=%s", chatID, text)
    
    // 4. Отправка уведомления через адаптер
    sentNotification, err := s.sender.Send(ctx, notification)
    if err != nil {
        return nil, domain.NewDomainError(domain.ErrExternalService, "failed to send notification", err)
    }
    
    // 5. Сохранение результата отправки
    if sentNotification != nil {
        if err := s.repo.Store(sentNotification); err != nil {
            s.logger.Printf("Failed to store sent notification: %v", err)
            // Не прерываем выполнение, только логируем
        }
    }
    
    s.logger.Printf("Notification sent successfully: messageID=%d", sentNotification.MessageID)
    
    return sentNotification, nil
}

// ProcessEntity обрабатывает сущность
// Реализует domain.NotificationService.ProcessEntity
func (s *NotificationService) ProcessEntity(ctx context.Context, entity interface{}) error {
    // Проверяем контекст перед началом работы
    if err := ctx.Err(); err != nil {
        return fmt.Errorf("operation cancelled: %w", err)
    }
    
    // 1. Сохраняем входящую сущность
    if err := s.repo.Store(entity); err != nil {
        return domain.NewDomainError(domain.ErrRepository, "failed to store entity", err)
    }
    
    // 2. Дополнительная бизнес-логика в зависимости от типа
    switch v := entity.(type) {
    case *domain.Notification:
        // Отправляем уведомление
        sentNotif, err := s.sender.Send(ctx, v)
        if err != nil {
            return domain.NewDomainError(domain.ErrExternalService, "failed to send notification", err)
        }
        
        // Сохраняем ответ
        if sentNotif != nil {
            if err := s.repo.Store(sentNotif); err != nil {
                s.logger.Printf("Failed to store sent notification: %v", err)
            }
        }
        
    case *domain.SentNotification:
        // Если это SentNotification - просто логируем
        s.logger.Printf("Sent notification stored: MessageID=%d, ChatID=%d", v.MessageID, v.ChatID)
        
    default:
        return domain.NewDomainError(domain.ErrValidation, "unsupported entity type", nil)
    }
    
    return nil
}

// workerResult результат обработки уведомления воркером
type workerResult struct {
    Text  string
    Error error
}

// ProcessWithIntervals обрабатывает уведомления с интервалами
// Реализует domain.NotificationService.ProcessWithIntervals
func (s *NotificationService) ProcessWithIntervals(
    ctx context.Context,
    notifications []*domain.Notification,
    interval time.Duration,
    numWorkers int,
) domain.ProcessResult {
    if len(notifications) == 0 {
        return domain.ProcessResult{}
    }
    
    // Ограничиваем количество воркеров
    if numWorkers <= 0 {
        numWorkers = 1
    }
    if numWorkers > 10 {
        numWorkers = 10
    }
    
    jobs := make(chan *domain.Notification, len(notifications))
    results := make(chan *workerResult, len(notifications))
    done := make(chan bool)
    
    var wg sync.WaitGroup
    
    // Запускаем worker'ы
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go s.notificationWorker(ctx, i+1, &wg, jobs, results)
    }
    
    // Отправляем уведомления с интервалами
    go s.sendNotificationsWithIntervals(ctx, notifications, jobs, interval)
    
    // Ждем завершения worker'ов
    go func() {
        wg.Wait()
        close(results)
        done <- true
    }()
    
    // Обрабатываем результаты
    return s.processResults(ctx, results, done)
}

// notificationWorker обрабатывает уведомления из канала jobs
func (s *NotificationService) notificationWorker(
    ctx context.Context,
    workerID int,
    wg *sync.WaitGroup,
    jobs <-chan *domain.Notification,
    results chan<- *workerResult,
) {
    defer wg.Done()
    
    s.logger.Printf("Worker %d started", workerID)
    defer s.logger.Printf("Worker %d finished", workerID)
    
    for {
        select {
        case <-ctx.Done():
            s.logger.Printf("Worker %d received cancellation signal", workerID)
            return
            
        case notification, ok := <-jobs:
            if !ok {
                return
            }
            
            s.logger.Printf("Worker %d processing: %s", workerID, notification.Text)
            
            err := s.ProcessEntity(ctx, notification)
            
            select {
            case <-ctx.Done():
                s.logger.Printf("Worker %d interrupted while sending result", workerID)
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

// sendNotificationsWithIntervals отправляет уведомления с интервалами
func (s *NotificationService) sendNotificationsWithIntervals(
    ctx context.Context,
    notifications []*domain.Notification,
    jobs chan<- *domain.Notification,
    interval time.Duration,
) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    sentCount := 0
    
    for {
        select {
        case <-ctx.Done():
            s.logger.Println("Notification sending interrupted by signal")
            close(jobs)
            return
            
        case <-ticker.C:
            if sentCount >= len(notifications) {
                close(jobs)
                s.logger.Printf("All %d notifications queued", sentCount)
                return
            }
            
            notification := notifications[sentCount]
            
            select {
            case <-ctx.Done():
                close(jobs)
                return
                
            case jobs <- notification:
                sentCount++
                
                if sentCount < len(notifications) {
                    s.logger.Printf("Next notification in %v", interval)
                }
            }
        }
    }
}

// processResults обрабатывает результаты
func (s *NotificationService) processResults(
    ctx context.Context,
    results <-chan *workerResult,
    done <-chan bool,
) domain.ProcessResult {
    successCount := 0
    errorCount := 0
    
    for {
        select {
        case <-ctx.Done():
            s.logger.Println("Results processing interrupted by signal")
            
            // Ждем завершения воркеров
            select {
            case <-done:
                s.logger.Println("All workers completed")
            case <-time.After(2 * time.Second):
                s.logger.Println("Timeout waiting for workers")
            }
            
            return domain.ProcessResult{
                SuccessCount: successCount,
                ErrorCount:   errorCount,
            }
            
        case result, ok := <-results:
            if !ok {
                // Канал закрыт, ждем подтверждения
                <-done
                return domain.ProcessResult{
                    SuccessCount: successCount,
                    ErrorCount:   errorCount,
                }
            }
            
            if result.Error != nil {
                s.logger.Printf("Error processing notification '%s': %v", result.Text, result.Error)
                errorCount++
            } else {
                s.logger.Printf("Notification successfully processed: %s", result.Text)
                successCount++
            }
        }
    }
}

// HealthCheck проверяет здоровье системы
// Реализует domain.NotificationService.HealthCheck
func (s *NotificationService) HealthCheck() error {
    return s.sender.HealthCheck()
}

// GetNotifications возвращает все уведомления
// Реализует domain.NotificationService.GetNotifications
func (s *NotificationService) GetNotifications() []*domain.Notification {
    return s.repo.GetNotifications()
}

// GetSentNotifications возвращает отправленные уведомления
// Реализует domain.NotificationService.GetSentNotifications
func (s *NotificationService) GetSentNotifications() []*domain.SentNotification {
    return s.repo.GetSentNotifications()
}

// GetStats возвращает статистику сервиса
// Реализует domain.NotificationService.GetStats
func (s *NotificationService) GetStats() *domain.ServiceStats {
    notifications := s.repo.GetNotifications()
    sentNotifications := s.repo.GetSentNotifications()
    
    return &domain.ServiceStats{
        TotalNotifications:      len(notifications),
        TotalSentNotifications:  len(sentNotifications),
        PendingNotifications:    len(notifications) - len(sentNotifications),
    }
}