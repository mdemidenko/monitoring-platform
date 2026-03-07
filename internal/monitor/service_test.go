package monitor

import (
    "context"
    "testing"

    "github.com/mdemidenko/monitoring-platform/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// MockRepository — мок для repository.Repository
type MockRepository struct {
    mock.Mock
}

// GetServices возвращает receive-only канал сервисов и ошибок
func (m *MockRepository) GetServices(ctx context.Context) (<-chan models.Service, <-chan error) {
    args := m.Called(ctx)
    return args.Get(0).(<-chan models.Service), args.Get(1).(<-chan error)
}

// SaveResults возвращает канал ошибок
func (m *MockRepository) SaveResults(ctx context.Context, results <-chan models.Result) <-chan error {
    args := m.Called(ctx, results)
    return args.Get(0).(<-chan error)
}

// === Тесты ===

// TestNew проверяет создание сервиса
func TestNew(t *testing.T) {
    repo := &MockRepository{}
    svc := New(repo)
    assert.NotNil(t, svc)
}

// TestService_FilterServicesBatch проверяет фильтрацию
func TestService_FilterServicesBatch(t *testing.T) {
    repo := &MockRepository{}
    svc := &service{repo: repo}
    ctx := context.Background()

    services := make(chan models.Service, 2)
    errors := make(chan error, 1)

    services <- models.Service{
        ID:             1,
        Name:           "Service 1",
        Tenant:         "Tenant A",
        DeprecatedDate: TargetDeprecatedDate,
        BusinessLine:   TargetBusinessLine,
    }
    services <- models.Service{
        ID:   2,
        Name: "Service 2",
    }

    close(services)
    close(errors)

    // ✅ Явное приведение к receive-only каналам
    repo.On("GetServices", ctx).Return((<-chan models.Service)(services), (<-chan error)(errors))

    results, procErrs := svc.FilterServicesBatch(ctx, 2)

    var resultCount int
    for result := range results {
        assert.Equal(t, 1, result.ID)
        assert.Equal(t, "Service 1", result.Name)
        resultCount++
    }

    assert.Equal(t, 1, resultCount)

    err := <-procErrs
    assert.Nil(t, err)

    repo.AssertExpectations(t)
}

// TestService_FilterServicesBatch_Empty проверяет пустой поток
func TestService_FilterServicesBatch_Empty(t *testing.T) {
    repo := &MockRepository{}
    svc := &service{repo: repo}
    ctx := context.Background()

    services := make(chan models.Service, 1)
    errors := make(chan error, 1)

    close(services)
    close(errors)

    // ✅ Приведение типов
    repo.On("GetServices", ctx).Return((<-chan models.Service)(services), (<-chan error)(errors))

    results, procErrs := svc.FilterServicesBatch(ctx, 2)

    var resultCount int
    for range results {
        resultCount++
    }
    assert.Equal(t, 0, resultCount)

    err := <-procErrs
    assert.Nil(t, err)

    repo.AssertExpectations(t)
}

// TestService_FilterServicesBatch_WithError проверяет ошибку из репозитория
func TestService_FilterServicesBatch_WithError(t *testing.T) {
    repo := &MockRepository{}
    svc := &service{repo: repo}
    ctx := context.Background()

    services := make(chan models.Service, 1)
    errors := make(chan error, 1)

    // Отправляем ошибку
    errors <- assert.AnError
    close(services)

    // ✅ Приведение типов
    repo.On("GetServices", ctx).Return((<-chan models.Service)(services), (<-chan error)(errors))

    results, procErrs := svc.FilterServicesBatch(ctx, 1)

    // Потребляем результаты
    for range results {
    }

    err := <-procErrs
    assert.Error(t, err)
    assert.Equal(t, assert.AnError, err)

    repo.AssertExpectations(t)
}

// TestService_FilterServicesBatch_ContextCancel проверяет отмену
func TestService_FilterServicesBatch_ContextCancel(t *testing.T) {
    repo := &MockRepository{}
    svc := &service{repo: repo}
    ctx, cancel := context.WithCancel(context.Background())

    services := make(chan models.Service, 1)
    errors := make(chan error, 1)

    // ✅ Приведение типов
    repo.On("GetServices", ctx).Return((<-chan models.Service)(services), (<-chan error)(errors))

    results, procErrs := svc.FilterServicesBatch(ctx, 1)

    cancel()

    // Закрываем services, чтобы воркеры завершились
    close(services)

    // Потребляем результаты
    var resultCount int
    for range results {
        resultCount++
    }
    assert.Equal(t, 0, resultCount)

    // Потребляем ошибку
    err := <-procErrs
    assert.Nil(t, err)

    repo.AssertExpectations(t)
}

// TestService_FilterServices проверяет алиас
func TestService_FilterServices(t *testing.T) {
    repo := &MockRepository{}
    svc := &service{repo: repo}
    ctx := context.Background()

    services := make(chan models.Service, 1)
    errors := make(chan error, 1)

    close(services)
    close(errors)

    // ✅ Приведение типов
    repo.On("GetServices", ctx).Return((<-chan models.Service)(services), (<-chan error)(errors))

    results, procErrs := svc.FilterServices(ctx, 1)

    // Потребляем результаты
    for range results {
    }

    err := <-procErrs
    assert.Nil(t, err)

    repo.AssertExpectations(t)
}