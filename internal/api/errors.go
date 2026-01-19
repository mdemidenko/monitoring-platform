package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse универсальный ответ на ошибку
// @Description Стандартный ответ при возникновении ошибки
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

// Предопределенные типы ошибок
const (
	ErrTypeBadRequest     = "Bad Request"
	ErrTypeUnauthorized   = "Unauthorized"
	ErrTypeForbidden      = "Forbidden"
	ErrTypeNotFound       = "Not Found"
	ErrTypeConflict       = "Conflict"
	ErrTypeInternal       = "Internal Server Error"
	ErrTypeBadGateway     = "Bad Gateway"
	ErrTypeServiceUnavailable = "Service Unavailable"
	ErrTypeValidation     = "Validation Error"
	ErrTypeRateLimit      = "Rate Limit Exceeded"
)

// NewErrorResponse создает новый ErrorResponse
func NewErrorResponse(statusCode int, errorType, message string, details ...interface{}) ErrorResponse {
	errResp := ErrorResponse{
		Success:    false,
		StatusCode: statusCode,
		ErrorType:  errorType,
		Message:    message,
	}

	if len(details) > 0 {
		errResp.Details = details[0]
	}

	return errResp
}

// ErrorResponse helpers для разных статусов
func BadRequestError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusBadRequest, ErrTypeBadRequest, message, details...)
}

func UnauthorizedError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusUnauthorized, ErrTypeUnauthorized, message, details...)
}

func ForbiddenError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusForbidden, ErrTypeForbidden, message, details...)
}

func NotFoundError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusNotFound, ErrTypeNotFound, message, details...)
}

func InternalServerError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusInternalServerError, ErrTypeInternal, message, details...)
}

func BadGatewayError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusBadGateway, ErrTypeBadGateway, message, details...)
}

func ServiceUnavailableError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusServiceUnavailable, ErrTypeServiceUnavailable, message, details...)
}

func ValidationError(message string, details ...interface{}) ErrorResponse {
	return NewErrorResponse(http.StatusUnprocessableEntity, ErrTypeValidation, message, details...)
}

// ErrorResponse middleware для стандартной обработки ошибок
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		
		// Проверяем, есть ли ошибки
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			
			var errorResp ErrorResponse
			
			// Преобразуем разные типы ошибок
			switch e := err.Err.(type) {
			case *gin.Error:
				// Ошибка Gin
				errorResp = InternalServerError("Internal server error", e.Error())
			default:
				// Общая ошибка
				errorResp = InternalServerError("Internal server error", e.Error())
			}
			
			c.JSON(errorResp.StatusCode, errorResp)
			c.Abort()
		}
	}
}