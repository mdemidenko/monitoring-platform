package adapters

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
    "github.com/mdemidenko/monitoring-platform/internal/domain"
)

// TelegramAdapter реализует domain.NotificationSender для Telegram API
type TelegramAdapter struct {
    config  *config.TelegramConfig
    client  *http.Client
    logger  *log.Logger
}

// NotificationResponse ответ от Telegram API
type NotificationResponse struct {
    OK     bool                     `json:"ok"`
    Error  string                   `json:"description,omitempty"`
    Result *domain.SentNotification `json:"result,omitempty"`
}

// NewTelegramAdapter создает новый адаптер для Telegram
func NewTelegramAdapter(cfg *config.TelegramConfig, logger *log.Logger) *TelegramAdapter {
    timeout := time.Duration(cfg.Timeout) * time.Second
    
    client := &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            MaxIdleConns:        10,
            IdleConnTimeout:     30 * time.Second,
            TLSHandshakeTimeout: 10 * time.Second,
        },
    }
    
    if logger == nil {
        logger = log.Default()
    }
    
    return &TelegramAdapter{
        config: cfg,
        client: client,
        logger: logger,
    }
}

// Send отправляет уведомление через Telegram API
func (a *TelegramAdapter) Send(ctx context.Context, notification *domain.Notification) (*domain.SentNotification, error) {
    // Используем chat_id из уведомления или дефолтный из конфига
    chatID := notification.ChatID
    if chatID == "" {
        chatID = a.config.ChatID
    }
    
    // Валидация
    if chatID == "" {
        return nil, domain.NewDomainError(
            domain.ErrValidation,
            "chat_id is required",
            nil,
        )
    }
    
    if notification.Text == "" {
        return nil, domain.NewDomainError(
            domain.ErrValidation,
            "text is required",
            nil,
        )
    }
    
    // Подготавливаем запрос к Telegram API
    requestBody := map[string]interface{}{
        "chat_id": chatID,
        "text":    notification.Text,
    }
    
    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return nil, domain.NewDomainError(
            domain.ErrExternalService,
            "failed to marshal request",
            err,
        )
    }
    
    if a.config.Debug {
        a.logger.Printf("Sending to Telegram: %s", string(jsonData))
    }
    
    // Формируем URL
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.config.BotToken)
    
    // Создаем запрос с контекстом
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, domain.NewDomainError(
            domain.ErrExternalService,
            "failed to create request",
            err,
        )
    }
    req.Header.Set("Content-Type", "application/json")
    
    // Выполняем запрос
    resp, err := a.client.Do(req)
    if err != nil {
        return nil, domain.NewDomainError(
            domain.ErrExternalService,
            "failed to send request to Telegram API",
            err,
        )
    }
    defer func() {
    if err := resp.Body.Close(); err != nil {
        log.Printf("Failed to close response body: %v", err)
    }
    }()
    
    // Читаем ответ
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, domain.NewDomainError(
            domain.ErrExternalService,
            "failed to read response from Telegram API",
            err,
        )
    }
    
    if a.config.Debug {
        a.logger.Printf("Telegram response: %s", string(body))
    }
    
    // Парсим ответ
    var telegramResp NotificationResponse
    if err := json.Unmarshal(body, &telegramResp); err != nil {
        return nil, domain.NewDomainError(
            domain.ErrExternalService,
            "failed to parse response from Telegram API",
            err,
        )
    }
    
    if !telegramResp.OK {
        return nil, domain.NewDomainError(
            domain.ErrExternalService,
            fmt.Sprintf("Telegram API error: %s", telegramResp.Error),
            nil,
        )
    }
    
    // Добавляем временную метку, если результат есть
    if telegramResp.Result != nil {
        telegramResp.Result.SentAt = time.Now()
    }
    
    return telegramResp.Result, nil
}

// HealthCheck проверяет доступность Telegram API
func (a *TelegramAdapter) HealthCheck() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", a.config.BotToken)
	
	resp, err := a.client.Get(url)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrExternalService,
			"Telegram API unavailable",
			err,
		)
	}
	defer func() {
    if err := resp.Body.Close(); err != nil {
        log.Printf("Failed to close response body: %v", err)
        }
    }()
	
	// Проверяем статус код ДО чтения тела
	if resp.StatusCode != http.StatusOK {
		// Пытаемся прочитать тело для получения деталей ошибки
		body, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("Telegram API returned status: %d", resp.StatusCode)
		
		// Если в теле есть JSON с описанием ошибки
		if len(body) > 0 {
			var telegramResp struct {
				OK     bool   `json:"ok"`
				Error  string `json:"description,omitempty"`
			}
			if json.Unmarshal(body, &telegramResp) == nil && telegramResp.Error != "" {
				errorMsg = fmt.Sprintf("Telegram API returned status: %d - %s", resp.StatusCode, telegramResp.Error)
			}
		}
		
		return domain.NewDomainError(
			domain.ErrExternalService,
			errorMsg,
			nil,
		)
	}
	
	// Читаем тело ответа для статуса 200 OK
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrExternalService,
			"failed to read response from Telegram API",
			err,
		)
	}
	
	if a.config.Debug {
		a.logger.Printf("Telegram health check response: %s", string(body))
	}
	
	// Проверяем что тело не пустое
	if len(body) == 0 {
		return domain.NewDomainError(
			domain.ErrExternalService,
			"Telegram API returned empty response",
			nil,
		)
	}
	
	// Парсим JSON
	var telegramResp struct {
		OK     bool   `json:"ok"`
		Error  string `json:"description,omitempty"`
	}
	
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return domain.NewDomainError(
			domain.ErrExternalService,
			"failed to parse response from Telegram API",
			err,
		)
	}
	
	if !telegramResp.OK {
		errorMsg := "Telegram API check failed"
		if telegramResp.Error != "" {
			errorMsg = fmt.Sprintf("Telegram API error: %s", telegramResp.Error)
		}
		return domain.NewDomainError(
			domain.ErrExternalService,
			errorMsg,
			nil,
		)
	}
	
	return nil
}