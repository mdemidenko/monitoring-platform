package core

import "github.com/mdemidenko/monitoring-platform/internal/domain"

// CoreError обертка для ошибок ядра с дополнительным контекстом
type CoreError struct {
    domainErr *domain.DomainError
    Operation string
}

func (e *CoreError) Error() string {
    return e.domainErr.Error()
}

func (e *CoreError) Unwrap() error {
    return e.domainErr
}

// NewCoreError создает новую ошибку ядра
func NewCoreError(operation string, domainErr *domain.DomainError) *CoreError {
    return &CoreError{
        Operation: operation,
        domainErr: domainErr,
    }
}

// IsValidationError проверяет, является ли ошибка ошибкой валидации
func IsValidationError(err error) bool {
    if domainErr, ok := err.(*domain.DomainError); ok {
        return domainErr.Code == domain.ErrValidation || domainErr.Code == domain.ErrInvalidInput
    }
    if coreErr, ok := err.(*CoreError); ok {
        return coreErr.domainErr.Code == domain.ErrValidation || coreErr.domainErr.Code == domain.ErrInvalidInput
    }
    return false
}

// IsExternalServiceError проверяет, является ли ошибка ошибкой внешнего сервиса
func IsExternalServiceError(err error) bool {
    if domainErr, ok := err.(*domain.DomainError); ok {
        return domainErr.Code == domain.ErrExternalService
    }
    if coreErr, ok := err.(*CoreError); ok {
        return coreErr.domainErr.Code == domain.ErrExternalService
    }
    return false
}