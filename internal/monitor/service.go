package monitor

import (
    "context"
    "log"
    "sync"
    "fmt"

    "github.com/mdemidenko/monitoring-platform/internal/client"
    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/mdemidenko/monitoring-platform/internal/repository"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
)

type Enricher struct {
    repo       repository.Repository
    passport   *client.PassportClient
    collection *mongo.Collection
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
    errChan := make(chan error, 1) // буферизован
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

                    // 🔍 Пропускаем, если уже есть clusters
                    if len(svc.Clusters) > 0 {
                        log.Printf("⏭️  Уже обогащён: %s/%s → clusters=%v", svc.Tenant, svc.Name, svc.Clusters)
                        continue
                    }

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
                            case errChan <- fmt.Errorf("не удалось обновить %v: %w", svc.ID, err):
                            case <-ctx.Done():
                            }
                            continue
                        }

                        if result.MatchedCount == 0 {
                            log.Printf("⚠️  Не найден сервис в MongoDB: id=%v", svc.ID)
                        } else {
                            log.Printf("💾 Обновлено: %s/%s → clusters=%v", svc.Tenant, svc.Name, clusters)
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
            // ✅ Отправляем nil — означает "всё хорошо"
            select {
            case errChan <- nil:
            case <-ctx.Done():
            }
        }()

        // Ожидаем отмену контекста ИЛИ успешное завершение
        select {
        case <-ctx.Done():
            log.Printf("🛑 Контекст отменён: %v", ctx.Err())
            errChan <- ctx.Err()
        }
    }()

    // Блокируем и ждём сигнал от горутины
    log.Println("⏳ Ожидание завершения обогащения...")
    err := <-errChan
    if err != nil && err != context.Canceled {
        log.Printf("❌ Ошибка в EnrichServices: %v", err)
        return err
    }
    log.Println("✅ EnrichServices завершён успешно")
    return nil
}