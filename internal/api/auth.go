package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mdemidenko/monitoring-platform/internal/middleware"
)

// LoginRequest запрос на аутентификацию
// @Description Запрос для получения JWT токена
type LoginRequest struct {
    // Логин пользователя
    Username string `json:"username" binding:"required,min=1" example:"admin"`
    // Пароль пользователя
    Password string `json:"password" binding:"required,min=1" example:"secure_password"`
}

// LoginResponse ответ с JWT токеном
// @Description Ответ с JWT токеном при успешной аутентификации
type LoginResponse struct {
    // Флаг успешного выполнения
    Success bool `json:"success" example:"true"`
    // JWT токен для авторизации
    Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
    // Время истечения токена
    ExpiresAt time.Time `json:"expires_at" example:"2024-01-01T12:00:00Z"`
    // Тип токена
    TokenType string `json:"token_type" example:"Bearer"`
}

// LoginHandler обработчик для аутентификации
// @Summary Аутентификация пользователя
// @Description Получение JWT токена по логину и паролю
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Данные для аутентификации"
// @Success 200 {object} LoginResponse "Успешная аутентификация"
// @Failure 400 {object} ErrorResponse "Некорректные данные запроса"
// @Failure 401 {object} ErrorResponse "Неверные учетные данные"
// @Router /api/auth/login [post]
func (h *Handler) LoginHandler(c *gin.Context) {
    var req LoginRequest
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "error":   "Invalid request: " + err.Error(),
        })
        return
    }

    // Проверяем учетные данные
    if req.Username != h.cfg.Auth.Login || req.Password != h.cfg.Auth.Password {
        c.JSON(http.StatusUnauthorized, gin.H{
            "success": false,
            "error":   "Invalid username or password",
        })
        return
    }

    // Генерируем JWT токен
    token, err := middleware.GenerateJWTToken(
        req.Username,
        h.cfg.Auth.JWTSecret,
        h.cfg.Auth.JWTExpiration,
    )
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "success": false,
            "error":   "Failed to generate token: " + err.Error(),
        })
        return
    }

    expirationTime := time.Now().Add(time.Duration(h.cfg.Auth.JWTExpiration) * time.Hour)
    
    c.JSON(http.StatusOK, LoginResponse{
        Success:    true,
        Token:      token,
        ExpiresAt:  expirationTime,
        TokenType:  "Bearer",
    })
}