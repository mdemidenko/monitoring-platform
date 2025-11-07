package models

// Notification модель для отправки уведомления
type Notification struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// NotificationResponse модель ответа от Telegram API
// type NotificationResponse struct {
// 	OK     bool   `json:"ok"`
// 	Error  string `json:"description,omitempty"`
// 	Result *SentNotification `json:"result,omitempty"`
// }

// SentNotification модель отправленного уведомления
type SentNotification struct {
	MessageID int64 `json:"message_id"`
	ChatID    int64 `json:"chat_id"`
}

// NewNotification создает новое уведомление
func NewNotification(chatID, text string) *Notification {
	return &Notification{
		ChatID: chatID,
		Text:   text,
	}
}