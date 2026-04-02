// internal/repository/loader.go

package repository

import (
    "context"
    "encoding/json"
    "log"
    "os"
	"fmt"

    "github.com/mdemidenko/monitoring-platform/internal/models"
	"go.mongodb.org/mongo-driver/mongo"
)

func LoadServicesFromJSON(ctx context.Context, collection *mongo.Collection, filePath string) error {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return err
    }

    var rawServices []map[string]interface{}
    if err := json.Unmarshal(data, &rawServices); err != nil {
        return err
    }

    var services []interface{}
    for _, r := range rawServices {
        id := r["id"]
        name, _ := r["name"].(string)
        tenant, _ := r["tenant"].(string)

        service := models.Service{
            ID:     id,
            Name:   name,
            Tenant: tenant,
        }
        services = append(services, service)
    }

    _, err = collection.InsertMany(ctx, services)
    if err != nil {
        return fmt.Errorf("ошибка вставки в MongoDB: %w", err)
    }

    log.Printf("✅ Успешно загружено %d сервисов в MongoDB", len(services))
    return nil
}