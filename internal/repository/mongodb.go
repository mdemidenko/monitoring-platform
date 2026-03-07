package repository

import (
    "context"
    "log"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

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