package grpc

import (
    "context"
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/assert"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// TestAuthInterceptor_Unary_PublicMethod проверяет, что public методы не требуют токена
func TestAuthInterceptor_Unary_PublicMethod(t *testing.T) {
    interceptor := NewAuthInterceptor("secret")
    ctx := context.Background()
    info := &grpc.UnaryServerInfo{
        FullMethod: "/monitoring.MonitoringService/Login",
    }

    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return nil, nil
    }

    _, err := interceptor.Unary()(ctx, nil, info, handler)
    assert.NoError(t, err)
}

// TestAuthInterceptor_Unary_MissingMetadata проверяет отсутствие metadata
func TestAuthInterceptor_Unary_MissingMetadata(t *testing.T) {
    interceptor := NewAuthInterceptor("secret")
    ctx := context.Background()
    info := &grpc.UnaryServerInfo{
        FullMethod: "/monitoring.MonitoringService/SendNotification",
    }

    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return nil, nil
    }

    _, err := interceptor.Unary()(ctx, nil, info, handler)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
    assert.Contains(t, err.Error(), "metadata is not provided")
}

// TestAuthInterceptor_Unary_MissingAuthorization проверяет отсутствие заголовка authorization
func TestAuthInterceptor_Unary_MissingAuthorization(t *testing.T) {
    md := metadata.MD{} // пустые metadata
    ctx := metadata.NewIncomingContext(context.Background(), md)
    interceptor := NewAuthInterceptor("secret")
    info := &grpc.UnaryServerInfo{
        FullMethod: "/monitoring.MonitoringService/SendNotification",
    }

    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return nil, nil
    }

    _, err := interceptor.Unary()(ctx, nil, info, handler)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
    assert.Contains(t, err.Error(), "authorization token is not provided")
}

// TestAuthInterceptor_Unary_InvalidFormat проверяет неверный формат токена
func TestAuthInterceptor_Unary_InvalidFormat(t *testing.T) {
    md := metadata.Pairs("authorization", "invalid-token")
    ctx := metadata.NewIncomingContext(context.Background(), md)
    interceptor := NewAuthInterceptor("secret")
    info := &grpc.UnaryServerInfo{
        FullMethod: "/monitoring.MonitoringService/SendNotification",
    }

    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return nil, nil
    }

    _, err := interceptor.Unary()(ctx, nil, info, handler)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
    assert.Contains(t, err.Error(), "invalid authorization format")
}

// TestAuthInterceptor_Unary_ValidToken проверяет валидный JWT токен
func TestAuthInterceptor_Unary_ValidToken(t *testing.T) {
    // Создаём валидный токен
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
    })
    tokenString, _ := token.SignedString([]byte("secret"))

    md := metadata.Pairs("authorization", "Bearer "+tokenString)
    ctx := metadata.NewIncomingContext(context.Background(), md)
    interceptor := NewAuthInterceptor("secret")
    info := &grpc.UnaryServerInfo{
        FullMethod: "/monitoring.MonitoringService/SendNotification",
    }

    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return nil, nil
    }

    _, err := interceptor.Unary()(ctx, nil, info, handler)
    assert.NoError(t, err)
}

// TestAuthInterceptor_Unary_ExpiredToken проверяет просроченный токен
func TestAuthInterceptor_Unary_ExpiredToken(t *testing.T) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // просрочен
    })
    tokenString, _ := token.SignedString([]byte("secret"))

    md := metadata.Pairs("authorization", "Bearer "+tokenString)
    ctx := metadata.NewIncomingContext(context.Background(), md)
    interceptor := NewAuthInterceptor("secret")
    info := &grpc.UnaryServerInfo{
        FullMethod: "/monitoring.MonitoringService/SendNotification",
    }

    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return nil, nil
    }

    _, err := interceptor.Unary()(ctx, nil, info, handler)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
    assert.Contains(t, err.Error(), "invalid token")
}

// TestAuthInterceptor_Unary_ValidToken проверяет невалидный JWT токен
func TestAuthInterceptor_authenticate_InvalidToken(t *testing.T) {
    interceptor := NewAuthInterceptor("secret")
    ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
        "authorization", "Bearer invalid.token.signature",
    ))

    err := interceptor.authenticate(ctx)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthInterceptor_Stream проверяет stream interceptor
func TestAuthInterceptor_Stream(t *testing.T) {
    interceptor := NewAuthInterceptor("secret")
    stream := &testServerStream{ctx: context.Background()}
    info := &grpc.StreamServerInfo{
        FullMethod: "/monitoring.MonitoringService/BatchSend",
    }

    handler := func(srv interface{}, stream grpc.ServerStream) error {
        return nil
    }

    err := interceptor.Stream()(nil, stream, info, handler)
    assert.Error(t, err)
    assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAuthInterceptor_Stream_PublicMethod(t *testing.T) {
    interceptor := NewAuthInterceptor("secret")
    stream := &testServerStream{ctx: context.Background()}
    info := &grpc.StreamServerInfo{
        FullMethod: "/monitoring.MonitoringService/Login",
    }

    handler := func(srv interface{}, stream grpc.ServerStream) error {
        return nil
    }

    err := interceptor.Stream()(nil, stream, info, handler)
    assert.NoError(t, err)
}

// testServerStream — минимальная реализация grpc.ServerStream для тестов
type testServerStream struct {
    ctx context.Context
}

func (s *testServerStream) Context() context.Context {
    return s.ctx
}

func (s *testServerStream) SendMsg(m interface{}) error {
    return nil
}

func (s *testServerStream) RecvMsg(m interface{}) error {
    return nil
}

func (s *testServerStream) SendHeader(md metadata.MD) error {
    return nil
}

func (s *testServerStream) SetHeader(md metadata.MD) error {
    return nil
}

func (s *testServerStream) SetTrailer(md metadata.MD) {
    // Ничего не возвращаем — в новых версиях gRPC это void
}