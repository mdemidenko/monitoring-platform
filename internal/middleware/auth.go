package middleware

import (
    "net/http"
    "strings"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

// Claims структура для JWT claims
type Claims struct {
    Username string `json:"username"`
    jwt.RegisteredClaims
}

// AuthMiddleware middleware для проверки JWT токена
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Получаем Authorization header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "error":   "Authorization header is required",
            })
            return
        }

        // Проверяем формат Bearer token
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        if tokenString == authHeader {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "error":   "Bearer token is required",
            })
            return
        }

        // Парсим и валидируем токен
        token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
            return []byte(jwtSecret), nil
        })

        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "error":   "Invalid token: " + err.Error(),
            })
            return
        }

        if !token.Valid {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "error":   "Token is invalid or expired",
            })
            return
        }

        // Если claims валидны, добавляем username в контекст
        if claims, ok := token.Claims.(*Claims); ok {
            c.Set("username", claims.Username)
        }

        c.Next()
    }
}

// GenerateJWTToken создает новый JWT токен
func GenerateJWTToken(username string, jwtSecret string, expirationHours int) (string, error) {
    expirationTime := time.Now().Add(time.Duration(expirationHours) * time.Hour)
    
    claims := &Claims{
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(expirationTime),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            NotBefore: jwt.NewNumericDate(time.Now()),
            Issuer:    "monitoring-platform",
            Subject:   username,
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(jwtSecret))
}