package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

type TelegramService struct {
	config  *config.Config
	client  *http.Client
	storage repository.Storage
}

type NotificationResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"description,omitempty"`
	Result *models.SentNotification `json:"result,omitempty"`
}

// ProcessResult результат обработки всех уведомлений
type ProcessResult struct {
	SuccessCount int
	ErrorCount   int
}

// workerResult результат обработки уведомления воркером
type workerResult struct {
	Text  string
	Error error
}

func NewTelegramService(cfg *config.Config, storage repository.Storage) *TelegramService {
	timeout := time.Duration(cfg.Telegram.Timeout) * time.Second
	
	client := &http.Client{
		Timeout: timeout,
	}

	return &TelegramService{
		config:  cfg,
		client:  client,
		storage: storage,
	}
}

// ProcessWithIntervals обрабатывает уведомления с интервалами между отправками
func (s *TelegramService) ProcessWithIntervals(ctx context.Context, notifications []*models.Notification, interval time.Duration, numWorkers int) ProcessResult {
	jobs := make(chan *models.Notification, len(notifications))
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

// sendNotificationsWithIntervals отправляет уведомления с интервалами
func (s *TelegramService) sendNotificationsWithIntervals(ctx context.Context, notifications []*models.Notification, jobs chan<- *models.Notification, interval time.Duration) {
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
				if sentCount < len(notifications) {
					log.Printf("⏰ Следующее уведомление через %v", interval)
				}
			}
		}
	}
}

// sendAllNotifications отправляет все уведомления сразу
func (s *TelegramService) sendAllNotifications(ctx context.Context, notifications []*models.Notification, jobs chan<- *models.Notification) {
	for i, notification := range notifications {
		select {
		case <-ctx.Done():
			log.Println("⏹️  Прерывание отправки уведомлений по сигналу")
			close(jobs)
			return
		default:
			log.Printf("📨 Постановка в очередь уведомления %d: %s", i+1, notification.Text)
			jobs <- notification
		}
	}
	close(jobs)
}

// notificationWorker обрабатывает уведомления из канала jobs
func (s *TelegramService) notificationWorker(ctx context.Context, workerID int, wg *sync.WaitGroup, jobs <-chan *models.Notification, results chan<- *workerResult) {
	defer wg.Done()

	log.Printf("Worker %d запущен", workerID)
	defer log.Printf("👷 Worker %d завершил работу", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d получил сигнал завершения", workerID)
			return
		case notification, ok := <-jobs:
			if !ok {
				return
			}

			log.Printf("Worker %d обрабатывает: %s", workerID, notification.Text)

			err := s.ProcessEntity(ctx, notification)

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

// processResults обрабатывает результаты из канала results
func (s *TelegramService) processResults(ctx context.Context, results <-chan *workerResult, done <-chan bool) ProcessResult {
	successCount := 0
	errorCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Println("⏹️  Прерывание обработки результатов по сигналу")
			select {
			case <-done:
				log.Println("✅ Все воркеры завершили работу")
			case <-time.After(2 * time.Second):
				log.Println("⚠️  Таймаут ожидания завершения воркеров")
			}
			return ProcessResult{SuccessCount: successCount, ErrorCount: errorCount}
		case result, ok := <-results:
			if !ok {
				<-done
				return ProcessResult{SuccessCount: successCount, ErrorCount: errorCount}
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

// ProcessEntity обрабатывает сущности и сохраняет их в репозиторий
func (s *TelegramService) ProcessEntity(ctx context.Context, entity any) error {
	// Проверяем контекст перед началом работы
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}
	
	// Сохраняем входящую сущность (происходит проверка типа)
	if err := s.storage.Store(entity); err != nil {
		return fmt.Errorf("failed to store entity: %w", err)
	}
	
	// Дополнительная логика в зависимости от типа
	switch v := entity.(type) {
	case *models.Notification:
		// Отправляем уведомление и получаем ответ от Telegram
		sentNotif, err := s.SendNotification(ctx, v.Text)
		if err != nil {
			return err
		}
		
		// Сохраняем ответ от Telegram (SentNotification)
		if sentNotif != nil {
			if err := s.storage.Store(sentNotif); err != nil {
				log.Printf("Failed to store sent notification: %v", err)
			}
		}
	case *models.SentNotification:
		// Если это SentNotification - просто логируем
		log.Printf("Sent notification stored: MessageID=%d, ChatID=%d", v.MessageID, v.ChatID)
	}
	
	return nil
}

// SendNotification отправляет уведомление в Telegram
func (s *TelegramService) SendNotification(ctx context.Context, text string) (*models.SentNotification, error) {
	// Проверяем контекст перед началом
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("operation cancelled: %w", err)
	}

	notification := models.NewNotification(s.config.Telegram.ChatID, text)

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notification: %w", err)
	}

	if s.config.Telegram.Debug {
		log.Printf("Sending notification: %s", string(jsonData))
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.config.Telegram.BotToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if s.config.Telegram.Debug {
		log.Printf("Response: %s", string(body))
	}

	var telegramResp NotificationResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !telegramResp.OK {
		return nil, fmt.Errorf("telegram API error: %s", telegramResp.Error)
	}

	return telegramResp.Result, nil
}

// HealthCheck проверяет доступность бота
func (s *TelegramService) HealthCheck() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", s.config.Telegram.BotToken)
	resp, err := s.client.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}