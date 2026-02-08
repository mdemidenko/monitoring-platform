package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/core"
	"github.com/mdemidenko/monitoring-platform/internal/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	handler    *Handler
	cfg        *config.Config
}

// NewServer создает новый сервер с Gin
func NewServer(notificationService *core.NotificationService, cfg *config.Config) *Server {
	// Устанавливаем режим Gin
	setGinMode(cfg)
	
	// Создаем роутер Gin
	router := gin.New()
	
	// Создаем обработчик
	handler := NewHandler(notificationService, cfg)
	
	server := &Server{
		router:  router,
		handler: handler,
		cfg:     cfg,
	}
	
	// Настраиваем middleware и роуты
	server.setupMiddleware()
	server.setupRoutes()
	
	return server
}

// setGinMode устанавливает режим работы Gin
func setGinMode(cfg *config.Config) {
	switch cfg.Server.GinMode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}

// setupMiddleware настраивает middleware для сервера
func (s *Server) setupMiddleware() {
	// Recovery middleware (восстанавливает сервер после panic)
	s.router.Use(gin.Recovery())
	
	// Логирование запросов в формате Gin
	if s.cfg.Server.GinMode != "release" {
		s.router.Use(gin.Logger())
	}
	
	// Пользовательское логирование
	s.router.Use(s.customLoggingMiddleware())
	
	// CORS если включен
	if s.cfg.Server.EnableCORS {
		s.router.Use(corsMiddleware())
	}
	
	// Настраиваем trusted proxies
	if len(s.cfg.Server.TrustedProxies) > 0 {
		if err := s.router.SetTrustedProxies(s.cfg.Server.TrustedProxies); err != nil {
			log.Printf("Warning: failed to set trusted proxies: %v", err)
		}
	}
}

// customLoggingMiddleware добавляет детальное логирование
func (s *Server) customLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		
		// Обрабатываем запрос
		c.Next()
		
		// Логируем после обработки
		duration := time.Since(start)
		status := c.Writer.Status()
		
		if query != "" {
			path = path + "?" + query
		}
		
		log.Printf("[API] %3d | %13v | %15s | %-7s %s",
			status,
			duration,
			c.ClientIP(),
			c.Request.Method,
			path,
		)
	}
}

// corsMiddleware настраивает CORS
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// setupRoutes настраивает маршруты API
func (s *Server) setupRoutes() {
	// Swagger UI документация
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, 
		ginSwagger.URL("/swagger/doc.json"),
		ginSwagger.DefaultModelsExpandDepth(-1),
	))
	
	// Группа API v1
	api := s.router.Group("/api")
	{
		// Public routes (не требуют аутентификации)
        api.GET("/health", s.handler.HealthHandler)
        api.POST("/auth/login", s.handler.LoginHandler)
		
		// Protected routes group (требуют JWT)
        protected := api.Group("")
        protected.Use(middleware.AuthMiddleware(s.cfg.Auth.JWTSecret))
        {
            // Отправка сообщений
            protected.POST("/send", s.handler.SendHandler)
            protected.POST("/batch", s.handler.BatchHandler)
            
            // Получение данных
            protected.GET("/notifications", s.handler.NotificationsHandler)
            protected.GET("/notifications/sent", s.handler.SentNotificationsHandler)
            protected.GET("/status", s.handler.StatusHandler)
        }
	}
	
	// Корневой маршрут
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "Telegram Notification Service",
			"version": s.cfg.App.Version,
			"status":  "running",
			"docs":    "/swagger/index.html",
			"api":     "/api/health",
		})
	})
	
	// Обработка 404
	s.router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not found",
			"message": "The requested route does not exist",
			"path":    c.Request.URL.Path,
			"docs":    "/swagger/index.html",
		})
	})
}

// Start запускает сервер
func (s *Server) Start(port string) {
	addr := ":" + port
	if s.cfg.Server.Host != "" && s.cfg.Server.Host != "localhost" {
		addr = s.cfg.Server.Host + ":" + port
	}
	
	s.httpServer = &http.Server{
		Addr:           addr,
		Handler:        s.router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
	
	log.Printf("🚀 Сервер запущен на %s", addr)
	log.Printf("📡 Режим: %s", s.cfg.Server.GinMode)
	log.Printf("📊 Endpoints:")
	log.Printf("   GET  %s/api/health", addr)
	log.Printf("   POST %s/api/auth/login", addr)
	log.Printf("   POST %s/api/send", addr)
	log.Printf("   POST %s/api/batch", addr)
	log.Printf("   GET  %s/api/notifications", addr)
	log.Printf("   GET  %s/api/notifications/sent", addr)
	log.Printf("   GET  %s/api/status", addr)
	log.Printf("📚 Swagger UI: %s/swagger/index.html", addr)
	
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Ошибка сервера: %v", err)
	}
}

// Shutdown gracefully останавливает сервер
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}