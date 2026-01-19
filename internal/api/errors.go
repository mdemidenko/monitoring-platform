package api

import (
	"net/http"

)

// ErrorResponse представляет ответ об ошибке
// @Description Стандартный ответ при возникновении ошибки
// @name ErrorResponse
type ErrorResponse struct {
	// Флаг успешного выполнения (всегда false)
	Success bool `json:"success" example:"false"`
	// HTTP статус код
	StatusCode int `json:"status_code" example:"400"`
	// Тип ошибки
	ErrorType string `json:"error_type" example:"Bad Request"`
	// Описание ошибки
	Message string `json:"message" example:"Invalid request parameters"`
	// Дополнительная информация об ошибке (опционально)
	Details interface{} `json:"details,omitempty"`
}

// Helper функции для создания ошибок
func newErrorResponse(statusCode int, errorType, message string, details interface{}) ErrorResponse {
	return ErrorResponse{
		Success:    false,
		StatusCode: statusCode,
		ErrorType:  errorType,
		Message:    message,
		Details:    details,
	}
}

// BadRequestError создает ошибку 400
func BadRequestError(message string, details ...interface{}) ErrorResponse {
	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}
	return newErrorResponse(http.StatusBadRequest, "Bad Request", message, detail)
}

// UnauthorizedError создает ошибку 401
func UnauthorizedError(message string, details ...interface{}) ErrorResponse {
	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}
	return newErrorResponse(http.StatusUnauthorized, "Unauthorized", message, detail)
}

// InternalServerError создает ошибку 500
func InternalServerError(message string, details ...interface{}) ErrorResponse {
	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}
	return newErrorResponse(http.StatusInternalServerError, "Internal Server Error", message, detail)
}

// BadGatewayError создает ошибку 502
func BadGatewayError(message string, details ...interface{}) ErrorResponse {
	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}
	return newErrorResponse(http.StatusBadGateway, "Bad Gateway", message, detail)
}

// ServiceUnavailableError создает ошибку 503
func ServiceUnavailableError(message string, details ...interface{}) ErrorResponse {
	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}
	return newErrorResponse(http.StatusServiceUnavailable, "Service Unavailable", message, detail)
}