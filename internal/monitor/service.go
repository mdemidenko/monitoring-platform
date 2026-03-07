package monitor

import (
    "context"
    "sync"
    "fmt"
    "log"

    "github.com/mdemidenko/monitoring-platform/internal/client"
    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/bson"
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

// Служба обогащения
type Enricher struct {
    repo       repository.Repository // для GetServices (из MongoDB)
    passport   *client.PassportClient
    collection *mongo.Collection // для UpdateOne
}

func NewEnricher(repo repository.Repository, passport *client.PassportClient, collection *mongo.Collection) *Enricher {
    return &Enricher{
        repo:       repo,
        passport:   passport,
        collection: collection,
    }
}

func (e *Enricher) EnrichServices(ctx context.Context, workers int) error {
    log.Println("🔄 Начало работы EnrichServices")

    errChan := make(chan error, 1)

    go func() {
        defer close(errChan)
        log.Println("✅ Горутина запущена")

        servicesChan, repoErrChan := e.repo.GetServices(ctx)
        log.Println("📥 Получены каналы: servicesChan, repoErrChan")

        jobChan := make(chan models.Service, workers)
        var wg sync.WaitGroup

        // Запуск воркеров
        for i := 0; i < workers; i++ {
            wg.Add(1)
            go func(workerID int) {
                defer wg.Done()
                log.Printf("👷 Воркер %d запущен", workerID)

                for svc := range jobChan {
                    log.Printf("📥 Воркер %d получил сервис: id=%v, name=%s", workerID, svc.ID, svc.Name)

                    select {
                    case <-ctx.Done():
                        log.Printf("🛑 Воркер %d: контекст отменён", workerID)
                        return
                    default:
                    }

                    log.Printf("➡️  Запрос к API: tenant=%s, service=%s", svc.Tenant, svc.Name)
                    releases, err := e.passport.GetLatestReleases(ctx, svc.Tenant, svc.Name)
                    if err != nil {
                        log.Printf("❌ Ошибка API для %s/%s: %v", svc.Tenant, svc.Name, err)
                        select {
                        case errChan <- fmt.Errorf("service %s/%s: %w", svc.Tenant, svc.Name, err):
                        case <-ctx.Done():
                        }
                        continue
                    }

                    log.Printf("📩 Получено релизов: %d", len(releases))

                    // Ищем FINISHED
                    var latestFinished *client.ReleaseInfo
                    for _, r := range releases {
                        if r.Status == "FINISHED" {
                            latestFinished = &r
                            break
                        }
                    }

                    if latestFinished != nil {
                        log.Printf("✅ Найден FINISHED релиз: id=%d, clusters=%v", latestFinished.ID, latestFinished.Clusters)

                        filter := bson.M{"id": svc.ID}
                        update := bson.M{"$set": bson.M{"clusters": latestFinished.Clusters}}

                        result, err := e.collection.UpdateOne(ctx, filter, update)
                        if err != nil {
                            log.Printf("❌ Ошибка обновления %v: %v", svc.ID, err)
                            select {
                            case errChan <- fmt.Errorf("не удалось обновить %v: %w", svc.ID, err):
                            case <-ctx.Done():
                            }
                            continue
                        }

                        if result.MatchedCount == 0 {
                            log.Printf("⚠️  Не найден сервис в MongoDB: id=%v", svc.ID)
                        } else {
                            log.Printf("💾 Обновлено: %s/%s → clusters=%v", svc.Tenant, svc.Name, latestFinished.Clusters)
                        }
                    } else {
                        log.Printf("🟡 Нет завершённых релизов для %s/%s", svc.Tenant, svc.Name)
                    }
                }

                log.Printf("🔚 Воркер %d завершил обработку jobChan", workerID)
            }(i)
        }

        // Отправка задач
        go func() {
            defer close(jobChan)
            log.Println("📤 Начало отправки задач в jobChan")

            for {
                select {
                case err := <-repoErrChan:
                    if err != nil {
                        log.Printf("❌ Ошибка из repoErrChan: %v", err)
                        select {
                        case errChan <- err:
                        case <-ctx.Done():
                        }
                    }
                    return
                case svc, ok := <-servicesChan:
                    if !ok {
                        log.Println("🔚 servicesChan закрыт, завершаем отправку")
                        return
                    }
                    log.Printf("📤 Отправляем сервис в jobChan: id=%v, name=%s", svc.ID, svc.Name)

                    select {
                    case <-ctx.Done():
                        log.Println("🛑 Контекст отменён, прекращаем отправку")
                        return
                    case jobChan <- svc:
                        log.Printf("✅ Сервис id=%v отправлен в jobChan", svc.ID)
                    }
                }
            }
        }()

        // Ожидание завершения воркеров
        go func() {
            wg.Wait()
            log.Println("✅ Все воркеры завершили работу")
        }()

        // Ждём завершения или отмены
        select {
        case <-ctx.Done():
            log.Printf("🛑 Контекст отменён: %v", ctx.Err())
            errChan <- ctx.Err()
        }
    }()

    // Блокируем и читаем первую ошибку
    log.Println("⏳ Ожидание результата из errChan...")
    if err := <-errChan; err != nil && err != context.Canceled {
        log.Printf("❌ Ошибка в EnrichServices: %v", err)
        return err
    }

    log.Println("✅ EnrichServices завершён успешно")
    return nil
}