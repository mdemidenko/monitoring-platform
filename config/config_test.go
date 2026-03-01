package config

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
	"path/filepath"
	"os"
    "flag"
    "time"
)

func TestDefaultConfig(t *testing.T) {
    cfg := DefaultConfig()
    assert.NotNil(t, cfg)
    assert.Equal(t, "telegram-bot", cfg.App.Name)
    assert.Equal(t, "1.0.0", cfg.App.Version)
    assert.Equal(t, "development", cfg.App.Environment)
    assert.Equal(t, "8080", cfg.Server.Port)
    assert.Equal(t, "your-default-secret-key-change-this", cfg.Auth.JWTSecret)
    assert.Equal(t, 24, cfg.Auth.JWTExpiration)
    assert.Equal(t, "admin", cfg.Auth.Login)
    assert.Equal(t, "admin123", cfg.Auth.Password)
}

func TestConfig_Validate(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
    }{
        {
            name: "valid config",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    ChatID:   "123456",
                    Timeout:  5,
                },
                App: AppConfig{
                    Environment: "development",
                },
                Auth: AuthConfig{
                    JWTSecret:     "secret",
                    JWTExpiration: 24,
                    Login:         "admin",
                    Password:      "pass",
                },
            },
            wantErr: false,
        },
        {
            name: "missing bot token",
            config: Config{
                Telegram: TelegramConfig{
                    ChatID:  "123456",
                    Timeout: 5,
                },
                App: AppConfig{Environment: "development"},
            },
            wantErr: true,
        },
        {
            name: "missing chat id",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    Timeout:  5,
                },
                App: AppConfig{Environment: "development"},
            },
            wantErr: true,
        },
        {
            name: "invalid timeout",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    ChatID:   "123456",
                    Timeout:  -1,
                },
                App: AppConfig{Environment: "development"},
            },
            wantErr: true,
        },
        {
            name: "invalid environment",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    ChatID:   "123456",
                    Timeout:  5,
                },
                App: AppConfig{Environment: "invalid"},
            },
            wantErr: true,
        },
        {
            name: "missing jwt secret",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    ChatID:   "123456",
                    Timeout:  5,
                },
                App: AppConfig{Environment: "development"},
                Auth: AuthConfig{
                    JWTExpiration: 24,
                    Login:         "admin",
                    Password:      "pass",
                },
            },
            wantErr: true,
        },
        {
            name: "invalid jwt expiration",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    ChatID:   "123456",
                    Timeout:  5,
                },
                App: AppConfig{Environment: "development"},
                Auth: AuthConfig{
                    JWTSecret:     "secret",
                    JWTExpiration: 0,
                    Login:         "admin",
                    Password:      "pass",
                },
            },
            wantErr: true,
        },
        {
            name: "missing auth login or password",
            config: Config{
                Telegram: TelegramConfig{
                    BotToken: "token123",
                    ChatID:   "123456",
                    Timeout:  5,
                },
                App: AppConfig{Environment: "development"},
                Auth: AuthConfig{
                    JWTSecret:     "secret",
                    JWTExpiration: 24,
                    Login:         "",
                    Password:      "pass",
                },
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestConfig_IsProduction_IsDevelopment(t *testing.T) {
    tests := []struct {
        environment string
        isProd      bool
        isDev       bool
    }{
        {"production", true, false},
        {"development", false, true},
        {"staging", false, false},
        {"test", false, false},
    }

    for _, tt := range tests {
        t.Run(tt.environment, func(t *testing.T) {
            cfg := &Config{
                App: AppConfig{Environment: tt.environment},
            }
            assert.Equal(t, tt.isProd, cfg.IsProduction())
            assert.Equal(t, tt.isDev, cfg.IsDevelopment())
        })
    }
}

func TestConfig_overrideFromEnv(t *testing.T) {
    // Сохраняем оригинальные переменные
    origBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    origChatID := os.Getenv("TELEGRAM_CHAT_ID")
    origDebug := os.Getenv("TELEGRAM_DEBUG")
    origJWTSecret := os.Getenv("AUTH_JWT_SECRET")
    origJWTExp := os.Getenv("AUTH_JWT_EXPIRATION_HOURS")
    origLogin := os.Getenv("AUTH_LOGIN")
    origPassword := os.Getenv("AUTH_PASSWORD")
    defer func() {
        t.Setenv("TELEGRAM_BOT_TOKEN", origBotToken)
        t.Setenv("TELEGRAM_CHAT_ID", origChatID)
        t.Setenv("TELEGRAM_DEBUG", origDebug)
        t.Setenv("AUTH_JWT_SECRET", origJWTSecret)
        t.Setenv("AUTH_JWT_EXPIRATION_HOURS", origJWTExp)
        t.Setenv("AUTH_LOGIN", origLogin)
        t.Setenv("AUTH_PASSWORD", origPassword)
    }()

    // Устанавливаем переменные
    t.Setenv("TELEGRAM_BOT_TOKEN", "env-token")
    t.Setenv("TELEGRAM_CHAT_ID", "987654")
    t.Setenv("TELEGRAM_DEBUG", "true")
    t.Setenv("AUTH_JWT_SECRET", "env-secret")
    t.Setenv("AUTH_JWT_EXPIRATION_HOURS", "48")
    t.Setenv("AUTH_LOGIN", "env-user")
    t.Setenv("AUTH_PASSWORD", "env-pass")

    cfg := &Config{
        Telegram: TelegramConfig{
            BotToken: "default-token",
            ChatID:   "default-chat",
            Debug:    false,
        },
        Auth: AuthConfig{
            JWTSecret:     "default-secret",
            JWTExpiration: 24,
            Login:         "default-user",
            Password:      "default-pass",
        },
    }

    cfg.overrideFromEnv()

    assert.Equal(t, "env-token", cfg.Telegram.BotToken)
    assert.Equal(t, "987654", cfg.Telegram.ChatID)
    assert.True(t, cfg.Telegram.Debug)
    assert.Equal(t, "env-secret", cfg.Auth.JWTSecret)
    assert.Equal(t, 48, cfg.Auth.JWTExpiration)
    assert.Equal(t, "env-user", cfg.Auth.Login)
    assert.Equal(t, "env-pass", cfg.Auth.Password)
}

func TestLoadConfig_Success(t *testing.T) {
    // Создаём временный YAML-файл
    tmpFile := filepath.Join(t.TempDir(), "config.yml")
    data := `
telegram:
  bot_token: "test-token"
  chat_id: "123456"
  timeout: 10
  debug: false
app:
  name: "test-app"
  version: "1.0.0"
  environment: "development"
server:
  port: "8081"
  host: "0.0.0.0"
  timeout: 60
  gin_mode: "test"
  enable_cors: false
  grpc_port: "9091"
auth:
  jwt_secret: "test-secret"
  jwt_expiration_hours: 12
  login: "testuser"
  password: "testpass"
`

    // ✅ Используем переменную `data`, а не строку
    err := os.WriteFile(tmpFile, []byte(data), 0644)
    require.NoError(t, err, "Failed to write test config file")

    cfg, err := LoadConfig(tmpFile)
    require.NoError(t, err) // ✅ Лучше require, чтобы остановить тест
    assert.NotNil(t, cfg)

    // Проверяем значения
    assert.Equal(t, "test-token", cfg.Telegram.BotToken)
    assert.Equal(t, "123456", cfg.Telegram.ChatID)
    assert.Equal(t, "test-app", cfg.App.Name)
    assert.Equal(t, "development", cfg.App.Environment)
    assert.Equal(t, "8081", cfg.Server.Port)
    assert.Equal(t, "test-secret", cfg.Auth.JWTSecret)
}

func TestLoadConfig_Error(t *testing.T) {
    // Файл не существует
    _, err := LoadConfig("nonexistent.yaml")
    assert.Error(t, err)

    // Неверный YAML
    tmpDir := t.TempDir()
    tmpFile := filepath.Join(tmpDir, "invalid.yaml")

    // ✅ Пишем файл и проверяем ошибку
    err = os.WriteFile(tmpFile, []byte("invalid: yaml: :"), 0644)
    require.NoError(t, err, "Failed to create test invalid YAML file")

    // Проверяем, что LoadConfig возвращает ошибку
    _, err = LoadConfig(tmpFile)
    assert.Error(t, err, "Expected error when loading invalid YAML")
}

func TestLoadConfigWithDefaults(t *testing.T) {
    // Передаём несуществующий путь → должен вернуться DefaultConfig
    cfg := LoadConfigWithDefaults("config.yml")
    assert.NotNil(t, cfg)
    assert.Equal(t, "telegram-bot", cfg.App.Name)
    assert.Equal(t, "1.0.0", cfg.App.Version)
}

func TestLoadConfig_WithFlagConfig(t *testing.T) {
    // Создаём временный файл
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "config.yaml")
    data := `
telegram:
  bot_token: "test-token"
  chat_id: "123"
  timeout: 5
app:
  name: "test"
  version: "1.0.0"
  environment: "development"
auth:
  jwt_secret: "secret"
  jwt_expiration_hours: 24
  login: "admin"
  password: "pass"
`
    err := os.WriteFile(configPath, []byte(data), 0644)
    require.NoError(t, err, "Failed to write config file")

    // Сохраняем аргументы
    origArgs := os.Args
    defer func() { os.Args = origArgs }()

    // Подменяем аргументы — как будто вызвали: ./app -config /path/to/config.yaml
    os.Args = []string{"cmd", "-config", configPath}

    cfg, err := LoadConfig("") // пустой путь → сработает findConfigFile
    assert.NoError(t, err)
    assert.NotNil(t, cfg)
    assert.Equal(t, "test-token", cfg.Telegram.BotToken)
}

func TestFindConfigFile_SearchPaths(t *testing.T) {
    // Создаём временную директорию
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "config.yaml")
    err := os.WriteFile(configPath, []byte("test: config"), 0644)
    require.NoError(t, err, "Failed to write test config file")

    // Подменяем рабочую директорию
    origWd, err := os.Getwd()
    assert.NoError(t, err)
    defer func() {
    if err := os.Chdir(origWd); err != nil {
        t.Logf("Failed to restore working directory: %v", err)
        }
    }()
    err = os.Chdir(tmpDir)
    assert.NoError(t, err)

    // Очищаем аргументы
    origArgs := os.Args
    os.Args = []string{"cmd"}
    defer func() { os.Args = origArgs }()

    // Очищаем флаги
    flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

    // Вызываем findConfigFile
    found := findConfigFile()

    // Приводим оба пути к канонической форме
    expected, _ := filepath.EvalSymlinks(configPath)
    actual, _ := filepath.EvalSymlinks(found)

    assert.Equal(t, expected, actual, "найденный путь должен совпадать с ожидаемым")
}

func TestFileLoadConfig(t *testing.T) {
    // Сохраняем os.Args
    origArgs := os.Args
    defer func() { os.Args = origArgs }()

    // Очищаем флаги
    flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

    // Устанавливаем аргументы
    os.Args = []string{"cmd", "-workers", "4", "-batch", "100", "-timeout", "60"}

    config := FileLoadConfig()

    assert.Equal(t, "services.json", config.InputFile)
    assert.Equal(t, "filtered_services.json", config.OutputFile)
    assert.Equal(t, 4, config.Workers)
    assert.Equal(t, 100, config.BatchSize)
    assert.Equal(t, 60*time.Second, config.ShutdownTimeout)
}