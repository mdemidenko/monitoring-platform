package main

import (
	"fmt"

	"github.com/mdemidenko/monitoring-platform/config"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
	"github.com/mdemidenko/monitoring-platform/internal/monitor"
)

func main() {
	cfg := config.FileLoadConfig()

	// Инициализация зависимостей
	repo := repository.NewRepository(cfg.InputFile, cfg.OutputFile)
	svc := monitor.New(repo)


	// Вызов бизнес-логики
	results, err := svc.FilterServices()
	if err != nil {
		fmt.Println("Ошибка фильтрации:", err)
		return
	}

	// Сохранение результата
	if err := repo.SaveResults(results); err != nil {
		fmt.Println("Ошибка сохранения:", err)
		return
	}

	// Вывод
	fmt.Printf("Найдено подходящих сервисов: %d\n", len(results))
	for i, svc := range results {
		fmt.Printf("  %d. ID: %d, Name: %s, Tenant: %s\n", i+1, svc.ID, svc.Name, svc.Tenant)
	}
}