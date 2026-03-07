package middleware

import (
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/assert"

    "net/http/httptest"
)

func init() {
    gin.SetMode(gin.TestMode)
}

// TestAuthMiddleware_MissingHeader проверяет отсутствие заголовка Authorization
func TestAuthMiddleware_MissingHeader(t *testing.T) {
    router := gin.New()
    router.Use(AuthMiddleware("secret"))
    router.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "ok"})
    })

    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 401, w.Code)
    assert.Contains(t, w.Body.String(), "Authorization header is required")
    assert.Contains(t, w.Body.String(), "hint")
}

// TestAuthMiddleware_InvalidFormat проверяет неверный формат Bearer
func TestAuthMiddleware_InvalidFormat(t *testing.T) {
    router := gin.New()
    router.Use(AuthMiddleware("secret"))
    router.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "ok"})
    })

    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "invalid-token")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 401, w.Code)
    assert.Contains(t, w.Body.String(), "Bearer token is required")
    assert.Contains(t, w.Body.String(), "expected_format")
}

// TestAuthMiddleware_ExpiredToken проверяет просроченный токен
func TestAuthMiddleware_ExpiredToken(t *testing.T) {
    router := gin.New()
    router.Use(AuthMiddleware("secret"))
    router.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "ok"})
    })

    claims := &Claims{
        Username: "testuser",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Subject:   "testuser",
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, _ := token.SignedString([]byte("secret"))

    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+tokenString)
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 401, w.Code)
    assert.Contains(t, w.Body.String(), "Token has expired")
    assert.Contains(t, w.Body.String(), "hint")
}

// TestAuthMiddleware_InvalidSignature проверяет неверную подпись
func TestAuthMiddleware_InvalidSignature(t *testing.T) {
    router := gin.New()
    router.Use(AuthMiddleware("secret"))
    router.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "ok"})
    })

    claims := &Claims{
        Username: "testuser",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, _ := token.SignedString([]byte("wrong-secret"))

    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+tokenString)
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 401, w.Code)
    assert.Contains(t, w.Body.String(), "Invalid token signature")
    assert.Contains(t, w.Body.String(), "reason")
}

// TestAuthMiddleware_ValidToken проверяет валидный токен
func TestAuthMiddleware_ValidToken(t *testing.T) {
    router := gin.New()
    router.Use(AuthMiddleware("secret"))
    var capturedUsername string
    router.GET("/test", func(c *gin.Context) {
        username, exists := c.Get("username")
        assert.True(t, exists)
        capturedUsername = username.(string)
        c.JSON(200, gin.H{"message": "ok"})
    })

    claims := &Claims{
        Username: "testuser",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Subject:   "testuser",
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, _ := token.SignedString([]byte("secret"))

    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+tokenString)
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)
    assert.Equal(t, "testuser", capturedUsername)
    assert.Contains(t, w.Body.String(), "ok")
}

// TestAuthMiddleware_TamperedToken проверяет изменённый токен
func TestAuthMiddleware_TamperedToken(t *testing.T) {
    router := gin.New()
    router.Use(AuthMiddleware("secret"))
    router.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "ok"})
    })

    // Валидный токен
    claims := &Claims{
        Username: "testuser",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, _ := token.SignedString([]byte("secret"))

    // Меняем подпись
    tampered := tokenString[:len(tokenString)-5] + "12345"

    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+tampered)
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, 401, w.Code)
    assert.Contains(t, w.Body.String(), "Invalid token signature")
}

// TestGenerateJWTToken проверяет создание токена
func TestGenerateJWTToken(t *testing.T) {
    token, err := GenerateJWTToken("testuser", "secret", 24)
    assert.NoError(t, err)
    assert.NotEmpty(t, token)

    // Парсим токен
    parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte("secret"), nil
    })
    assert.NoError(t, err)
    assert.True(t, parsedToken.Valid)

    claims, ok := parsedToken.Claims.(*Claims)
    assert.True(t, ok)
    assert.Equal(t, "testuser", claims.Username)
    assert.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, 1*time.Second)
}

// TestGenerateJWTToken_EmptyUsername проверяет пустой username
func TestGenerateJWTToken_EmptyUsername(t *testing.T) {
    token, err := GenerateJWTToken("", "secret", 24)
    assert.NoError(t, err)
    assert.NotEmpty(t, token)

    parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte("secret"), nil
    })
    assert.NoError(t, err)
    assert.True(t, parsedToken.Valid)

    claims, ok := parsedToken.Claims.(*Claims)
    assert.True(t, ok)
    assert.Equal(t, "", claims.Username)
}