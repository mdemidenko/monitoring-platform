package repository

import (
    "context"
    "log"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/bson"
)

var _ Repository = (*MongoDBRepository)(nil)

// MongoDBRepository — реализация репозитория для MongoDB
type MongoDBRepository struct {
    client     *mongo.Client
    collection *mongo.Collection
}

// NewMongoDBRepository — создаёт новый репозиторий с подключением к MongoDB
func NewMongoDBRepository(uri, dbName, collectionName string) (*MongoDBRepository, error) {
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
    if err != nil {
        return nil, err
    }

    // Проверим подключение
    if err := client.Ping(context.Background(), nil); err != nil {
        return nil, err
    }

    log.Printf("✅ Подключено к MongoDB: %s", uri)

    collection := client.Database(dbName).Collection(collectionName)

    return &MongoDBRepository{
        client:     client,
        collection: collection,
    }, nil
}

// LoadAllServices загружает все сервисы из MongoDB
// Возвращает только необходимые поля: id, name, tenant
func (r *MongoDBRepository) LoadAllServices(ctx context.Context) ([]models.Service, error) {
    cursor, err := r.collection.Find(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var services []models.Service
    if err = cursor.All(ctx, &services); err != nil {
        return nil, err
    }

    log.Printf("📥 Загружено %d сервисов из MongoDB", len(services))
    return services, nil
}

// Close закрывает соединение с MongoDB
func (r *MongoDBRepository) Close(ctx context.Context) error {
    return r.client.Disconnect(ctx)
}

// GetCollection возвращает native mongo collection для прямых операций
func (r *MongoDBRepository) GetCollection() *mongo.Collection {
    return r.collection
}

// GetServices — реализация интерфейса Repository: читает из MongoDB
func (r *MongoDBRepository) GetServices(ctx context.Context) (<-chan models.Service, <-chan error) {
    servicesChan := make(chan models.Service, 100)
    errChan := make(chan error, 1)

    go func() {
        defer close(servicesChan)
        defer close(errChan)

        cursor, err := r.collection.Find(ctx, bson.M{})
        if err != nil {
            errChan <- err
            return
        }
        defer cursor.Close(ctx)

        for cursor.Next(ctx) {
            var svc models.Service
            if err := cursor.Decode(&svc); err != nil {
                log.Printf("⚠️  Пропускаем документ: %v", err)
                continue
            }
            select {
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            case servicesChan <- svc:
            }
        }

        if err := cursor.Err(); err != nil {
            errChan <- err
        }
    }()

    return servicesChan, errChan
}

// Пока не сохраняем результаты — можно заглушку
func (r *MongoDBRepository) SaveResults(ctx context.Context, results <-chan models.Result) <-chan error {
    errChan := make(chan error, 1)
    go func() {
        defer close(errChan)
        // Просто потребляем канал, ничего не делаем
        for {
            select {
            case _, ok := <-results:
                if !ok {
                    return
                }
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            }
        }
    }()
    return errChan
}