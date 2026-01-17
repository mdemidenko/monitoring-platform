package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/notifier"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)

type Handler struct {
	telegramService *notifier.TelegramService
	storage         *repository.MemoryStorage
	cfg             *config.Config
}

func NewHandler(telegramService *notifier.TelegramService, storage *repository.MemoryStorage, cfg *config.Config) *Handler {
	return &Handler{
		telegramService: telegramService,
		storage:         storage,
		cfg:             cfg,
	}
}

// HealthHandler проверяет здоровье сервиса
func (h *Handler) HealthHandler(c *gin.Context) {
	if err := h.telegramService.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Telegram service unavailable: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"app":       h.cfg.App.Name,
		"version":   h.cfg.App.Version,
		"storage": gin.H{
			"notifications":      len(h.storage.GetNotifications()),
			"sent_notifications": len(h.storage.GetSentNotifications()),
		},
	})
}

// SendHandler отправляет одно сообщение
func (h *Handler) SendHandler(c *gin.Context) {
	var req struct {
		ChatID string `json:"chat_id" binding:"omitempty"`
		Text   string `json:"text" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	// Используем chat_id из запроса или дефолтный из конфига
	chatID := req.ChatID
	if chatID == "" {
		chatID = h.cfg.Telegram.ChatID
	}

	// Создаем уведомление
	notification := models.NewNotification(chatID, req.Text)

	// Сохраняем в хранилище
	if err := h.storage.Store(notification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to store notification: " + err.Error(),
		})
		return
	}

	// Отправляем через сервис
	sentNotification, err := h.telegramService.SendNotification(c.Request.Context(), req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to send notification: " + err.Error(),
		})
		return
	}

	// Сохраняем отправленное уведомление
	if sentNotification != nil {
		if err := h.storage.Store(sentNotification); err != nil {
			log.Printf("Failed to store sent notification: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notification sent successfully",
		"data": gin.H{
			"chat_id":    chatID,
			"text":       req.Text,
			"message_id": sentNotification.MessageID,
		},
	})
}

// BatchHandler отправляет несколько сообщений
func (h *Handler) BatchHandler(c *gin.Context) {
	var req struct {
		Messages []struct {
			ChatID string `json:"chat_id" binding:"omitempty"`
			Text   string `json:"text" binding:"required,min=1"`
		} `json:"messages" binding:"required,min=1,dive"`
		IntervalMs int `json:"interval_ms" binding:"omitempty,min=0"`
		Workers    int `json:"workers" binding:"omitempty,min=1,max=10"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	// Подготавливаем нотификации
	notifications := make([]*models.Notification, 0, len(req.Messages))
	for _, msg := range req.Messages {
		chatID := msg.ChatID
		if chatID == "" {
			chatID = h.cfg.Telegram.ChatID
		}
		notifications = append(notifications, models.NewNotification(chatID, msg.Text))
	}

	// Настраиваем параметры обработки
	interval := 2 * time.Second
	if req.IntervalMs > 0 {
		interval = time.Duration(req.IntervalMs) * time.Millisecond
	}

	workers := 2
	if req.Workers > 0 {
		workers = req.Workers
	}

	// Запускаем обработку
	result := h.telegramService.ProcessWithIntervals(c.Request.Context(), notifications, interval, workers)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Batch processing completed",
		"data": gin.H{
			"total":         len(notifications),
			"success_count": result.SuccessCount,
			"error_count":   result.ErrorCount,
			"interval_ms":   interval.Milliseconds(),
			"workers":       workers,
		},
	})
}

// NotificationsHandler возвращает список всех уведомлений
func (h *Handler) NotificationsHandler(c *gin.Context) {
	notifications := h.storage.GetNotifications()
	
	response := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		response = append(response, gin.H{
			"chat_id": n.ChatID,
			"text":    n.Text,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"count":         len(notifications),
			"notifications": response,
		},
	})
}

// SentNotificationsHandler возвращает список отправленных уведомлений
func (h *Handler) SentNotificationsHandler(c *gin.Context) {
	sentNotifications := h.storage.GetSentNotifications()
	
	response := make([]gin.H, 0, len(sentNotifications))
	for _, n := range sentNotifications {
		response = append(response, gin.H{
			"message_id": n.MessageID,
			"chat_id":    n.ChatID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"count":              len(sentNotifications),
			"sent_notifications": response,
		},
	})
}

// StatusHandler возвращает статус сервиса
func (h *Handler) StatusHandler(c *gin.Context) {
	notifications := h.storage.GetNotifications()
	sentNotifications := h.storage.GetSentNotifications()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status": "running",
			"stats": gin.H{
				"total_notifications":      len(notifications),
				"total_sent_notifications": len(sentNotifications),
				"pending_notifications":    len(notifications) - len(sentNotifications),
			},
			"config": gin.H{
				"app_name":    h.cfg.App.Name,
				"app_version": h.cfg.App.Version,
				"environment": h.cfg.App.Environment,
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})
}