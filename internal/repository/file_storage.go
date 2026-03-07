package repository

import (
    "context"
    "encoding/json"
    "log"
    "os"
	"fmt"

    "github.com/mdemidenko/monitoring-platform/internal/models"
	"go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
    GetServices(ctx context.Context) (<-chan models.Service, <-chan error)
    SaveResults(ctx context.Context, results <-chan models.Result) <-chan error
    // GetCollection() *mongo.Collection
}

type repository struct {
    inputFile    string
    outputFile   string
    mongoURI     string
    dbName       string
    collectionName string
}

func NewRepository(inputFile, mongoURI, dbName, collectionName string) *repository {
    return &repository{
        inputFile:      inputFile,
        mongoURI:       mongoURI,
        dbName:         dbName,
        collectionName: collectionName,
    }
}

// GetServices читает сервисы и отправляет в канал
func (r *repository) GetServices(ctx context.Context) (<-chan models.Service, <-chan error) {
	servicesChan := make(chan models.Service, 100)
	errChan := make(chan error, 1)

	// Проверяем контекст СРАЗУ (синхронно)
	if ctx.Err() != nil {
		go func() {
			defer close(servicesChan)
			defer close(errChan)
			errChan <- ctx.Err()
		}()
		return servicesChan, errChan
	}

	go func() {
		defer close(servicesChan)
		defer close(errChan)

		data, err := os.ReadFile(r.inputFile)
		if err != nil {
			errChan <- fmt.Errorf("ошибка чтения файла: %w", err)
			return
		}

		var services []models.Service
		if err := json.Unmarshal(data, &services); err != nil {
			errChan <- fmt.Errorf("ошибка парсинга JSON: %w", err)
			return
		}

		// Отправляем сервисы в канал с проверкой контекста
		for _, service := range services {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case servicesChan <- service:
			}
		}
	}()

	return servicesChan, errChan
}

// SaveResults сохраняет результаты из канала в mongodb
func (r *repository) SaveResults(ctx context.Context, results <-chan models.Result) <-chan error {
    errChan := make(chan error, 1)

    if ctx.Err() != nil {
        go func() {
            defer close(errChan)
            errChan <- ctx.Err()
        }()
        return errChan
    }

    go func() {
        defer close(errChan)

        // Подключаемся к MongoDB
        client, err := mongo.Connect(ctx, options.Client().ApplyURI(r.mongoURI))
        if err != nil {
            errChan <- err
            return
        }
        defer client.Disconnect(ctx)

        collection := client.Database(r.dbName).Collection(r.collectionName)

        // Собираем результаты
        var docs []interface{}
        for {
            select {
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            case result, ok := <-results:
                if !ok {
                    // Канал закрыт — сохраняем всё
                    if len(docs) > 0 {
                        _, err := collection.InsertMany(ctx, docs)
                        if err != nil {
                            errChan <- err
                            return
                        }
                    }
                    log.Printf("✅ Сохранено %d результатов в MongoDB", len(docs))
                    return
                }
                docs = append(docs, result)
            }
        }
    }()

    return errChan
}

// saveToFile - внутренний метод сохранения
func (r *repository) saveToFile(results []models.Result) error {
    if len(results) == 0 {
        return nil
    }
    
    file, err := os.Create(r.outputFile)
    if err != nil {
        return fmt.Errorf("ошибка создания файла: %w", err)
    }
    defer func() {
        if closeErr := file.Close(); closeErr != nil {
            log.Printf("Ошибка при закрытии файла %s: %v", r.outputFile, closeErr)
        }
    }()

    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(results); err != nil {
        return fmt.Errorf("ошибка записи JSON: %w", err)
    }
    
    return nil
}