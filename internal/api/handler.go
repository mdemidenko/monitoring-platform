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
// @Summary Проверка состояния сервиса
// @Description Проверяет доступность сервиса и всех зависимых компонентов (Telegram API, хранилище)
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse "Сервис работает корректно"
// @Failure 503 {object} ErrorResponse "Сервис недоступен"
// @Router /api/health [get]
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
// @Summary Отправка одного уведомления
// @Description Отправляет одно сообщение в указанный чат Telegram. Если chat_id не указан, используется чат из конфигурации
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body SendRequest true "Данные для отправки уведомления"
// @Success 200 {object} SendResponse "Уведомление успешно отправлено"
// @Failure 400 {object} ErrorResponse "Некорректные данные запроса"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 500 {object} ErrorResponse "Ошибка отправки уведомления"
// @Router /api/send [post]
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
// @Summary Пакетная отправка уведомлений
// @Description Отправляет несколько сообщений с возможностью настройки интервалов между отправками и количества параллельных воркеров
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body BatchRequest true "Данные для пакетной отправки"
// @Success 200 {object} BatchResponse "Пакетная обработка завершена"
// @Failure 400 {object} ErrorResponse "Некорректные данные запроса"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Router /api/batch [post]
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
// @Summary Получение списка всех созданных уведомлений
// @Description Возвращает список всех уведомлений, которые были созданы для отправки (включая неотправленные)
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} NotificationsResponse "Список уведомлений"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Router /api/notifications [get]
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
// @Summary Получение списка отправленных уведомлений
// @Description Возвращает список уведомлений, которые были успешно отправлены в Telegram
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SentNotificationsResponse "Список отправленных уведомлений"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Router /api/notifications/sent [get]
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
// @Summary Получение статуса и статистики сервиса
// @Description Возвращает текущее состояние сервиса, статистику отправленных уведомлений и конфигурационные параметры
// @Tags status
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} StatusResponse "Статус сервиса"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Router /api/status [get]
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

// Определение структур для документации Swagger

// SendRequest представляет запрос на отправку уведомления
// @Description Запрос на отправку одного уведомления
type SendRequest struct {
	// ID чата Telegram (опционально, если не указан - используется из конфигурации)
	ChatID string `json:"chat_id" example:"123456789"`
	// Текст сообщения для отправки (обязательное поле)
	Text string `json:"text" example:"Привет, это тестовое сообщение!" binding:"required,min=1"`
}

// BatchMessage представляет сообщение в пакетном запросе
// @Description Сообщение в составе пакетного запроса
type BatchMessage struct {
	// ID чата Telegram (опционально)
	ChatID string `json:"chat_id" example:"123456789"`
	// Текст сообщения для отправки
	Text string `json:"text" example:"Тестовое сообщение 1" binding:"required,min=1"`
}

// BatchRequest представляет запрос на пакетную отправку уведомлений
// @Description Запрос на пакетную отправку уведомлений с настройками интервалов и параллелизма
type BatchRequest struct {
	// Список сообщений для отправки
	Messages []BatchMessage `json:"messages" binding:"required,min=1,dive"`
	// Интервал между отправками сообщений в миллисекундах (опционально)
	IntervalMs int `json:"interval_ms" example:"1000"`
	// Количество параллельных воркеров для отправки (опционально, от 1 до 10)
	Workers int `json:"workers" example:"2"`
}

// HealthResponse представляет ответ на запрос проверки здоровья
// @Description Ответ сервиса на запрос проверки состояния
type HealthResponse struct {
	// Статус сервиса
	Status string `json:"status" example:"ok"`
	// Временная метка ответа
	Timestamp string `json:"timestamp" example:"2024-01-01T12:00:00Z"`
	// Название приложения
	App string `json:"app" example:"monitoring-platform"`
	// Версия приложения
	Version string `json:"version" example:"1.0.0"`
	// Статистика хранилища
	Storage struct {
		// Количество созданных уведомлений
		Notifications int `json:"notifications" example:"10"`
		// Количество отправленных уведомлений
		SentNotifications int `json:"sent_notifications" example:"8"`
	} `json:"storage"`
}

// SendResponse представляет ответ на успешную отправку уведомления
// @Description Ответ при успешной отправке уведомления
type SendResponse struct {
	// Флаг успешного выполнения
	Success bool `json:"success" example:"true"`
	// Сообщение о результате
	Message string `json:"message" example:"Notification sent successfully"`
	// Данные об отправленном уведомлении
	Data struct {
		// ID чата, куда отправлено сообщение
		ChatID string `json:"chat_id" example:"123456789"`
		// Текст отправленного сообщения
		Text string `json:"text" example:"Тестовое сообщение"`
		// ID сообщения в Telegram (если отправка успешна)
		MessageID int64 `json:"message_id" example:"123"`
	} `json:"data"`
}

// BatchResponse представляет ответ на пакетную отправку уведомлений
// @Description Ответ при выполнении пакетной отправки уведомлений
type BatchResponse struct {
	// Флаг успешного выполнения
	Success bool `json:"success" example:"true"`
	// Сообщение о результате
	Message string `json:"message" example:"Batch processing completed"`
	// Данные о результате обработки
	Data struct {
		// Общее количество обработанных сообщений
		Total int `json:"total" example:"10"`
		// Количество успешно отправленных сообщений
		SuccessCount int `json:"success_count" example:"8"`
		// Количество сообщений с ошибкой отправки
		ErrorCount int `json:"error_count" example:"2"`
		// Использованный интервал между отправками в мс
		IntervalMs int64 `json:"interval_ms" example:"2000"`
		// Количество использованных воркеров
		Workers int `json:"workers" example:"2"`
	} `json:"data"`
}

// NotificationItem представляет элемент списка уведомлений
// @Description Элемент уведомления в списке
type NotificationItem struct {
	// ID чата
	ChatID string `json:"chat_id" example:"123456789"`
	// Текст уведомления
	Text string `json:"text" example:"Тестовое сообщение"`
}

// NotificationsResponse представляет ответ со списком уведомлений
// @Description Ответ со списком всех созданных уведомлений
type NotificationsResponse struct {
	// Флаг успешного выполнения
	Success bool `json:"success" example:"true"`
	// Данные со списком уведомлений
	Data struct {
		// Общее количество уведомлений
		Count int `json:"count" example:"5"`
		// Список уведомлений
		Notifications []NotificationItem `json:"notifications"`
	} `json:"data"`
}

// SentNotificationItem представляет элемент списка отправленных уведомлений
// @Description Элемент отправленного уведомления в списке
type SentNotificationItem struct {
	// ID сообщения в Telegram
	MessageID int64 `json:"message_id" example:"123"`
	// ID чата
	ChatID int64 `json:"chat_id" example:"123456789"`
}

// SentNotificationsResponse представляет ответ со списком отправленных уведомлений
// @Description Ответ со списком отправленных уведомлений
type SentNotificationsResponse struct {
	// Флаг успешного выполнения
	Success bool `json:"success" example:"true"`
	// Данные со списком отправленных уведомлений
	Data struct {
		// Общее количество отправленных уведомлений
		Count int `json:"count" example:"3"`
		// Список отправленных уведомлений
		SentNotifications []SentNotificationItem `json:"sent_notifications"`
	} `json:"data"`
}

// StatusResponse представляет ответ со статусом сервиса
// @Description Ответ с текущим статусом и статистикой сервиса
type StatusResponse struct {
	// Флаг успешного выполнения
	Success bool `json:"success" example:"true"`
	// Данные о статусе сервиса
	Data struct {
		// Текущий статус сервиса
		Status string `json:"status" example:"running"`
		// Статистика сервиса
		Stats struct {
			// Всего созданных уведомлений
			TotalNotifications int `json:"total_notifications" example:"15"`
			// Всего отправленных уведомлений
			TotalSentNotifications int `json:"total_sent_notifications" example:"12"`
			// Количество ожидающих отправки уведомлений
			PendingNotifications int `json:"pending_notifications" example:"3"`
		} `json:"stats"`
		// Конфигурационные параметры
		Config struct {
			// Название приложения
			AppName string `json:"app_name" example:"monitoring-platform"`
			// Версия приложения
			AppVersion string `json:"app_version" example:"1.0.0"`
			// Окружение выполнения
			Environment string `json:"environment" example:"development"`
		} `json:"config"`
		// Временная метка
		Timestamp string `json:"timestamp" example:"2024-01-01T12:00:00Z"`
	} `json:"data"`
}
