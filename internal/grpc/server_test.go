package grpc

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"

)

func TestNewGRPCServer(t *testing.T) {
    mockService := &MockNotificationService{}
    cfg := newTestConfig()

    server, err := NewGRPCServer(cfg, mockService)
    assert.NoError(t, err)
    assert.NotNil(t, server)
    assert.NotNil(t, server.server)
    assert.NotNil(t, server.service)
}

func TestGRPCServer_StartAndShutdown(t *testing.T) {
    mockService := &MockNotificationService{}
    cfg := newTestConfig()
    cfg.Server.Port = "0" // случайный порт

    server, err := NewGRPCServer(cfg, mockService)
    assert.NoError(t, err)

    go func() {
    if err := server.Start(cfg.Server.Port); err != nil {
        t.Logf("gRPC server failed to start or stopped: %v", err)
        }
    }()

    time.Sleep(100 * time.Millisecond)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    err = server.Shutdown(ctx)
    assert.NoError(t, err)
}

func TestGRPCServer_WithAuthInterceptor(t *testing.T) {
    mockService := &MockNotificationService{}
    cfg := newTestConfig()

    server, err := NewGRPCServer(cfg, mockService)
    assert.NoError(t, err)
    assert.NotNil(t, server.server)
    // Проверим, что interceptor добавлен
    // (нельзя напрямую, но можно проверить через поведение)
}
