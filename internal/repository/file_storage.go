package repository


import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mdemidenko/monitoring-platform/internal/models"
)

type Repository interface {
	GetServices() ([]models.Service, error)
	SaveResults(results []models.Result) error
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

func (r *repository) GetServices() ([]models.Service, error) {
	data, err := os.ReadFile(r.inputFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %w", err)
	}

	var services []models.Service
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	return services, nil
}

func (r *repository) SaveResults(results []models.Result) error {
    file, err := os.Create(r.outputFile)
    if err != nil {
        return fmt.Errorf("ошибка создания файла: %w", err)
    }
    
    // Записываем данные
    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")
    
    encodeErr := encoder.Encode(results)
    
    // Закрываем файл и проверяем ошибку
    closeErr := file.Close()
    
    // Возвращаем первую возникшую ошибку
    if encodeErr != nil {
        return fmt.Errorf("ошибка записи JSON: %w", encodeErr)
    }
    if closeErr != nil {
        return fmt.Errorf("ошибка закрытия файла: %w", closeErr)
    }
    
    return nil
}