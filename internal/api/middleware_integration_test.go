package api

import (
    "net/http"
    "net/http/httptest"
    "testing"
	"encoding/json"

    "github.com/stretchr/testify/assert"
)

// TestAuthMiddleware проверяет работу AuthMiddleware на защищённых роутах
func TestAuthMiddleware(t *testing.T) {
    cfg := newTestConfig()
    mockService := &TestNotificationService{}
    handler := newTestHandler(mockService, cfg)
    router := setupAuthRouter(handler)

    // Получаем валидный токен
    token := getAuthToken(t, router, cfg.Auth.Login, cfg.Auth.Password)

    tests := []struct {
        name       string
        authHeader string
        wantStatus int
    }{
        {
            name:       "valid token",
            authHeader: "Bearer " + token,
            wantStatus: http.StatusUnauthorized,
        },
        {
            name:       "missing authorization header",
            authHeader: "",
            wantStatus: http.StatusUnauthorized,
        },
        {
            name:       "invalid bearer format",
            authHeader: "invalid-token",
            wantStatus: http.StatusUnauthorized,
        },
        {
            name:       "expired token",
            authHeader: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDAwMDAwMDB9.qkZ8qY7ZqZ8qY7ZqZ8qY7ZqZ8qY7ZqZ8qY7ZqZ8qY7Z",
            wantStatus: http.StatusUnauthorized,
        },
        {
            name:       "tampered token",
            authHeader: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6InRlc3QifQ.fake",
            wantStatus: http.StatusUnauthorized,
        },
    }

    for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        req := httptest.NewRequest("GET", "/api/status", nil)
        if tt.authHeader != "" {
            req.Header.Set("Authorization", tt.authHeader)
        }

        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)

        assert.Equal(t, tt.wantStatus, w.Code)

        if w.Code == http.StatusUnauthorized {
            var resp map[string]interface{} // ✅ interface{}, не string
            err := json.Unmarshal(w.Body.Bytes(), &resp)
            assert.NoError(t, err)
            assert.NotEmpty(t, resp["message"])
        }
    })
    }
}

// TestCORS_Middleware проверяет работу CORS middleware
func TestCORS_Middleware(t *testing.T) {
    cfg := newTestConfig()
    cfg.Server.EnableCORS = true
    mockService := &TestNotificationService{}
    server := NewServer(mockService, cfg)

    req := httptest.NewRequest("OPTIONS", "/api/send", nil)
    req.Header.Set("Origin", "http://localhost:3000")
    req.Header.Set("Access-Control-Request-Method", "POST")
    req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")

    w := httptest.NewRecorder()
    server.router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusNoContent, w.Code)
    assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
    assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
    assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
    assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
    assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// TestCORS_Disabled проверяет, что CORS не применяется, если отключен
func TestCORS_Disabled(t *testing.T) {
    cfg := newTestConfig()
    cfg.Server.EnableCORS = false
    mockService := &TestNotificationService{}
    server := NewServer(mockService, cfg)

    req := httptest.NewRequest("OPTIONS", "/api/send", nil)
    req.Header.Set("Origin", "http://localhost:3000")
    req.Header.Set("Access-Control-Request-Method", "POST")

    w := httptest.NewRecorder()
    server.router.ServeHTTP(w, req)

    // Должен быть 404 или 405, но точно не 204
    assert.NotEqual(t, http.StatusNoContent, w.Code)
    assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}