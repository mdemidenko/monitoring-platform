package monitor

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/mdemidenko/monitoring-platform/internal/client"
    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// === Моки ===

type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) GetServices(ctx context.Context) (<-chan models.Service, <-chan error) {
    args := m.Called(ctx)
    return args.Get(0).(<-chan models.Service), args.Get(1).(<-chan error)
}

func (m *MockRepository) SaveResults(ctx context.Context, results <-chan models.Result) <-chan error {
    args := m.Called(ctx, results)
    return args.Get(0).(<-chan error)
}

type MockPassportClient struct {
    mock.Mock
}

func (m *MockPassportClient) GetLatestReleases(ctx context.Context, tenant, service string) ([]client.ReleaseInfo, error) {
    args := m.Called(ctx, tenant, service)
    return args.Get(0).([]client.ReleaseInfo), args.Error(1)
}

type MockCollection struct {
    mock.Mock
}

func (m *MockCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
    args := m.Called(ctx, filter, update)
    return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}

type MockRedisRepository struct {
    mock.Mock
}

func (m *MockRedisRepository) LogChange(ctx context.Context, entityType, entityID, action string, data interface{}) error {
    args := m.Called(ctx, entityType, entityID, action, data)
    return args.Error(0)
}

func (m *MockRedisRepository) Close() error {
    args := m.Called()
    return args.Error(0)
}

// === Вспомогательная функция: chan → <-chan ===
func toRecvChan[T any](ch chan T) <-chan T {
    return (<-chan T)(ch)
}

// === Тесты ===

func TestEnricher_EnrichServices_Success(t *testing.T) {
    services := []models.Service{
        {ID: int64(1), Name: "svc1", Tenant: "t1", Clusters: nil},
    }
    releases := []client.ReleaseInfo{
        {ID: 101, Status: "FINISHED", Clusters: []string{"cluster-a", "cluster-b"}},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)
    redisRepo := new(MockRedisRepository)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1) // ← НЕ закрываем

    servicesChan <- services[0]
    close(servicesChan) // можно закрыть — Enricher сам завершит

    servicesRecv := toRecvChan(servicesChan)
    errRecv := toRecvChan(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesRecv, errRecv).Once()
    passport.On("GetLatestReleases", mock.Anything, "t1", "svc1").Return(releases, nil).Once()
    collection.On("UpdateOne",
        mock.Anything,
        bson.M{"id": int64(1)},
        bson.M{"$set": bson.M{"clusters": []string{"cluster-a", "cluster-b"}}},
    ).Return(&mongo.UpdateResult{MatchedCount: 1}, nil).Once()
    redisRepo.On("LogChange",
        mock.Anything,
        "service",
        "1",
        "enrich",
        mock.MatchedBy(func(data interface{}) bool {
            d, ok := data.(map[string]interface{})
            return ok && d["service_id"] == int64(1)
        }),
    ).Return(nil).Once()

    enricher := NewEnricher(repo, passport, collection, redisRepo)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var enrichErr error
    go func() {
        enrichErr = enricher.EnrichServices(ctx, 1)
    }()

    time.Sleep(100 * time.Millisecond)
    cancel()
    time.Sleep(50 * time.Millisecond)
    enricher.Wait()

    assert.NoError(t, enrichErr)
    repo.AssertExpectations(t)
    passport.AssertExpectations(t)
    collection.AssertExpectations(t)
    redisRepo.AssertExpectations(t)
}

func TestEnricher_EnrichServices_APIError(t *testing.T) {
    services := []models.Service{
        {ID: int64(1), Name: "svc1", Tenant: "t1", Clusters: nil},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)
    redisRepo := new(MockRedisRepository)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1) // ← НЕ закрываем

    servicesChan <- services[0]
    close(servicesChan)

    servicesRecv := toRecvChan(servicesChan)
    errRecv := toRecvChan(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesRecv, errRecv).Once()
    passport.On("GetLatestReleases", mock.Anything, "t1", "svc1").Return(nil, assert.AnError).Once()

    collection.AssertNotCalled(t, "UpdateOne")
    redisRepo.AssertNotCalled(t, "LogChange")

    enricher := NewEnricher(repo, passport, collection, redisRepo)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var enrichErr error
    go func() {
        enrichErr = enricher.EnrichServices(ctx, 1)
    }()

    time.Sleep(100 * time.Millisecond)
    cancel()
    time.Sleep(50 * time.Millisecond)
    enricher.Wait()

    assert.NoError(t, enrichErr)
    repo.AssertExpectations(t)
    passport.AssertExpectations(t)
}

func TestEnricher_EnrichServices_ContextCancelled(t *testing.T) {
    services := []models.Service{
        {ID: int64(1), Name: "svc1", Tenant: "t1", Clusters: nil},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)
    redisRepo := new(MockRedisRepository)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1) // ← НЕ закрываем

    servicesChan <- services[0]
    close(servicesChan)

    servicesRecv := toRecvChan(servicesChan)
    errRecv := toRecvChan(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesRecv, errRecv).Once()

    enricher := NewEnricher(repo, passport, collection, redisRepo)

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // сразу отменяем

    err := enricher.EnrichServices(ctx, 1)
    enricher.Wait()

    assert.ErrorIs(t, err, context.Canceled)
    repo.AssertExpectations(t)
    passport.AssertNotCalled(t, "GetLatestReleases")
    collection.AssertNotCalled(t, "UpdateOne")
    redisRepo.AssertNotCalled(t, "LogChange")
}

func TestEnricher_EnrichServices_RepoError(t *testing.T) {
    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)
    redisRepo := new(MockRedisRepository)

    servicesChan := make(chan models.Service, 1)
    // errChan := make(chan error, 1) // ← НЕ закрываем

    // Создаём отдельный repoErrChan
    repoErrChan := make(chan error, 1)
    repoErrChan <- assert.AnError
    close(repoErrChan)

    close(servicesChan) // servicesChan пустой, но можно закрыть

    servicesRecv := toRecvChan(servicesChan)
    errRecv := toRecvChan(repoErrChan) // ← передаём repoErrChan, а не errChan

    repo.On("GetServices", mock.Anything).Return(servicesRecv, errRecv).Once()

    enricher := NewEnricher(repo, passport, collection, redisRepo)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var enrichErr error
    go func() {
        enrichErr = enricher.EnrichServices(ctx, 1)
    }()

    time.Sleep(100 * time.Millisecond)
    cancel()
    time.Sleep(50 * time.Millisecond)
    enricher.Wait()

    assert.ErrorIs(t, enrichErr, assert.AnError)
    repo.AssertExpectations(t)
    passport.AssertNotCalled(t, "GetLatestReleases")
    collection.AssertNotCalled(t, "UpdateOne")
    redisRepo.AssertNotCalled(t, "LogChange")
}

func TestEnricher_Wait_BlocksUntilDone(t *testing.T) {
    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)
    redisRepo := new(MockRedisRepository)

    var wg sync.WaitGroup
    wg.Add(1)

    redisRepo.On("LogChange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
        Run(func(args mock.Arguments) {
            time.Sleep(100 * time.Millisecond)
            wg.Done()
        }).
        Return(nil).Once()

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1)

    servicesChan <- models.Service{ID: int64(1), Name: "test", Tenant: "t1", Clusters: nil}
    close(servicesChan)

    servicesRecv := toRecvChan(servicesChan)
    errRecv := toRecvChan(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesRecv, errRecv).Once()
    passport.On("GetLatestReleases", mock.Anything, "t1", "test").Return([]client.ReleaseInfo{
        {ID: 1, Status: "FINISHED", Clusters: []string{"c1"}},
    }, nil).Once()
    collection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything).Return(&mongo.UpdateResult{MatchedCount: 1}, nil).Once()

    enricher := NewEnricher(repo, passport, collection, redisRepo)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var enrichErr error
    go func() {
        enrichErr = enricher.EnrichServices(ctx, 1)
    }()

    time.Sleep(50 * time.Millisecond)
    cancel()
    time.Sleep(100 * time.Millisecond)
    start := time.Now()
    enricher.Wait()
    elapsed := time.Since(start)

    assert.Greater(t, elapsed, 50*time.Millisecond, "Wait() должен дождаться завершения асинхронной операции")
    assert.NoError(t, enrichErr)
    redisRepo.AssertExpectations(t)
}

func TestEnricher_LogChange_Error(t *testing.T) {
    services := []models.Service{
        {ID: int64(1), Name: "svc1", Tenant: "t1", Clusters: nil},
    }
    releases := []client.ReleaseInfo{
        {ID: 101, Status: "FINISHED", Clusters: []string{"c1"}},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)
    redisRepo := new(MockRedisRepository)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1)

    servicesChan <- services[0]
    close(servicesChan)

    servicesRecv := toRecvChan(servicesChan)
    errRecv := toRecvChan(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesRecv, errRecv).Once()
    passport.On("GetLatestReleases", mock.Anything, "t1", "svc1").Return(releases, nil).Once()
    collection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything).Return(&mongo.UpdateResult{MatchedCount: 1}, nil).Once()
    redisRepo.On("LogChange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError).Once()

    enricher := NewEnricher(repo, passport, collection, redisRepo)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var enrichErr error
    go func() {
        enrichErr = enricher.EnrichServices(ctx, 1)
    }()

    time.Sleep(100 * time.Millisecond)
    cancel()
    time.Sleep(50 * time.Millisecond)
    enricher.Wait()

    assert.NoError(t, enrichErr, "Ошибка в LogChange не должна влиять на результат EnrichServices")
    redisRepo.AssertExpectations(t)
}