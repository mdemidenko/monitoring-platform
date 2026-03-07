package domain

import "testing"

import "github.com/stretchr/testify/assert"

func TestNewNotification(t *testing.T) {
    notification := NewNotification("123", "Hello")

    assert.Equal(t, "123", notification.ChatID)
    assert.Equal(t, "Hello", notification.Text)
}

// Опционально: если есть логика в будущем
func TestNotification_Validate(t *testing.T) {
    // Пока нет метода Validate — можно пропустить
    // Или добавить, если появится
}