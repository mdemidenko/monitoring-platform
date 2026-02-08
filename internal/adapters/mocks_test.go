package adapters

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/domain"
)

// MockRoundTripper - мок для http.RoundTripper
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"ok": true}`)),
		Header:     make(http.Header),
	}, nil
}

// newTestTelegramConfig создает тестовую конфигурацию
func newTestTelegramConfig() *config.TelegramConfig {
	return &config.TelegramConfig{
		BotToken: "test-bot-token",
		ChatID:   "test-chat-id",
		Timeout:  5,
		Debug:    false,
	}
}

// newTestNotification создает тестовое уведомление
func newTestNotification() *domain.Notification {
	return &domain.Notification{
		ChatID: "123",
		Text:   "Test message",
	}
}

// newTelegramAdapterWithMock создает TelegramAdapter с моком транспорта
func newTelegramAdapterWithMock(cfg *config.TelegramConfig, roundTripFunc func(*http.Request) (*http.Response, error)) *TelegramAdapter {
	mockTransport := &MockRoundTripper{
		RoundTripFunc: roundTripFunc,
	}

	client := &http.Client{
		Transport: mockTransport,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}

	return &TelegramAdapter{
		config: cfg,
		client: client,
		logger: nil,
	}
}