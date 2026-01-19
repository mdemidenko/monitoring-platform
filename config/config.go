package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)
type AuthConfig struct {
    JWTSecret      string `yaml:"jwt_secret" json:"jwt_secret"`
    JWTExpiration  int    `yaml:"jwt_expiration_hours" json:"jwt_expiration_hours"`
    Login          string `yaml:"login" json:"login"`
    Password       string `yaml:"password" json:"password"`
}

type ServerConfig struct {
	Port           string   `yaml:"port" json:"port"`
	Host           string   `yaml:"host" json:"host"`
	Timeout        int      `yaml:"timeout" json:"timeout"`
	GinMode        string   `yaml:"gin_mode" json:"gin_mode"`
	EnableCORS     bool     `yaml:"enable_cors" json:"enable_cors"`
	TrustedProxies []string `yaml:"trusted_proxies" json:"trusted_proxies"`
}


type FileConfig struct {
    InputFile        string
    OutputFile       string
    Workers          int           // количество воркеров для параллельной обработки
    BatchSize        int           // размер батча для обработки
    ShutdownTimeout  time.Duration // время для graceful shutdown
}

func FileLoadConfig() FileConfig {
    // Добавляем флаги командной строки
    var workers int
    var batchSize int
    var shutdownTimeout int
    
    flag.IntVar(&workers, "workers", 1, "количество воркеров для параллельной обработки")
    flag.IntVar(&batchSize, "batch", 10, "размер батча обработки")
    flag.IntVar(&shutdownTimeout, "timeout", 30, "таймаут graceful shutdown в секундах")
    flag.Parse()
    
    return FileConfig{
        InputFile:       "services.json",
        OutputFile:      "filtered_services.json",
        Workers:         workers,
        BatchSize:       batchSize,
        ShutdownTimeout: time.Duration(shutdownTimeout) * time.Second,
    }
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token" json:"bot_token"`
	ChatID   string `yaml:"chat_id" json:"chat_id"`
	Timeout  int    `yaml:"timeout" json:"timeout"`
	Debug    bool   `yaml:"debug" json:"debug"`
}

type AppConfig struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Environment string `yaml:"environment" json:"environment"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}

type Config struct {
	Telegram TelegramConfig `yaml:"telegram" json:"telegram"`
	App      AppConfig      `yaml:"app" json:"app"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Server   ServerConfig   `yaml:"server" json:"server"`
	Auth     AuthConfig     `yaml:"auth" json:"auth"`
}

// LoadConfig загружает конфигурацию из YAML файла
func LoadConfig(configPath string) (*Config, error) {
	// Если путь не указан, ищем файл в стандартных местах
	if configPath == "" {
		configPath = findConfigFile()
		if configPath == "" {
			return nil, fmt.Errorf("config file not found in standard locations")
		}
	}

	log.Printf("Loading configuration from: %s", configPath)

	// Читаем файл
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Парсим YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Валидация обязательных полей
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Переопределяем из environment variables
	config.overrideFromEnv()

	log.Printf("Configuration loaded successfully for app: %s v%s",
		config.App.Name, config.App.Version)

	return &config, nil
}

// LoadConfigWithDefaults загружает конфиг или использует значения по умолчанию
func LoadConfigWithDefaults(configPath string) *Config {
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Printf("Warning: %v, using default configuration", err)
		return DefaultConfig()
	}
	return config
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		Telegram: TelegramConfig{
			Timeout: 500,
			Debug:   false,
		},
		App: AppConfig{
			Name:        "telegram-bot",
			Version:     "1.0.0",
			Environment: "development",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Server: ServerConfig{
			Port:           "8080",
			Host:           "localhost",
			Timeout:        30,
			GinMode:        "debug",
			EnableCORS:     true,
			TrustedProxies: []string{"127.0.0.1"},
		},
		Auth: AuthConfig{
            JWTSecret:      "your-default-secret-key-change-this",
            JWTExpiration:  24,
            Login:          "admin",
            Password:       "admin123",
        },
	}
}

// Validate проверяет валидность конфигурации
func (c *Config) Validate() error {
	if c.Telegram.BotToken == "" {
		return fmt.Errorf("telegram.bot_token is required")
	}
	if c.Telegram.ChatID == "" {
		return fmt.Errorf("telegram.chat_id is required")
	}
	if c.Telegram.Timeout <= 0 {
		return fmt.Errorf("telegram.timeout must be positive")
	}
	if c.Server.Port == "" {
		c.Server.Port = "8080"
	}
	if c.Auth.JWTSecret == "" {
        return fmt.Errorf("auth.jwt_secret is required")
    }
    if c.Auth.JWTExpiration <= 0 {
        return fmt.Errorf("auth.jwt_expiration_hours must be positive")
    }
    if c.Auth.Login == "" || c.Auth.Password == "" {
        return fmt.Errorf("auth.login and auth.password are required")
    }

	validEnvironments := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
	}
	if !validEnvironments[c.App.Environment] {
		return fmt.Errorf("invalid environment: %s", c.App.Environment)
	}

	return nil
}

// IsProduction проверяет, production ли окружение
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// IsDevelopment проверяет, development ли окружение
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// overrideFromEnv переопределяет значения из environment variables
func (c *Config) overrideFromEnv() {
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		c.Telegram.BotToken = token
	}
	if chatID := os.Getenv("TELEGRAM_CHAT_ID"); chatID != "" {
		c.Telegram.ChatID = chatID
	}
	if debug := os.Getenv("TELEGRAM_DEBUG"); debug != "" {
		c.Telegram.Debug = debug == "true" || debug == "1"
	}
	// JWT и аутентификация
    if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
        c.Auth.JWTSecret = jwtSecret
    }
    if jwtExp := os.Getenv("AUTH_JWT_EXPIRATION_HOURS"); jwtExp != "" {
        if exp, err := strconv.Atoi(jwtExp); err == nil && exp > 0 {
            c.Auth.JWTExpiration = exp
        }
    }
    if login := os.Getenv("AUTH_LOGIN"); login != "" {
        c.Auth.Login = login
    }
    if password := os.Getenv("AUTH_PASSWORD"); password != "" {
        c.Auth.Password = password
    }
}

// findConfigFile ищет конфигурационный файл в стандартных местах
func findConfigFile() string {
    // Можно добавить флаг командной строки
    var configPath string
    flag.StringVar(&configPath, "config", "", "path to config file")
    flag.Parse()
    
    // Если путь указан через флаг, используем его
    if configPath != "" {
        if _, err := os.Stat(configPath); err == nil {
            log.Printf("✓ Using config from flag: %s", configPath)
            return configPath
        }
        log.Printf("✗ Config file not found: %s", configPath)
    }
    
    // Иначе ищем в рабочей директории
    wd, err := os.Getwd()
    if err != nil {
        log.Printf("Error getting working directory: %v", err)
        return ""
    }

    possiblePaths := []string{
        filepath.Join(wd, "config.yml"),
        filepath.Join(wd, "config.yaml"),
        filepath.Join(wd, "configs", "config.yml"),
        filepath.Join(wd, "configs", "config.yaml"),
    }
    
    log.Printf("Searching for config file in working directory: %s", wd)
    for _, path := range possiblePaths {
        if _, err := os.Stat(path); err == nil {
            log.Printf("✓ Found: %s", path)
            return path
        }
        log.Printf("✗ Not found: %s", path)
    }
    
    return ""
}
