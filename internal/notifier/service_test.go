package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/h2non/gock"
	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStorage — мок для repository.Storage
type MockStorage struct {
    mock.Mock
}

func (m *MockStorage) Store(entity interface{}) error {
    args := m.Called(entity)
    return args.Error(0)
}

func (m *MockStorage) GetNotifications() []*models.Notification {
    args := m.Called()
    return args.Get(0).([]*models.Notification)
}

func (m *MockStorage) GetSentNotifications() []*models.SentNotification {
    args := m.Called()
    return args.Get(0).([]*models.SentNotification)
}

func TestSendNotification_Success(t *testing.T) {
    defer gock.Off() // отключаем моки после теста

    // Мокируем POST https://api.telegram.org/bottest-token/sendMessage
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        JSON(map[string]interface{}{
            "ok": true,
            "result": map[string]interface{}{
                "message_id": 123,
                "chat":       map[string]interface{}{"id": 12345},
            },
        })

    // Конфиг
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
            Timeout:  1,
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    // Вызов
    ctx := context.Background()
    sent, err := svc.SendNotification(ctx, "Hello")

    // Проверка
    assert.NoError(t, err)
    assert.NotNil(t, sent)
    assert.Equal(t, int64(123), sent.MessageID)

    // Проверяем, что мок был вызван
    assert.True(t, gock.IsDone(), "Expected HTTP request to be made")
}

func TestSendNotification_Failure(t *testing.T) {
    defer gock.Off()

    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        JSON(map[string]interface{}{
            "ok":          false,
            "description": "Not Found",
        })

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    ctx := context.Background()
    sent, err := svc.SendNotification(ctx, "Hello")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "telegram API error")
    assert.Nil(t, sent)
    assert.True(t, gock.IsDone())
}

func TestProcessEntity_ProcessNotification(t *testing.T) {
    defer gock.Off()

    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        JSON(map[string]interface{}{
            "ok": true,
            "result": map[string]interface{}{
                "message_id": 999,
                "chat":       map[string]interface{}{"id": 12345},
            },
        })

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    notification := &models.Notification{ChatID: "12345", Text: "Test message"}

    storage.On("Store", mock.MatchedBy(func(n *models.Notification) bool {
        return n.Text == "Test message"
    })).Return(nil).Once()

    storage.On("Store", mock.MatchedBy(func(s *models.SentNotification) bool {
        return s.MessageID == 999
    })).Return(nil).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    assert.NoError(t, err)
    storage.AssertExpectations(t)
    assert.True(t, gock.IsDone())
}

func TestProcessWithIntervals_Success(t *testing.T) {
    defer gock.Off()

    // Мокируем два вызова
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Times(2).
        Reply(200).
        JSON(map[string]interface{}{
            "ok": true,
            "result": map[string]interface{}{
                "message_id": 1,
                "chat":       map[string]interface{}{"id": 12345},
            },
        })

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    notifications := []*models.Notification{
        {ChatID: "12345", Text: "Msg1"},
        {ChatID: "12345", Text: "Msg2"},
    }

    storage.On("Store", mock.Anything).Return(nil).Times(4) // 2×Notification + 2×SentNotification

    result := svc.ProcessWithIntervals(context.Background(), notifications, 1*time.Millisecond, 2)

    assert.Equal(t, 2, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)
    storage.AssertExpectations(t)
    assert.True(t, gock.IsDone(), "Expected 2 HTTP requests")
}

func TestProcessWithIntervals_ContextCancel(t *testing.T) {
    // Не мокаем HTTP — хотим, чтобы был timeout или ошибка
    defer gock.Off()

    // Блокируем все запросы к Telegram
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        ReplyError(context.Canceled)

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    notifications := []*models.Notification{
        {ChatID: "12345", Text: "Msg1"},
    }

    result := svc.ProcessWithIntervals(ctx, notifications, 10*time.Millisecond, 1)

    assert.Equal(t, 0, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)
}

// TestHealthCheck_Success проверяет успешный health check
func TestHealthCheck_Success(t *testing.T) {
    defer gock.Off()

    // Мокируем GET https://api.telegram.org/bottest-token/getMe
    gock.New("https://api.telegram.org").
        Get("/bottest-token/getMe").
        Reply(200).
        JSON(map[string]interface{}{
            "ok": true,
            "result": map[string]interface{}{
                "id":        123456,
                "is_bot":    true,
                "first_name": "TestBot",
            },
        })

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    err := svc.HealthCheck()

    assert.NoError(t, err)
    assert.True(t, gock.IsDone(), "Expected GET /getMe to be called")
}

// TestHealthCheck_Failure_Network проверяет ошибку сети
func TestHealthCheck_Failure_Network(t *testing.T) {
    defer gock.Off()

    // Мокируем ошибку сети
    gock.New("https://api.telegram.org").
        Get("/bottest-token/getMe").
        ReplyError(fmt.Errorf("connection refused"))

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    err := svc.HealthCheck()

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "health check failed")
    assert.Contains(t, err.Error(), "connection refused")
    assert.True(t, gock.IsDone())
}

// TestHealthCheck_Failure_Status проверяет статус ≠ 200
func TestHealthCheck_Failure_Status(t *testing.T) {
    defer gock.Off()

    // Мокируем 500 Internal Server Error
    gock.New("https://api.telegram.org").
        Get("/bottest-token/getMe").
        Reply(500).
        BodyString("Internal Server Error")

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    err := svc.HealthCheck()

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "health check failed with status: 500")
    assert.True(t, gock.IsDone())
}

func TestProcessEntity_StoreEntity_Fails(t *testing.T) {
    defer gock.Off()

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    notification := &models.Notification{ChatID: "12345", Text: "Msg"}

    // Мок: Store(entity) возвращает ошибку
    storage.On("Store", notification).Return(fmt.Errorf("db error")).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to store entity")
    storage.AssertExpectations(t)
}

func TestProcessEntity_StoreSentNotification_Fails(t *testing.T) {
    defer gock.Off()

    // Мок: успешный ответ от Telegram
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        JSON(map[string]interface{}{
            "ok": true,
            "result": map[string]interface{}{
                "message_id": 1,
                "chat":       map[string]interface{}{"id": 12345},
            },
        })

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    notification := &models.Notification{ChatID: "12345", Text: "Msg"}

    // Мок: Store(notification) — OK
    storage.On("Store", notification).Return(nil).Once()

    // Мок: Store(sentNotif) — ошибка
    storage.On("Store", mock.MatchedBy(func(s *models.SentNotification) bool {
        return s.MessageID == 1
    })).Return(fmt.Errorf("db error")).Once()

    // Выполняем
    err := svc.ProcessEntity(context.Background(), notification)

    // Ошибки нет — только лог
    assert.NoError(t, err)
    storage.AssertExpectations(t)
    assert.True(t, gock.IsDone())
}

func TestProcessEntity_SentNotification_Logs(t *testing.T) {
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            ChatID: "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    sent := &models.SentNotification{MessageID: 123, ChatID: 12345}

    // Мок: Store — OK
    storage.On("Store", sent).Return(nil).Once()

    err := svc.ProcessEntity(context.Background(), sent)

    assert.NoError(t, err)
    storage.AssertExpectations(t)
    // Логирование не проверяем, но код выполнен
}

func TestProcessEntity_SendNotification_Fails(t *testing.T) {
    defer gock.Off()

    // Мок: ошибка от Telegram
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        JSON(map[string]interface{}{
            "ok":          false,
            "description": "Invalid token",
        })

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    notification := &models.Notification{ChatID: "12345", Text: "Msg"}

    // Store(notification) — OK
    storage.On("Store", notification).Return(nil).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "telegram API error")
    storage.AssertExpectations(t)
    assert.True(t, gock.IsDone())
}

func TestSendNotification_NewRequestError(t *testing.T) {
    // Перехватим через gock, но с невалидным URL
    defer gock.Off()

    // Невалидный BotToken, который сломает URL
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    // Подменяем client, чтобы NewRequestWithContext вернул ошибку
    // Например, если URL невалидный
    svc.config.Telegram.BotToken = string([]byte{0x7f}) // invalid byte in URL

    sent, err := svc.SendNotification(context.Background(), "Hello")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to create request")
    assert.Nil(t, sent)
}

func TestSendNotification_UnmarshalError(t *testing.T) {
    defer gock.Off()

    // Мок: возвращаем не-JSON
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        BodyString("This is not JSON")

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    sent, err := svc.SendNotification(context.Background(), "Hello")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to unmarshal response")
    assert.Nil(t, sent)
    assert.True(t, gock.IsDone())
}

func TestSendNotification_DebugLogging(t *testing.T) {
    defer gock.Off()

    // Включаем Debug
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
            Debug:    true,
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    // Мок: успешный ответ
    gock.New("https://api.telegram.org").
        Post("/bottest-token/sendMessage").
        Reply(200).
        JSON(map[string]interface{}{
            "ok": true,
            "result": map[string]interface{}{
                "message_id": 1,
                "chat":       map[string]interface{}{"id": 12345},
            },
        })


    sent, err := svc.SendNotification(context.Background(), "Hello")

    assert.NoError(t, err)
    assert.NotNil(t, sent)
    assert.True(t, gock.IsDone())
}

func TestSendNotification_ReadBodyError(t *testing.T) {
    // Явно отключаем gock
    defer gock.Off()

    // Создаём HTTPS-сервер
    server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Убедимся, что это наш запрос
        if r.URL.Path != "/bottest-token/sendMessage" {
            t.Errorf("Expected /bottest-token/sendMessage, got %s", r.URL.Path)
            http.Error(w, "Not Found", 404)
            return
        }

        // Начинаем ответ
        w.WriteHeader(200)
        w.(http.Flusher).Flush()

        // Немедленно разрываем соединение
        conn, bufrw, _ := w.(http.Hijacker).Hijack()
        defer func() {
            if err := conn.Close(); err != nil {
                t.Logf("Failed to close connection: %v", err)
            }
        }()

        // Отправляем часть тела
        _, err := bufrw.Write([]byte("HTTP/1.1 200 OK\r\n"))
        require.NoError(t, err, "Failed to write response")
        err = bufrw.Flush()
        require.NoError(t, err, "Failed to flush response")

        // Закрываем соединение → следующее чтение вызовет ошибку
       if err := conn.Close(); err != nil {
        t.Logf("Failed to close connection: %v", err)
       }
    }))
    defer server.Close()

    // Клиент с перехватом вызова к api.telegram.org:443
    transport := &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            if addr == "api.telegram.org:443" {
                return net.Dial("tcp", server.Listener.Addr().String())
            }
            return (&net.Dialer{}).DialContext(ctx, network, addr)
        },
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true,
            ServerName:         "api.telegram.org",
        },
    }

    client := &http.Client{
        Transport: transport,
        Timeout:   3 * time.Second,
    }

    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            BotToken: "test-token",
            ChatID:   "12345",
        },
    }
    storage := new(MockStorage)
    svc := &TelegramService{
        config:  cfg,
        client:  client,
        storage: storage,
    }

    sent, err := svc.SendNotification(context.Background(), "Hello")

    // Ожидаем ошибку при чтении тела
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to read response")
    assert.Nil(t, sent)
}

func TestProcessResults_ChannelClosed(t *testing.T) {
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            ChatID: "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    results := make(chan *workerResult)
    done := make(chan bool)

    // Закрываем каналы
    close(results)
    close(done)

    result := svc.processResults(context.Background(), results, done)

    assert.Equal(t, 0, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)
}

func TestProcessResults_ContextDone_Timeout(t *testing.T) {
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            ChatID: "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    results := make(chan *workerResult, 1)
    done := make(chan bool) // не закрываем → сработает таймаут

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // отменяем сразу

    // Запускаем processResults
    result := svc.processResults(ctx, results, done)

    // Должно вернуть 0,0 без паники
    assert.Equal(t, 0, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)
}

func TestProcessResults_WithError(t *testing.T) {
    cfg := &config.Config{
        Telegram: config.TelegramConfig{
            ChatID: "12345",
        },
    }
    storage := new(MockStorage)
    svc := NewTelegramService(cfg, storage)

    results := make(chan *workerResult, 1)
    done := make(chan bool)

    // Отправляем ошибку
    results <- &workerResult{
        Text:  "Test error",
        Error: fmt.Errorf("test error"),
    }
    close(results)
    close(done)

    result := svc.processResults(context.Background(), results, done)

    assert.Equal(t, 0, result.SuccessCount)
    assert.Equal(t, 1, result.ErrorCount)
}