package grpc

import (
	"context"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthInterceptor для JWT аутентификации в gRPC
type AuthInterceptor struct {
	jwtSecret     string
	publicMethods map[string]bool
}

// NewAuthInterceptor создает новый JWT интерцептор
func NewAuthInterceptor(jwtSecret string) *AuthInterceptor {
	publicMethods := map[string]bool{
		"/monitoring.MonitoringService/Login": true,
	}
	
	return &AuthInterceptor{
		jwtSecret:     jwtSecret,
		publicMethods: publicMethods,
	}
}

// Unary возвращает unary interceptor для аутентификации
func (interceptor *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if interceptor.publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}
		
		err := interceptor.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		
		return handler(ctx, req)
	}
}

// Stream возвращает stream interceptor для аутентификации
func (interceptor *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if interceptor.publicMethods[info.FullMethod] {
			return handler(srv, stream)
		}
		
		err := interceptor.authenticate(stream.Context())
		if err != nil {
			return err
		}
		
		return handler(srv, stream)
	}
}

// authenticate проверяет JWT токен из metadata
func (interceptor *AuthInterceptor) authenticate(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "metadata is not provided")
	}
	
	authHeaders := md["authorization"]
	if len(authHeaders) == 0 {
		return status.Error(codes.Unauthenticated, "authorization token is not provided")
	}
	
	tokenString := authHeaders[0]
	if !strings.HasPrefix(tokenString, "Bearer ") {
		return status.Error(codes.Unauthenticated, "invalid authorization format")
	}
	
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, status.Errorf(codes.Unauthenticated, "unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(interceptor.jwtSecret), nil
	})
	
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	
	if !token.Valid {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	
	return nil
}