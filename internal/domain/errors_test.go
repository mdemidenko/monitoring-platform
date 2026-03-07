package domain

import (
    "errors"
    "testing"

    "github.com/stretchr/testify/assert"
)

// === Тесты для DomainError ===

func TestDomainError_Error_WithoutCause(t *testing.T) {
    err := NewDomainError("TEST", "test error", nil)
    assert.Equal(t, "TEST: test error", err.Error())
}

func TestDomainError_Error_WithCause(t *testing.T) {
    cause := errors.New("original")
    err := NewDomainError("TEST", "test error", cause)
    assert.Contains(t, err.Error(), "TEST: test error - original")
}

func TestDomainError_Unwrap(t *testing.T) {
    cause := errors.New("original")
    err := NewDomainError("TEST", "test error", cause)
    assert.Equal(t, cause, err.Unwrap())
}

func TestDomainError_Unwrap_Nil(t *testing.T) {
    err := NewDomainError("TEST", "test error", nil)
    assert.Nil(t, err.Unwrap())
}

// === Тесты для ValidateNotification ===

func TestValidateNotification_Nil(t *testing.T) {
    err := ValidateNotification(nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "notification cannot be nil")
    assert.Equal(t, ErrInvalidInput, err.(*DomainError).Code)
}

func TestValidateNotification_EmptyText(t *testing.T) {
    notification := &Notification{ChatID: "123", Text: ""}
    err := ValidateNotification(notification)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "text cannot be empty")
    assert.Equal(t, ErrValidation, err.(*DomainError).Code)
}

func TestValidateNotification_EmptyChatID(t *testing.T) {
    notification := &Notification{ChatID: "", Text: "Hello"}
    err := ValidateNotification(notification)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "chat_id cannot be empty")
    assert.Equal(t, ErrValidation, err.(*DomainError).Code)
}

func TestValidateNotification_Valid(t *testing.T) {
    notification := &Notification{ChatID: "123", Text: "Hello"}
    err := ValidateNotification(notification)
    assert.NoError(t, err)
}