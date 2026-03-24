package monitor

import (
    "context"
    "log"
    "sync"
    "fmt"
    "time"

    "github.com/mdemidenko/monitoring-platform/internal/client"
    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// === Интерфейсы для внедрения зависимостей ===
type PassportClient interface {
    GetLatestReleases(ctx context.Context, tenant, service string) ([]client.ReleaseInfo, error)
}

type MongoCollection interface {
    UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
}

// === Enricher ===
type Enricher struct {
    repo       repository.Repository
    passport   PassportClient
    collection MongoCollection
    redisRepo  repository.RedisRepositoryInterface
    wg         sync.WaitGroup
}

// NewEnricher создаёт новый обогатитель
func NewEnricher(
    repo repository.Repository,
    passport PassportClient,
    collection MongoCollection,
    redisRepo repository.RedisRepositoryInterface,
    ) *Enricher {
        return &Enricher{
            repo:       repo,
            passport:   passport,
            collection: collection,
            redisRepo:  redisRepo,
            wg:         sync.WaitGroup{},
        }
}

// EnrichServices обогащает сервисы кластерами из последнего FINISHED релиза
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
                for svc := range jobChan {
                    // 🔍 Пропускаем, если уже есть clusters
                    if len(svc.Clusters) > 0 {
                        continue
                    }

                    select {
                    case <-ctx.Done():
                        log.Printf("🛑 Воркер %d: контекст отменён", workerID)
                        return
                    default:
                    }

                    releases, err := e.passport.GetLatestReleases(ctx, svc.Tenant, svc.Name)
                    if err != nil {
                        log.Printf("❌ Ошибка API для %s/%s: %v", svc.Tenant, svc.Name, err)
                        select {
                        case errChan <- err:
                        case <-ctx.Done():
                        }
                        continue
                    }
                    log.Printf("📩 Получено релизов: %d", len(releases))

                    // Ищем первый FINISHED релиз
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
                        clusters := latestFinished.Clusters
                        if clusters == nil {
                            clusters = []string{}
                        }
                        update := bson.M{"$set": bson.M{"clusters": clusters}}
                        result, err := e.collection.UpdateOne(ctx, filter, update)
                        if err != nil {
                            log.Printf("❌ Ошибка обновления %v: %v", svc.ID, err)
                            select {
                            case errChan <- err:
                            case <-ctx.Done():
                            }
                            continue
                        }

                        if result.MatchedCount == 0 {
                            log.Printf("⚠️  Не найден сервис в MongoDB: id=%v", svc.ID)
                        } else {
                            log.Printf("💾 Обновлено: %s/%s → clusters=%v", svc.Tenant, svc.Name, clusters)

                            // ✅ REDIS: Логируем изменение
                            e.wg.Add(1)
                            go func() {
                                defer e.wg.Done()
                                logData := map[string]interface{}{
                                    "action":        "enrich",
                                    "service_id":    svc.ID,
                                    "service_name":  svc.Name,
                                    "tenant":        svc.Tenant,
                                    "new_clusters":  clusters,
                                    "release_id":    latestFinished.ID,
                                    "updated_at":    time.Now().UTC().Format(time.RFC3339),
                                    "worker_id":     workerID,
                                }

                                // Используем fmt.Sprintf("%v") на случай, если ID — ObjectID
                                entityID := fmt.Sprintf("%v", svc.ID)
                                if err := e.redisRepo.LogChange(ctx, "service", entityID, "enrich", logData); err != nil {
                                    log.Printf("⚠️ REDIS: Не удалось записать событие: %v", err)
                                    // Не паникуем — Redis не критичен
                                }
                            }()
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
                    select {
                    case <-ctx.Done():
                        return
                    case jobChan <- svc:
                    }
                }
            }
        }()

        // Ожидание завершения воркеров
        go func() {
            wg.Wait()
            log.Println("✅ Все воркеры завершили работу")
            select {
            case errChan <- nil:
            case <-ctx.Done():
            }
        }()

        // Ожидаем отмену контекста или успешное завершение
        <-ctx.Done()
        log.Printf("🛑 Контекст отменён: %v", ctx.Err())
        errChan <- ctx.Err()
    }()

    log.Println("⏳ Ожидание завершения обогащения...")
    err := <-errChan
    if err != nil && err != context.Canceled {
        log.Printf("❌ Ошибка в EnrichServices: %v", err)
        return err
    }
    log.Println("✅ EnrichServices завершён успешно")
    return nil
}

func (e *Enricher) Wait() {
    e.wg.Wait()
}