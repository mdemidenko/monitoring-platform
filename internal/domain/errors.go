package domain

import "fmt"

// DomainError - ошибка доменного слоя
type DomainError struct {
    Code    string
    Message string
    Err     error
}

func (e *DomainError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
    return e.Err
}

// Валидные коды ошибок доменного слоя
const (
    ErrValidation      = "VALIDATION_ERROR"
    ErrNotFound        = "NOT_FOUND"
    ErrAlreadyExists   = "ALREADY_EXISTS"
    ErrExternalService = "EXTERNAL_SERVICE_ERROR"
    ErrRepository      = "REPOSITORY_ERROR"
    ErrInvalidInput    = "INVALID_INPUT"
)

// NewDomainError создает новую доменную ошибку
func NewDomainError(code, message string, err error) *DomainError {
    return &DomainError{
        Code:    code,
        Message: message,
        Err:     err,
    }
}

// ValidateNotification проверяет валидность уведомления
func ValidateNotification(notification *Notification) error {
    if notification == nil {
        return NewDomainError(ErrInvalidInput, "notification cannot be nil", nil)
    }
    if notification.Text == "" {
        return NewDomainError(ErrValidation, "text cannot be empty", nil)
    }
    if notification.ChatID == "" {
        return NewDomainError(ErrValidation, "chat_id cannot be empty", nil)
    }
    return nil
}