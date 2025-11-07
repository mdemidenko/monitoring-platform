package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"monitoring-platform/config"
	"monitoring-platform/internal/models"

)

type TelegramService struct {
	config *config.Config
	client *http.Client
}

type NotificationResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"description,omitempty"`
	Result *models.SentNotification `json:"result,omitempty"`
}

func NewTelegramService(cfg *config.Config) *TelegramService {
	timeout := time.Duration(cfg.Telegram.Timeout) * time.Second
	
	client := &http.Client{
		Timeout: timeout,
	}

	return &TelegramService{
		config: cfg,
		client: client,
	}
}

// SendNotification отправляет уведомление в Telegram
func (s *TelegramService) SendNotification(text string) (*models.SentNotification, error) {

	notification := models.NewNotification(s.config.Telegram.ChatID, text)

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notification: %w", err)
	}

	if s.config.Telegram.Debug {
		log.Printf("Sending notification: %s", string(jsonData))
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.config.Telegram.BotToken)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
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