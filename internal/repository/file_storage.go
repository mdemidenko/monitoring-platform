package repository

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "github.com/mdemidenko/monitoring-platform/internal/models"
)

type Repository interface {
    GetServices(ctx context.Context) (<-chan models.Service, <-chan error)
    SaveResults(ctx context.Context, results <-chan models.Result) <-chan error
}

type repository struct {
    inputFile  string
    outputFile string
}

func NewRepository(inputFile, outputFile string) Repository {
    return &repository{
        inputFile:  inputFile,
        outputFile: outputFile,
    }
}

// GetServices читает сервисы и отправляет в канал
func (r *repository) GetServices(ctx context.Context) (<-chan models.Service, <-chan error) {
    servicesChan := make(chan models.Service, 100)
    errChan := make(chan error, 1)

    go func() {
        defer close(servicesChan)
        defer close(errChan)

        // Проверяем контекст перед началом чтения
        if ctx.Err() != nil {
            errChan <- ctx.Err()
            return
        }

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

// SaveResults сохраняет результаты из канала в файл
func (r *repository) SaveResults(ctx context.Context, results <-chan models.Result) <-chan error {
    errChan := make(chan error, 1)

    go func() {
        defer close(errChan)

        var allResults []models.Result
        
        for {
            select {
            case <-ctx.Done():
                // Пытаемся сохранить то, что успели собрать
                if len(allResults) > 0 {
                    if err := r.saveToFile(allResults); err != nil {
                        errChan <- fmt.Errorf("ошибка сохранения при отмене: %w", err)
                        return
                    }
                }
                errChan <- ctx.Err()
                return
                
            case result, ok := <-results:
                if !ok {
                    // Канал закрыт, сохраняем все результаты
                    if err := r.saveToFile(allResults); err != nil {
                        errChan <- err
                    }
                    return
                }
                allResults = append(allResults, result)
            }
        }
    }()

    return errChan
}

// saveToFile - внутренний метод сохранения
func (r *repository) saveToFile(results []models.Result) error {
    if len(results) == 0 {
        return nil // ничего не сохраняем
    }
    
    file, err := os.Create(r.outputFile)
    if err != nil {
        return fmt.Errorf("ошибка создания файла: %w", err)
    }

    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")

    if err := encoder.Encode(results); err != nil {
        return fmt.Errorf("ошибка записи JSON: %w", err)
    }

    return nil
}