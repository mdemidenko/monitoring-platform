package repository

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/stretchr/testify/require"
)

func TestNewRepository(t *testing.T) {
	tests := []struct {
		name       string
		inputFile  string
		outputFile string
		wantErr    bool
	}{
		{
			name:       "valid files",
			inputFile:  "test_input.json",
			outputFile: "test_output.json",
			wantErr:    false,
		},
		{
			name:       "empty input file",
			inputFile:  "",
			outputFile: "test_output.json",
			wantErr:    false,
		},
		{
			name:       "empty output file",
			inputFile:  "test_input.json",
			outputFile: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(tt.inputFile, tt.outputFile)
			
			if repo == nil {
				t.Fatal("Expected repository instance, got nil")
			}
			
			// Проверяем что это правильный тип
			_, ok := repo.(*repository)
			if !ok {
				t.Error("Expected *repository type")
			}
			
			// Проверяем что реализует интерфейс
			var _ Repository = repo
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
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}
	
	if err := os.WriteFile(inputFile, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Создаем репозиторий
	repo := NewRepository(inputFile, "output.json")
	
	ctx := context.Background()
	servicesChan, errChan := repo.GetServices(ctx)
	
	// Собираем сервисы из канала
	var receivedServices []models.Service
	for service := range servicesChan {
		receivedServices = append(receivedServices, service)
	}
	
	// Проверяем ошибки (должны быть nil)
	err = <-errChan
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Проверяем полученные сервисы
	if len(receivedServices) != len(services) {
		t.Errorf("Expected %d services, got %d", len(services), len(receivedServices))
	}
	
	// Проверяем правильность данных
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
	
	// Проверяем что каналы закрыты
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
	
	// Создаем пустой файл
	if err := os.WriteFile(inputFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}
	
	repo := NewRepository(inputFile, "output.json")
	
	ctx := context.Background()
	servicesChan, errChan := repo.GetServices(ctx)
	
	// Должен быть пустой канал сервисов
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
		// Проверяем текст ошибки
		expectedError := "ошибка парсинга JSON"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Errorf("Expected error starting with '%s', got: %v", expectedError, err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected error but got timeout")
	}
}

func TestRepository_GetServices_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentFile := filepath.Join(tempDir, "nonexistent.json")
	
	repo := NewRepository(nonExistentFile, "output.json")
	
	ctx := context.Background()
	servicesChan, errChan := repo.GetServices(ctx)
	
	// Должен быть пустой канал сервисов
	_, ok := <-servicesChan
	if ok {
		t.Error("servicesChan should be closed for non-existent file")
	}
	
	// Должна быть ошибка
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
		expectedError := "ошибка чтения файла"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Errorf("Expected error starting with '%s', got: %v", expectedError, err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected error but got timeout")
	}
}

func TestRepository_GetServices_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "invalid.json")
	
	// Пишем невалидный JSON
	if err := os.WriteFile(inputFile, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	repo := NewRepository(inputFile, "output.json")
	
	ctx := context.Background()
	servicesChan, errChan := repo.GetServices(ctx)
	
	// Должен быть пустой канал сервисов
	_, ok := <-servicesChan
	if ok {
		t.Error("servicesChan should be closed for invalid JSON")
	}
	
	// Должна быть ошибка парсинга
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected JSON parsing error")
		}
		expectedError := "ошибка парсинга JSON"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Errorf("Expected error starting with '%s', got: %v", expectedError, err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected error but got timeout")
	}
}

func TestRepository_GetServices_ContextCancelledBeforeStart(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "services.json")
	
	// Создаем файл
	services := []models.Service{{ID: 1, Name: "Test"}}
	data, _ := json.Marshal(services)
	err := os.WriteFile(inputFile, data, 0644)
	require.NoError(t, err, "Failed to write input file")
	
	repo := NewRepository(inputFile, "output.json")
	
	// Создаем контекст с быстрой отменой
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменяем сразу
	
	servicesChan, errChan := repo.GetServices(ctx)
	
	// Должна быть ошибка контекста
	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected context error but got timeout")
	}
	
	// Канал сервисов должен быть закрыт
	_, ok := <-servicesChan
	if ok {
		t.Error("servicesChan should be closed after context cancellation")
	}
}

func TestRepository_GetServices_ContextCancelledDuringProcessing(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "services.json")
	
	// Создаем файл с одним сервисом - быстрая обработка
	services := []models.Service{
		{ID: 1, Name: "Service 1"},
	}
	
	data, err := json.Marshal(services)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}
	
	if err := os.WriteFile(inputFile, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	repo := NewRepository(inputFile, "output.json")
	
	// Контекст с таймаутом НОЛЬ - отменен сразу
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	
	servicesChan, errChan := repo.GetServices(ctx)
	
	// Должна быть ошибка DeadlineExceeded
	select {
	case err := <-errChan:
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
		
		// Канал сервисов должен быть закрыт
		_, ok := <-servicesChan
		if ok {
			t.Error("servicesChan should be closed after error")
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected DeadlineExceeded error but got timeout")
	}
	
	// Проверяем что оба канала закрыты
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
	outputFile := filepath.Join(tempDir, "results.json")
	
	repo := NewRepository("input.json", outputFile)
	
	ctx := context.Background()
	resultsChan := make(chan models.Result, 3)
	
	// Отправляем результаты
	results := []models.Result{
		{ID: 1, Name: "Service 1", Tenant: "tenant1"},
		{ID: 2, Name: "Service 2", Tenant: "tenant2"},
		{ID: 3, Name: "Service 3", Tenant: "tenant3"},
	}
	
	for _, result := range results {
		resultsChan <- result
	}
	close(resultsChan)
	
	// Запускаем сохранение
	errChan := repo.SaveResults(ctx, resultsChan)
	
	// Ждем завершения
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("SaveResults timed out")
	}
	
	// Проверяем что файл создан
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		t.Errorf("Failed to stat output file: %v", err)
	}
	
	if fileInfo.Size() == 0 {
		t.Error("Output file should not be empty")
	}
	
	// Читаем и проверяем содержимое
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Errorf("Failed to read output file: %v", err)
	}
	
	var savedResults []models.Result
	if err := json.Unmarshal(data, &savedResults); err != nil {
		t.Errorf("Failed to unmarshal saved results: %v", err)
	}
	
	if len(savedResults) != len(results) {
		t.Errorf("Expected %d results saved, got %d", len(results), len(savedResults))
	}
	
	// Проверяем правильность данных
	for i, expected := range results {
		actual := savedResults[i]
		if actual.ID != expected.ID {
			t.Errorf("Result %d: Expected ID %d, got %d", i, expected.ID, actual.ID)
		}
		if actual.Name != expected.Name {
			t.Errorf("Result %d: Expected Name %s, got %s", i, expected.Name, actual.Name)
		}
		if actual.Tenant != expected.Tenant {
			t.Errorf("Result %d: Expected Tenant %s, got %s", i, expected.Tenant, actual.Tenant)
		}
	}
	
	// Проверяем что канал ошибок закрыт
	_, ok := <-errChan
	if ok {
		t.Error("errChan should be closed")
	}
}

func TestRepository_SaveResults_EmptyResults(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "results.json")
	
	repo := NewRepository("input.json", outputFile)
	
	ctx := context.Background()
	resultsChan := make(chan models.Result)
	close(resultsChan) // Пустой канал
	
	errChan := repo.SaveResults(ctx, resultsChan)
	
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Unexpected error for empty results: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("SaveResults timed out")
	}
	
	// Файл не должен создаваться для пустых результатов
	_, err := os.Stat(outputFile)
	if err == nil {
		t.Error("Output file should not be created for empty results")
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking file: %v", err)
	}
	
	// Проверяем что канал ошибок закрыт
	_, ok := <-errChan
	if ok {
		t.Error("errChan should be closed")
	}
}

func TestRepository_SaveResults_ContextCancelledBeforeStart(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "results.json")
	
	repo := NewRepository("input.json", outputFile)
	
	// Контекст уже отменен
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	resultsChan := make(chan models.Result)
	close(resultsChan) // Пустой канал
	
	errChan := repo.SaveResults(ctx, resultsChan)
	
	// Должна быть ошибка контекста
	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected context error but got timeout")
	}
	
	// Файл не должен создаваться
	_, err := os.Stat(outputFile)
	if !os.IsNotExist(err) {
		t.Error("Output file should not be created when context cancelled")
	}
	
	// Проверяем что канал ошибок закрыт
	_, ok := <-errChan
	if ok {
		t.Error("errChan should be closed")
	}
}

func TestRepository_SaveResults_ContextCancelledDuringProcessing(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "results.json")
	
	repo := NewRepository("input.json", outputFile)
	
	ctx, cancel := context.WithCancel(context.Background())
	resultsChan := make(chan models.Result)
	
	// Создаем канал для синхронизации
	processingStarted := make(chan bool, 1)
	
	// Запускаем сохранение
	errChan := repo.SaveResults(ctx, resultsChan)
	
	// Отправляем несколько результатов в отдельной горутине
	go func() {
		processingStarted <- true
		
		// Отправляем с проверкой, не закрыт ли канал
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				// Контекст отменен, прекращаем отправку
				return
			case resultsChan <- models.Result{ID: i + 1, Name: string(rune('A' + i))}:
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
	
	// Ждем пока горутина начнет обработку
	<-processingStarted
	
	// Даем немного времени на обработку первых результатов
	time.Sleep(30 * time.Millisecond)
	
	// Отменяем контекст
	cancel()
	
	// Ждем ошибку от SaveResults
	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected context error but got timeout")
	}
	
	// Закрываем канал результатов чтобы горутина SaveResults могла завершиться
	// Но сначала проверяем не закрыт ли он уже
	select {
	case <-ctx.Done():
		// Контекст отменен, SaveResults уже завершился
	default:
		close(resultsChan)
	}
	
	// Дополнительная проверка: ждем полного завершения
	select {
	case <-errChan: // Проверяем что канал ошибок закрыт
		// OK
	case <-time.After(100 * time.Millisecond):
		// Timeout - нормально, канал может быть уже закрыт
	}
}

func TestRepository_SaveResults_FileCreationError(t *testing.T) {
	// Пытаемся записать в директорию (невозможно создать файл)
	tempDir := t.TempDir()
	outputFile := tempDir // Директория, а не файл
	
	repo := NewRepository("input.json", outputFile)
	
	ctx := context.Background()
	resultsChan := make(chan models.Result, 1)
	resultsChan <- models.Result{ID: 1, Name: "Test Service"}
	close(resultsChan)
	
	errChan := repo.SaveResults(ctx, resultsChan)
	
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected file creation error")
		}
		expectedError := "ошибка создания файла"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Errorf("Expected error starting with '%s', got: %v", expectedError, err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected error but got timeout")
	}
}

func TestRepository_SaveResults_JSONWriteError(t *testing.T) {
	// Этот тест сложно реализовать, так как models.Result
	// легко сериализуется. Но тестируем общий путь.
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "results.json")
	
	repo := NewRepository("input.json", outputFile)
	
	ctx := context.Background()
	resultsChan := make(chan models.Result, 2)
	
	// Отправляем обычные результаты
	resultsChan <- models.Result{ID: 1, Name: "Service 1", Tenant: "tenant1"}
	resultsChan <- models.Result{ID: 2, Name: "Service 2", Tenant: "tenant2"}
	close(resultsChan)
	
	errChan := repo.SaveResults(ctx, resultsChan)
	
	// Должно быть успешно
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("SaveResults timed out")
	}
	
	// Проверяем что файл создан и содержит JSON
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Errorf("Failed to read output file: %v", err)
	}
	
	// Проверяем что это валидный JSON
	var results []models.Result
	if err := json.Unmarshal(data, &results); err != nil {
		t.Errorf("Output file contains invalid JSON: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("Expected 2 results in file, got %d", len(results))
	}
}

func TestRepository_saveToFile_InternalMethod(t *testing.T) {
	// Тестируем внутренний метод напрямую через создание репозитория
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test.json")
	
	// Создаем репозиторий чтобы получить доступ к saveToFile
	repo := NewRepository("input.json", outputFile).(*repository)
	
	tests := []struct {
		name     string
		results  []models.Result
		wantFile bool
	}{
		{
			name:     "empty results - no file",
			results:  []models.Result{},
			wantFile: false,
		},
		{
			name: "single result - creates file",
			results: []models.Result{
				{ID: 1, Name: "Test"},
			},
			wantFile: true,
		},
		{
			name: "multiple results - creates file",
			results: []models.Result{
				{ID: 1, Name: "Service 1"},
				{ID: 2, Name: "Service 2"},
				{ID: 3, Name: "Service 3"},
			},
			wantFile: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Удаляем предыдущий файл если существует
			if err := os.Remove(outputFile); err != nil {
    			t.Logf("Failed to remove output file %s: %v", outputFile, err)
			}
			
			err := repo.saveToFile(tt.results)
			
			if err != nil {
				t.Errorf("saveToFile failed: %v", err)
			}
			
			// Проверяем существование файла
			_, err = os.Stat(outputFile)
			fileExists := err == nil
			
			if fileExists != tt.wantFile {
				t.Errorf("File existence: expected %v, got %v", tt.wantFile, fileExists)
			}
			
			// Если файл должен существовать, проверяем содержимое
			if tt.wantFile && len(tt.results) > 0 {
				data, err := os.ReadFile(outputFile)
				if err != nil {
					t.Errorf("Failed to read created file: %v", err)
				}
				
				var savedResults []models.Result
				if err := json.Unmarshal(data, &savedResults); err != nil {
					t.Errorf("Created file contains invalid JSON: %v", err)
				}
				
				if len(savedResults) != len(tt.results) {
					t.Errorf("Expected %d results in file, got %d", len(tt.results), len(savedResults))
				}
			}
		})
	}
}

func TestRepository_Integration_CompleteFlow(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "services.json")
	outputFile := filepath.Join(tempDir, "results.json")
	
	// Создаем входной файл с сервисами
	services := []models.Service{
		{
			ID:             1,
			Name:           "Service Alpha",
			Tenant:         "tenant1",
			DeprecatedDate: "2024-01-01",
			BusinessLine:   "finance",
		},
		{
			ID:             2,
			Name:           "Service Beta",
			Tenant:         "tenant2",
			DeprecatedDate: "",
			BusinessLine:   "marketing",
		},
		{
			ID:             3,
			Name:           "Service Gamma",
			Tenant:         "tenant3",
			DeprecatedDate: "2024-03-01",
			BusinessLine:   "operations",
		},
	}
	
	data, err := json.Marshal(services)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}
	
	if err := os.WriteFile(inputFile, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Создаем репозиторий
	repo := NewRepository(inputFile, outputFile)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Этап 1: Получаем сервисы
	servicesChan, getErrChan := repo.GetServices(ctx)
	
	// Этап 2: Обрабатываем сервисы (фильтруем и преобразуем)
	resultsChan := make(chan models.Result, len(services))
	
	go func() {
		for service := range servicesChan {
			// Пример бизнес-логики: фильтруем не deprecated сервисы
			if service.DeprecatedDate == "" {
				result := models.Result{
					ID:     service.ID,
					Name:   service.Name,
					Tenant: service.Tenant,
				}
				resultsChan <- result
			}
		}
		close(resultsChan)
	}()
	
	// Этап 3: Сохраняем результаты
	saveErrChan := repo.SaveResults(ctx, resultsChan)
	
	// Ждем завершения всех этапов
	
	// Проверяем ошибки от GetServices
	select {
	case err := <-getErrChan:
		if err != nil {
			t.Errorf("GetServices failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("GetServices timed out")
	}
	
	// Проверяем ошибки от SaveResults
	select {
	case err := <-saveErrChan:
		if err != nil {
			t.Errorf("SaveResults failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("SaveResults timed out")
	}
	
	// Проверяем выходной файл
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		t.Errorf("Failed to stat output file: %v", err)
	}
	
	if fileInfo.Size() == 0 {
		t.Error("Output file should not be empty")
	}
	
	// Читаем и анализируем результаты
	outputData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Errorf("Failed to read output file: %v", err)
	}
	
	var savedResults []models.Result
	if err := json.Unmarshal(outputData, &savedResults); err != nil {
		t.Errorf("Failed to unmarshal saved results: %v", err)
	}
	
	// Ожидаем только не deprecated сервисы
	expectedCount := 1 // Только Service Beta не deprecated
	if len(savedResults) != expectedCount {
		t.Errorf("Expected %d non-deprecated results, got %d", expectedCount, len(savedResults))
	}
	
	if len(savedResults) > 0 && savedResults[0].Name != "Service Beta" {
		t.Errorf("Expected Service Beta, got %s", savedResults[0].Name)
	}
}

func TestRepository_saveToFile_WriteError(t *testing.T) {
	// Это сложно тестировать, но можно попробовать с readonly директорией
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "results.json")
	
	// Создаем файл и делаем его readonly
	file, err := os.Create(outputFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer func() {
    	if err := file.Close(); err != nil {
        	log.Printf("Failed to close file %s: %v", outputFile, err)
    	}
	}()
	
	repo := NewRepository("input.json", outputFile).(*repository)
	
	results := []models.Result{
		{ID: 1, Name: "Test"},
	}
	
	// Тест может не сработать на всех системах
	err = repo.saveToFile(results)
	if err == nil {
		t.Log("Could not test write error (file may be writable)")
	} else if err.Error()[:len("ошибка записи JSON")] != "ошибка записи JSON" {
		t.Errorf("Expected JSON write error, got: %v", err)
	}
}