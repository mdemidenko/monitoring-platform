package middleware

import (
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
			c.AbortWithStatusJSON(401, gin.H{
				"success":     false,
				"status_code": 401,
				"error_type":  "Unauthorized",
				"message":     "Authorization header is required",
				"details": gin.H{
					"hint": "Include 'Authorization: Bearer <token>' header",
				},
			})
			return
		}

		// Проверяем формат Bearer token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.AbortWithStatusJSON(401, gin.H{
				"success":     false,
				"status_code": 401,
				"error_type":  "Unauthorized",
				"message":     "Bearer token is required",
				"details": gin.H{
					"expected_format": "Bearer <jwt_token>",
					"received":        authHeader,
				},
			})
			return
		}

		// Парсим и валидируем токен
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil {
			// Детализируем тип ошибки
			errorMsg := "Invalid token"
			var errorDetails gin.H
			
			if strings.Contains(err.Error(), "expired") {
				errorMsg = "Token has expired"
				errorDetails = gin.H{
					"reason": "token_expired",
					"hint":   "Please login again to get a new token",
				}
			} else if strings.Contains(err.Error(), "signature") {
				errorMsg = "Invalid token signature"
				errorDetails = gin.H{
					"reason": "invalid_signature",
					"hint":   "Token may be tampered with",
				}
			} else {
				errorDetails = gin.H{
					"parsing_error": err.Error(),
				}
			}
			
			c.AbortWithStatusJSON(401, gin.H{
				"success":     false,
				"status_code": 401,
				"error_type":  "Unauthorized",
				"message":     errorMsg,
				"details":     errorDetails,
			})
			return
		}

		if !token.Valid {
			c.AbortWithStatusJSON(401, gin.H{
				"success":     false,
				"status_code": 401,
				"error_type":  "Unauthorized",
				"message":     "Token is invalid or expired",
				"details": gin.H{
					"reason": "token_validation_failed",
				},
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