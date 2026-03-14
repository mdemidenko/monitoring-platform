package monitor

import (
    "context"
    "testing"

    "github.com/mdemidenko/monitoring-platform/internal/client"
    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// === Моки ===

// MockRepository — мок репозитория
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

// MockPassportClient — мок клиента паспорта
type MockPassportClient struct {
    mock.Mock
}

func (m *MockPassportClient) GetLatestReleases(ctx context.Context, tenant, service string) ([]client.ReleaseInfo, error) {
    args := m.Called(ctx, tenant, service)
    return args.Get(0).([]client.ReleaseInfo), args.Error(1)
}

// MockCollection — мок MongoDB collection
type MockCollection struct {
    mock.Mock
}

func (m *MockCollection) UpdateOne(ctx context.Context, filter, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
    args := m.Called(ctx, filter, update)
    return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}

// === Тесты ===

func TestEnricher_EnrichServices_Success(t *testing.T) {
    // Данные
    services := []models.Service{
        {
            ID:       1,
            Name:     "svc1",
            Tenant:   "t1",
            Clusters: nil,
        },
        {
            ID:       2,
            Name:     "svc2",
            Tenant:   "t2",
            Clusters: []string{"existing"},
        },
    }

    releases := []client.ReleaseInfo{
        {ID: 100, Status: "PENDING"},
        {ID: 101, Status: "FINISHED", Clusters: []string{"cluster-a", "cluster-b"}},
    }

    // Моки
    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)

    // Каналы
    servicesChan := make(chan models.Service, len(services))
    errChan := make(chan error, 1)

    for _, svc := range services {
        servicesChan <- svc
    }
    close(servicesChan)
    close(errChan)

    // Ожидания
    repo.On("GetServices", mock.Anything).Return(servicesChan, errChan)
    passport.On("GetLatestReleases", mock.Anything, "t1", "svc1").Return(releases, nil)
    collection.On("UpdateOne", mock.Anything, bson.M{"id": int64(1)}, bson.M{"$set": bson.M{"clusters": []string{"cluster-a", "cluster-b"}}}).Return(&mongo.UpdateResult{MatchedCount: 1}, nil)

    // Создаём enricher
    enricher := NewEnricher(repo, passport, collection)

    // Запускаем
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    err := enricher.EnrichServices(ctx, 2)

    // Проверяем
    assert.NoError(t, err)
    repo.AssertExpectations(t)
    passport.AssertExpectations(t)
    collection.AssertExpectations(t)
}

func TestEnricher_EnrichServices_NoFinishedReleases(t *testing.T) {
    services := []models.Service{
        {ID: 1, Name: "svc1", Tenant: "t1"},
    }

    releases := []client.ReleaseInfo{
        {ID: 100, Status: "PENDING"},
        {ID: 101, Status: "FAILED"},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1)

    servicesChan <- services[0]
    close(servicesChan)
    close(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesChan, errChan)
    passport.On("GetLatestReleases", mock.Anything, "t1", "svc1").Return(releases, nil)
    // Не должно быть вызова UpdateOne

    enricher := NewEnricher(repo, passport, collection)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    err := enricher.EnrichServices(ctx, 1)

    assert.NoError(t, err)
    repo.AssertExpectations(t)
    passport.AssertExpectations(t)
    collection.AssertNotCalled(t, "UpdateOne")
}

func TestEnricher_EnrichServices_APIError(t *testing.T) {
    services := []models.Service{
        {ID: 1, Name: "svc1", Tenant: "t1"},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1)

    servicesChan <- services[0]
    close(servicesChan)
    close(errChan)

    repo.On("GetServices", mock.Anything).Return(servicesChan, errChan)
    passport.On("GetLatestReleases", mock.Anything, "t1", "svc1").Return(nil, assert.AnError)

    enricher := NewEnricher(repo, passport, collection)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    err := enricher.EnrichServices(ctx, 1)

    assert.NoError(t, err) // Ошибка логируется, но не возвращается
    repo.AssertExpectations(t)
    passport.AssertExpectations(t)
    collection.AssertNotCalled(t, "UpdateOne")
}

func TestEnricher_EnrichServices_ContextCancelled(t *testing.T) {
    services := []models.Service{
        {ID: 1, Name: "svc1", Tenant: "t1"},
    }

    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1)

    servicesChan <- services[0]
    // Не закрываем — имитируем поток

    repo.On("GetServices", mock.Anything).Return(servicesChan, errChan)

    enricher := NewEnricher(repo, passport, collection)

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // сразу отменяем

    err := enricher.EnrichServices(ctx, 1)

    assert.ErrorIs(t, err, context.Canceled)
    repo.AssertExpectations(t)
    passport.AssertNotCalled(t, "GetLatestReleases")
    collection.AssertNotCalled(t, "UpdateOne")

    close(servicesChan) // чистим
}

func TestEnricher_EnrichServices_RepoError(t *testing.T) {
    repo := new(MockRepository)
    passport := new(MockPassportClient)
    collection := new(MockCollection)

    servicesChan := make(chan models.Service, 1)
    errChan := make(chan error, 1)

    errChan <- assert.AnError
    close(servicesChan)

    repo.On("GetServices", mock.Anything).Return(servicesChan, errChan)

    enricher := NewEnricher(repo, passport, collection)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    err := enricher.EnrichServices(ctx, 1)

    assert.ErrorIs(t, err, assert.AnError)
    repo.AssertExpectations(t)
    passport.AssertNotCalled(t, "GetLatestReleases")
    collection.AssertNotCalled(t, "UpdateOne")
}