package monitor

import (
    "context"
    "sync"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
)

type Service interface {
    FilterServices(ctx context.Context, workers int) (<-chan models.Result, <-chan error)
    FilterServicesBatch(ctx context.Context, workers int) (<-chan models.Result, <-chan error)
}

const (
    TargetDeprecatedDate = "0001-01-01T00:00:00Z"
    TargetBusinessLine   = "Управление разработки решений для бизнеса и Центр оптимизации процессов поставки"
)

type service struct {
    repo repository.Repository
}

func New(repo repository.Repository) Service {
    return &service{repo: repo}
}

// FilterServicesBatch - основная функция с конкурентной обработкой
func (s *service) FilterServicesBatch(ctx context.Context, workers int) (<-chan models.Result, <-chan error) {
    // Получаем сервисы из репозитория
    servicesChan, readErrChan := s.repo.GetServices(ctx)
    
    resultsChan := make(chan models.Result, 100)
    procErrChan := make(chan error, 1)
    
    var wg sync.WaitGroup
    
    // Запускаем worker'ов
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for svc := range servicesChan {
                select {
                case <-ctx.Done():
                    return
                default:
                    if svc.DeprecatedDate == TargetDeprecatedDate && 
                       svc.BusinessLine == TargetBusinessLine {
                        result := models.Result{
                            ID:     svc.ID,
                            Name:   svc.Name,
                            Tenant: svc.Tenant,
                        }
                        
                        select {
                        case <-ctx.Done():
                            return
                        case resultsChan <- result:
                        }
                    }
                }
            }
        }(i)
    }
    
    // Горутина для координации завершения
    go func() {
        wg.Wait()
        close(resultsChan)
        
        // Проверяем ошибки чтения
        select {
        case err := <-readErrChan:
            if err != nil && err != context.Canceled {
                procErrChan <- err
            }
        default:
        }
        close(procErrChan)
    }()
    
    return resultsChan, procErrChan
}

// FilterServices - алиас для обратной совместимости
func (s *service) FilterServices(ctx context.Context, workers int) (<-chan models.Result, <-chan error) {
    return s.FilterServicesBatch(ctx, workers)
}