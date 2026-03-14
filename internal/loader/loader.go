package loader

import (
    "context"
    "encoding/json"
    "log"
    "os"
	"time"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// Константы фильтрации
const (
    TargetDeprecatedDate = "0001-01-01T00:00:00Z"
    TargetBusinessLine   = "Управление разработки решений для бизнеса и Центр оптимизации процессов поставки"
)

// getExistingIDs — получает все id из коллекции
func getExistingIDs(ctx context.Context, collection *mongo.Collection) (map[interface{}]bool, error) {
    cursor, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"id": 1}))
    if err != nil {
        return nil, err
    }
    defer func() {
    if err := cursor.Close(ctx); err != nil {
        log.Printf("Failed to close cursor: %v", err)
    }
    }()

    existing := make(map[interface{}]bool)

    log.Printf("🔍 Начало чтения существующих ID из MongoDB...")

    var count int
    for cursor.Next(ctx) {
    var result struct {
        ID interface{} `bson:"id"`
    }
    if err := cursor.Decode(&result); err != nil {
        return nil, err
    }

    normalizedID := result.ID
    switch v := result.ID.(type) {
    case float64:
        if float64(int(v)) == v {
            normalizedID = int(v)
        }
    case int32:
        normalizedID = int(v)
    case int64:
        normalizedID = int(v)
    }

    existing[normalizedID] = true
    log.Printf("📥 Найден в БД: id=%v → нормализовано: %v", result.ID, normalizedID)
    count++
	}

    if err := cursor.Err(); err != nil {
        return nil, err
    }

    log.Printf("✅ Загружено %d существующих ID из MongoDB", count)
    return existing, nil
}

// LoadAndFilterServices загружает, фильтрует, вставляет и возвращает статистику
func LoadAndFilterServices(ctx context.Context, collection *mongo.Collection, filePath string) (int, error) {
    startTime := time.Now()

    data, err := os.ReadFile(filePath)
    if err != nil {
        return 0, err
    }

    var rawServices []map[string]interface{}
    if err := json.Unmarshal(data, &rawServices); err != nil {
        return 0, err
    }

    log.Printf("📄 Прочитано %d сервисов из JSON", len(rawServices))

    existing, err := getExistingIDs(ctx, collection)
    if err != nil {
        return 0, err
    }
    log.Printf("📊 Уже есть в БД: %d сервисов", len(existing))

    var newResults []interface{}

	for _, r := range rawServices {
    // Извлекаем ID
    idRaw := r["id"]

	normalizedID := idRaw
    if floatID, ok := idRaw.(float64); ok && float64(int(floatID)) == floatID {
        normalizedID = int(floatID)
    }
    
    // Приводим к int, если возможно, но сохраняем тип как в JSON
    // var id interface{} = idRaw

    // Если float64 и целое — можно оставить как float64, потому что в existing он такой же
    // Но если в existing — int, а тут float64 — не совпадёт
    // Лучше: не менять тип, а использовать как есть

    name, _ := r["name"].(string)
    tenant, _ := r["tenant"].(string)
    deprecatedDate, _ := r["deprecated_date"].(string)
    businessLine, _ := r["businessLine"].(string)
	deployPlatform, _ := r["deployPlatform"].(string)

    // Фильтр
    if deprecatedDate != TargetDeprecatedDate || businessLine != TargetBusinessLine || deployPlatform != "k8s"{
        continue
    }

    // 🔍 Проверяем: есть ли уже в БД
    if _, exists := existing[normalizedID]; exists {
        log.Printf("🔁 Уже в БД: id=%v, пропускаем", idRaw)
        continue
    }

    // Подходит — создаём Result
    result := models.Result{
        ID:     idRaw,   // сохраняем как есть
        Name:   name,
        Tenant: tenant,
    }

    newResults = append(newResults, result)
	}	

    if len(newResults) > 0 {
        _, err = collection.InsertMany(ctx, newResults)
        if err != nil {
            return 0, err
        }
    }

    // Выводим статистику
    finalCount := len(newResults)
    elapsed := time.Since(startTime)

    log.Printf("========================================")
    log.Printf("ОБРАБОТКА ЗАВЕРШЕНА")
    log.Printf("Всего времени: %v", elapsed)
    log.Printf("Найдено подходящих сервисов: %d", finalCount)
    log.Printf("Скорость обработки: %.2f записей/сек", float64(finalCount)/elapsed.Seconds())
    log.Printf("========================================")

    return finalCount, nil
}