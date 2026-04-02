package repository

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/stretchr/testify/require"
)

func TestNewRepository(t *testing.T) {
    tests := []struct {
        name      string
        inputFile string
        wantErr   bool
    }{
        {
            name:      "valid input file",
            inputFile: "test_input.json",
            wantErr:   false,
        },
        {
            name:      "empty input file",
            inputFile: "",
            wantErr:   false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := NewRepository(tt.inputFile, "mongodb://localhost:27017", "testdb", "results")

            if repo == nil {
                t.Fatal("Expected repository instance, got nil")
            }
        })
    }
}

func TestRepository_GetServices_Success(t *testing.T) {
    // Создаем временный файл с тестовыми данными
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")

    services := []models.Service{
        {
            ID:             1,
            Name:           "Service 1",
            Tenant:         "tenant1",
            DeprecatedDate: "2024-01-01",
            BusinessLine:   "line1",
        },
        {
            ID:             2,
            Name:           "Service 2",
            Tenant:         "tenant2",
            DeprecatedDate: "2024-02-01",
            BusinessLine:   "line2",
        },
    }

    data, err := json.Marshal(services)
    require.NoError(t, err, "Failed to marshal test data")

    err = os.WriteFile(inputFile, data, 0644)
    require.NoError(t, err, "Failed to create test file")

    // Создаем репозиторий
    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx := context.Background()
    servicesChan, errChan := repo.GetServices(ctx)

    // Собираем сервисы из канала
    var receivedServices []models.Service
    for service := range servicesChan {
        receivedServices = append(receivedServices, service)
    }

    // Проверяем ошибки
    err = <-errChan
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }

    // Проверяем количество
    if len(receivedServices) != len(services) {
        t.Errorf("Expected %d services, got %d", len(services), len(receivedServices))
    }

    // Проверяем данные
    for i, expected := range services {
        actual := receivedServices[i]
        if actual.ID != expected.ID {
            t.Errorf("Service %d: Expected ID %d, got %d", i, expected.ID, actual.ID)
        }
        if actual.Name != expected.Name {
            t.Errorf("Service %d: Expected Name %s, got %s", i, expected.Name, actual.Name)
        }
        if actual.Tenant != expected.Tenant {
            t.Errorf("Service %d: Expected Tenant %s, got %s", i, expected.Tenant, actual.Tenant)
        }
    }

    // Проверяем, что каналы закрыты
    _, ok := <-servicesChan
    if ok {
        t.Error("servicesChan should be closed")
    }
    _, ok = <-errChan
    if ok {
        t.Error("errChan should be closed")
    }
}

func TestRepository_GetServices_EmptyFile(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "empty.json")

    err := os.WriteFile(inputFile, []byte(""), 0644)
    require.NoError(t, err, "Failed to create empty file")

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx := context.Background()
    servicesChan, errChan := repo.GetServices(ctx)

    // Канал сервисов должен быть закрыт
    _, ok := <-servicesChan
    if ok {
        t.Error("servicesChan should be closed for empty file")
    }

    // Должна быть ошибка парсинга
    select {
    case err := <-errChan:
        if err == nil {
            t.Error("Expected JSON parsing error for empty file")
        }
        if err.Error()[:len("ошибка парсинга JSON")] != "ошибка парсинга JSON" {
            t.Errorf("Expected error starting with 'ошибка парсинга JSON', got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected error but got timeout")
    }
}

func TestRepository_GetServices_FileNotFound(t *testing.T) {
    tempDir := t.TempDir()
    nonExistentFile := filepath.Join(tempDir, "nonexistent.json")

    repo := NewRepository(nonExistentFile, "mongodb://localhost:27017", "testdb", "results")

    ctx := context.Background()
    servicesChan, errChan := repo.GetServices(ctx)

    _, ok := <-servicesChan
    if ok {
        t.Error("servicesChan should be closed for non-existent file")
    }

    select {
    case err := <-errChan:
        if err == nil {
            t.Error("Expected error for non-existent file")
        }
        if err.Error()[:len("ошибка чтения файла")] != "ошибка чтения файла" {
            t.Errorf("Expected error starting with 'ошибка чтения файла', got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected error but got timeout")
    }
}

func TestRepository_GetServices_InvalidJSON(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "invalid.json")

    err := os.WriteFile(inputFile, []byte("{invalid json}"), 0644)
    require.NoError(t, err, "Failed to create test file")

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx := context.Background()
    servicesChan, errChan := repo.GetServices(ctx)

    _, ok := <-servicesChan
    if ok {
        t.Error("servicesChan should be closed for invalid JSON")
    }

    select {
    case err := <-errChan:
        if err == nil {
            t.Error("Expected JSON parsing error")
        }
        if err.Error()[:len("ошибка парсинга JSON")] != "ошибка парсинга JSON" {
            t.Errorf("Expected error starting with 'ошибка парсинга JSON', got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected error but got timeout")
    }
}

func TestRepository_GetServices_ContextCancelledBeforeStart(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")

    services := []models.Service{{ID: 1, Name: "Test"}}
    data, _ := json.Marshal(services)
    err := os.WriteFile(inputFile, data, 0644)
    require.NoError(t, err, "Failed to write input file")

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    servicesChan, errChan := repo.GetServices(ctx)

    select {
    case err := <-errChan:
        if err != context.Canceled {
            t.Errorf("Expected context.Canceled error, got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected context error but got timeout")
    }

    _, ok := <-servicesChan
    if ok {
        t.Error("servicesChan should be closed after context cancellation")
    }
}

func TestRepository_GetServices_ContextCancelledDuringProcessing(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")

    services := []models.Service{{ID: 1, Name: "Service 1"}}
    data, err := json.Marshal(services)
    require.NoError(t, err)
    err = os.WriteFile(inputFile, data, 0644)
    require.NoError(t, err)

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx, cancel := context.WithTimeout(context.Background(), 0)
    defer cancel()

    servicesChan, errChan := repo.GetServices(ctx)

    select {
    case err := <-errChan:
        if err != context.DeadlineExceeded {
            t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected DeadlineExceeded error but got timeout")
    }

    _, ok := <-servicesChan
    if ok {
        t.Error("servicesChan should be closed")
    }
    _, ok = <-errChan
    if ok {
        t.Error("errChan should be closed")
    }
}

func TestRepository_SaveResults_Success(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")
    _ = os.WriteFile(inputFile, []byte(`[{"id":1,"name":"Test"}]`), 0644)

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx := context.Background()
    resultsChan := make(chan models.Result, 3)

    results := []models.Result{
        {ID: 1, Name: "Service 1", Tenant: "tenant1"},
        {ID: 2, Name: "Service 2", Tenant: "tenant2"},
        {ID: 3, Name: "Service 3", Tenant: "tenant3"},
    }

    for _, result := range results {
        resultsChan <- result
    }
    close(resultsChan)

    errChan := repo.SaveResults(ctx, resultsChan)

    select {
    case err := <-errChan:
        if err != nil {
            t.Errorf("Unexpected error: %v", err)
        }
    case <-time.After(3 * time.Second): // Увеличено для MongoDB
        t.Error("SaveResults timed out")
    }

    _, ok := <-errChan
    if ok {
        t.Error("errChan should be closed")
    }
}

func TestRepository_SaveResults_EmptyResults(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")
    _ = os.WriteFile(inputFile, []byte(`[]`), 0644)

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx := context.Background()
    resultsChan := make(chan models.Result)
    close(resultsChan)

    errChan := repo.SaveResults(ctx, resultsChan)

    select {
    case err := <-errChan:
        if err != nil {
            t.Errorf("Unexpected error for empty results: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("SaveResults timed out")
    }

    _, ok := <-errChan
    if ok {
        t.Error("errChan should be closed")
    }
}

func TestRepository_SaveResults_ContextCancelledBeforeStart(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")
    _ = os.WriteFile(inputFile, []byte(`[]`), 0644)

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    resultsChan := make(chan models.Result)
    close(resultsChan)

    errChan := repo.SaveResults(ctx, resultsChan)

    select {
    case err := <-errChan:
        if err != context.Canceled {
            t.Errorf("Expected context.Canceled error, got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected context error but got timeout")
    }

    _, ok := <-errChan
    if ok {
        t.Error("errChan should be closed")
    }
}

func TestRepository_SaveResults_ContextCancelledDuringProcessing(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")
    _ = os.WriteFile(inputFile, []byte(`[]`), 0644)

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx, cancel := context.WithCancel(context.Background())
    resultsChan := make(chan models.Result)

    errChan := repo.SaveResults(ctx, resultsChan)

    // Имитация отправки результатов
    go func() {
        for i := 0; i < 5; i++ {
            select {
            case <-ctx.Done():
                return
            case resultsChan <- models.Result{ID: i + 1, Name: string(rune('A'+i))}:
                time.Sleep(10 * time.Millisecond)
            }
        }
    }()

    time.Sleep(30 * time.Millisecond)
    cancel()

    select {
    case err := <-errChan:
        if err != context.Canceled {
            t.Errorf("Expected context.Canceled error, got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Expected context error but got timeout")
    }

    close(resultsChan)
}

func TestRepository_Integration_CompleteFlow(t *testing.T) {
    tempDir := t.TempDir()
    inputFile := filepath.Join(tempDir, "services.json")

    services := []models.Service{
        {ID: 1, Name: "Service Alpha", Tenant: "tenant1", DeprecatedDate: "2024-01-01"},
        {ID: 2, Name: "Service Beta", Tenant: "tenant2", DeprecatedDate: ""},
        {ID: 3, Name: "Service Gamma", Tenant: "tenant3", DeprecatedDate: "2024-03-01"},
    }

    data, err := json.Marshal(services)
    require.NoError(t, err)
    err = os.WriteFile(inputFile, data, 0644)
    require.NoError(t, err)

    repo := NewRepository(inputFile, "mongodb://localhost:27017", "testdb", "results")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    servicesChan, getErrChan := repo.GetServices(ctx)

    resultsChan := make(chan models.Result, len(services))
    go func() {
        for service := range servicesChan {
            if service.DeprecatedDate == "" {
                resultsChan <- models.Result{
                    ID:     service.ID,
                    Name:   service.Name,
                    Tenant: service.Tenant,
                }
            }
        }
        close(resultsChan)
    }()

    saveErrChan := repo.SaveResults(ctx, resultsChan)

    select {
    case err := <-getErrChan:
        if err != nil {
            t.Errorf("GetServices failed: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("GetServices timed out")
    }

    select {
    case err := <-saveErrChan:
        if err != nil {
            t.Errorf("SaveResults failed: %v", err)
        }
    case <-time.After(3 * time.Second):
        t.Error("SaveResults timed out")
    }
}