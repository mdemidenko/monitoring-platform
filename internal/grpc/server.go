package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/core"
	grpcGenerated "github.com/mdemidenko/monitoring-platform/pkg/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// GRPCServer представляет gRPC сервер
type GRPCServer struct {
	server   *grpc.Server
	listener net.Listener
	service  *MonitoringService
	cfg      *config.Config
}

// NewGRPCServer создает новый gRPC сервер
func NewGRPCServer(
	cfg *config.Config,
	notificationService *core.NotificationService,
) (*GRPCServer, error) {
	
	// Создаем реализацию сервиса
	monitoringService := NewMonitoringService(notificationService, cfg)
	
	// Настраиваем опции сервера
	serverOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    10 * time.Second,
			Timeout: 20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	
	// Добавляем JWT interceptor если включена аутентификация
	if cfg.Auth.JWTSecret != "" {
		authInterceptor := NewAuthInterceptor(cfg.Auth.JWTSecret)
		serverOpts = append(serverOpts, 
			grpc.UnaryInterceptor(authInterceptor.Unary()),
			grpc.StreamInterceptor(authInterceptor.Stream()),
		)
	}
	
	// Опционально добавляем TLS для production
	if cfg.Server.GinMode == "production" {
		log.Println("⚠️  Внимание: gRPC запущен без TLS. Для production рекомендуется настроить TLS")
	}
	
	// Создаем gRPC сервер
	gRPCServer := grpc.NewServer(serverOpts...)
	
	// Регистрируем сервис
	grpcGenerated.RegisterMonitoringServiceServer(gRPCServer, monitoringService)
	
	// Включаем reflection для тестирования (можно отключить в production)
	if cfg.IsDevelopment() {
		reflection.Register(gRPCServer)
	}
	
	return &GRPCServer{
		server:  gRPCServer,
		service: monitoringService,
		cfg:     cfg,
	}, nil
}

// Start запускает gRPC сервер
func (s *GRPCServer) Start(port string) error {
	addr := ":" + port
	if s.cfg.Server.Host != "" && s.cfg.Server.Host != "localhost" {
		addr = s.cfg.Server.Host + ":" + port
	}
	
	// Создаем listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	
	s.listener = listener
	
	log.Printf("🚀 gRPC сервер запущен на %s", addr)
	log.Printf("📡 Режим: %s", s.cfg.Server.GinMode)
	log.Printf("🔐 Аутентификация: %v", s.cfg.Auth.JWTSecret != "")
	
	// Запускаем сервер
	if err := s.server.Serve(listener); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}
	
	return nil
}

// Shutdown gracefully останавливает gRPC сервер
func (s *GRPCServer) Shutdown(ctx context.Context) error {
	if s.server != nil {
		log.Println("🛑 Graceful shutdown gRPC сервера...")
		s.server.GracefulStop()
	}
	return nil
}

// GetServer возвращает инстанс grpc.Server (для тестирования)
func (s *GRPCServer) GetServer() *grpc.Server {
	return s.server
}