package repository

import (
    "context"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
)

// RedisRepository — репозиторий для логирования изменений в Redis
type RedisRepository struct {
    client *redis.Client
    ttl    time.Duration
}

// NewRedisRepository — создаем новый клиент Redis и проверяем соединение
func NewRedisRepository(addr, password string, db, ttlSeconds int) (*RedisRepository, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    })

    // Проверка подключения
    if _, err := client.Ping(context.Background()).Result(); err != nil {
        return nil, err
    }

    return &RedisRepository{
        client: client,
        ttl:    time.Duration(ttlSeconds) * time.Second,
    }, nil
}

// LogChange записывает событие изменения с TTL
func (r *RedisRepository) LogChange(
    ctx context.Context,
    entityType, entityID, action string,
    data interface{},
) error {
    key := "changes:" + entityType + ":" + entityID

    // Формируем запись
    record := map[string]interface{}{
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "action":    action,
        "data":      data,
    }

    // Сериализуем в JSON, чтобы красиво хранить
    value, err := json.Marshal(record)
    if err != nil {
        return err
    }

    // Добавляем в начало списка
    if err := r.client.LPush(ctx, key, value).Err(); err != nil {
        return err
    }

    // Устанавливаем TTL, только если ключ ещё не имеет его (NX = Not eXists)
    r.client.ExpireNX(ctx, key, r.ttl)

    return nil
}

// Close закрывает соединение с Redis
func (r *RedisRepository) Close() error {
    return r.client.Close()
}

// RedisRepositoryInterface — интерфейс для логирования изменений
type RedisRepositoryInterface interface {
    LogChange(ctx context.Context, entityType, entityID, action string, data interface{}) error
    Close() error
}

var _ RedisRepositoryInterface = (*RedisRepository)(nil)