package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mdemidenko/monitoring-platform/internal/middleware"
)

// LoginHandler обработчик для аутентификации
// @Summary Аутентификация пользователя
// @Description Получение JWT токена по логину и паролю
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Данные для аутентификации"
// @Success 200 {object} LoginResponse "Успешная аутентификация"
// @Failure 400 {object} api.ErrorResponse "Некорректные данные запроса"
// @Failure 401 {object} api.ErrorResponse "Неверные учетные данные"
// @Failure 500 {object} api.ErrorResponse "Ошибка генерации токена"
// @Router /api/auth/login [post]
func (h *Handler) LoginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=1"`
		Password string `json:"password" binding:"required,min=1"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BadRequestError(
			"Invalid request format",
			gin.H{"validation_error": err.Error()},
		))
		return
	}

	// Проверяем учетные данные
	if req.Username != h.cfg.Auth.Login || req.Password != h.cfg.Auth.Password {
		c.JSON(http.StatusUnauthorized, UnauthorizedError(
			"Invalid username or password",
			gin.H{"hint": "Check your credentials or contact administrator"},
		))
		return
	}

	// Генерируем JWT токен
	token, err := middleware.GenerateJWTToken(
		req.Username,
		h.cfg.Auth.JWTSecret,
		h.cfg.Auth.JWTExpiration,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, InternalServerError(
			"Failed to generate authentication token",
			gin.H{"jwt_error": err.Error()},
		))
		return
	}

	expirationTime := time.Now().Add(time.Duration(h.cfg.Auth.JWTExpiration) * time.Hour)
	
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"token":      token,
		"expires_at": expirationTime,
		"token_type": "Bearer",
	})
}