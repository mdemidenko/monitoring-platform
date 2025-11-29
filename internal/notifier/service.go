package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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