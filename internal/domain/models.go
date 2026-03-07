package domain

import "time"

// Notification - доменная модель уведомления
type Notification struct {
    ChatID string `json:"chat_id"`
    Text   string `json:"text"`
}

// SentNotification - доменная модель отправленного уведомления
type SentNotification struct {
    MessageID int64     `json:"message_id"`
    ChatID    int64     `json:"chat_id"`
    SentAt    time.Time `json:"sent_at"`
}

// ProcessResult - результат пакетной обработки
type ProcessResult struct {
    SuccessCount int
    ErrorCount   int
}

// ServiceStats статистика сервиса
type ServiceStats struct {
    TotalNotifications      int `json:"total_notifications"`
    TotalSentNotifications  int `json:"total_sent_notifications"`
    PendingNotifications    int `json:"pending_notifications"`
}

// NewNotification создает новое уведомление
func NewNotification(chatID, text string) *Notification {
    return &Notification{
        ChatID: chatID,
        Text:   text,
    }
}