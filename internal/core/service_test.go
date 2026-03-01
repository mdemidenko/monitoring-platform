package core

import (
    "context"
    "testing"
    "time"
    "sync"
    "fmt"

    "github.com/mdemidenko/monitoring-platform/internal/domain"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// === Моки ===

type MockNotificationRepository struct {
    mock.Mock
}

func (m *MockNotificationRepository) Store(entity interface{}) error {
    args := m.Called(entity)
    return args.Error(0)
}

func (m *MockNotificationRepository) GetNotifications() []*domain.Notification {
    args := m.Called()
    return args.Get(0).([]*domain.Notification)
}

func (m *MockNotificationRepository) GetSentNotifications() []*domain.SentNotification {
    args := m.Called()
    return args.Get(0).([]*domain.SentNotification)
}

func (m *MockNotificationRepository) GetStats() *domain.ServiceStats {
    args := m.Called()
    return args.Get(0).(*domain.ServiceStats)
}

type MockNotificationSender struct {
    mock.Mock
}

// ✅ Ключевое: используем domain.Notification
func (m *MockNotificationSender) Send(ctx context.Context, notification *domain.Notification) (*domain.SentNotification, error) {
    args := m.Called(ctx, notification)
    return args.Get(0).(*domain.SentNotification), args.Error(1)
}

func (m *MockNotificationSender) HealthCheck() error {
    args := m.Called()
    return args.Error(0)
}

// === Тесты ===

func TestNewNotificationService(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)
    assert.NotNil(t, svc)
}

func TestProcessWithIntervals_ContextCancel(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    expectedNotification := &domain.Notification{ChatID: "123", Text: "Msg1"}

    // ✅ Настройка моков
    repo.On("Store", expectedNotification).Return(nil)
    repo.On("Store", mock.AnythingOfType("*domain.SentNotification")).Return(nil)

    sender.On("Send", mock.AnythingOfType("*context.timerCtx"), expectedNotification).
        Return(&domain.SentNotification{MessageID: 1, ChatID: 123}, nil)

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    notifications := []*domain.Notification{expectedNotification}

    result := svc.ProcessWithIntervals(ctx, notifications, 10*time.Millisecond, 1)

    // Проверки результата
    assert.Equal(t, 1, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)

    // ✅ Проверка, что моки были вызваны
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

func TestSendNotification_ContextCancelled(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    repo.On("Store", mock.AnythingOfType("*domain.Notification")).Return(nil).Once()
    sender.On("Send", mock.Anything, mock.AnythingOfType("*domain.Notification")).
    Return((*domain.SentNotification)(nil), context.Canceled).Once()

    // Вызов
    sent, err := svc.SendNotification(ctx, "123", "Hello")

    // Проверка
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")
    assert.Contains(t, err.Error(), "failed to send notification")
    assert.Nil(t, sent)

    // Проверяем моки
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

func TestProcessEntity_StoreError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    notification := &domain.Notification{ChatID: "123", Text: "Test"}

    repo.On("Store", notification).Return(assert.AnError).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to store entity")
    repo.AssertExpectations(t)
}

func TestGetNotifications(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    expected := []*domain.Notification{
        {ChatID: "123", Text: "Msg1"},
    }
    repo.On("GetNotifications").Return(expected).Once()

    result := svc.GetNotifications()

    assert.Equal(t, expected, result)
    repo.AssertExpectations(t)
}

func TestGetSentNotifications(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    expected := []*domain.SentNotification{
        {MessageID: 1, ChatID: 123, SentAt: time.Now()},
    }
    repo.On("GetSentNotifications").Return(expected).Once()

    result := svc.GetSentNotifications()

    assert.Equal(t, expected, result)
    repo.AssertExpectations(t)
}

func TestGetStats(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    // Мок: GetNotifications
    repo.On("GetNotifications").Return([]*domain.Notification{
        {ChatID: "1", Text: "Test1"},
        {ChatID: "2", Text: "Test2"},
    }).Once()

    // Мок: GetSentNotifications
    repo.On("GetSentNotifications").Return([]*domain.SentNotification{
        {MessageID: 1, ChatID: 1, SentAt: time.Now()},
    }).Once()

    result := svc.GetStats()

    // Ожидаем: TotalNotifications=2, Sent=1, Pending=1
    expected := &domain.ServiceStats{
        TotalNotifications:     2,
        TotalSentNotifications: 1,
        PendingNotifications:   1,
    }

    assert.Equal(t, expected, result)
    repo.AssertExpectations(t)
}
func TestSendNotification_Success(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx := context.Background()

    sentNotif := &domain.SentNotification{
        MessageID: 1,
        ChatID:    123,
        SentAt:    time.Now(),
    }

    // ✅ Исправлено: указатель + правильное сравнение контекста
    sender.On("Send",
        mock.MatchedBy(func(ctx context.Context) bool { return true }),
        &domain.Notification{
            ChatID: "123",
            Text:   "Hello",
        },
    ).Return(sentNotif, nil).Once()

    repo.On("Store", mock.MatchedBy(func(n *domain.Notification) bool {
        return n.ChatID == "123" && n.Text == "Hello"
    })).Return(nil).Once()

    repo.On("Store", sentNotif).Return(nil).Once()

    sent, err := svc.SendNotification(ctx, "123", "Hello")

    assert.NoError(t, err)
    assert.NotNil(t, sent)
    assert.Equal(t, int64(1), sent.MessageID)
    assert.Equal(t, int64(123), sent.ChatID)

    sender.AssertExpectations(t)
    repo.AssertExpectations(t)
}

func TestHealthCheck(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    // Мок: sender.HealthCheck → OK
    sender.On("HealthCheck").Return(nil).Once()

    err := svc.HealthCheck()

    assert.NoError(t, err)
    sender.AssertExpectations(t)
}

func TestHealthCheck_Failure(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    // Мок: sender.HealthCheck → error
    sender.On("HealthCheck").Return(assert.AnError).Once()

    err := svc.HealthCheck()

    assert.Error(t, err)
    sender.AssertExpectations(t)
}

func TestProcessEntity_SendError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    notification := &domain.Notification{ChatID: "123", Text: "Test"}

    // Store — OK
    repo.On("Store", notification).Return(nil).Once()

    // Send — ошибка
    sender.On("Send", mock.Anything, &domain.Notification{
        ChatID: "123",
        Text:   "Test",
    }).Return((*domain.SentNotification)(nil), assert.AnError).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to send notification")
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

func TestNotificationWorker_ContextCancelled(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    jobs := make(chan *domain.Notification, 1)
    results := make(chan *workerResult, 1)
    var wg sync.WaitGroup

    // Только один Add
    wg.Add(1)

    // Запускаем worker — он сам вызовет wg.Done()
    go svc.notificationWorker(ctx, 1, &wg, jobs, results)

    close(jobs)

    // Ждём завершения
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // OK
    case <-time.After(2 * time.Second):
        t.Fatal("Worker не завершился")
    }

    // Потребляем результаты
    close(results)
    for r := range results {
        t.Errorf("Unexpected result: %v", r)
    }
}
func TestProcessResults_ContextCancelled(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    results := make(chan *workerResult, 1)
    done := make(chan bool, 1)

    result := svc.processResults(ctx, results, done)

    assert.Equal(t, 0, result.SuccessCount)
    assert.Equal(t, 0, result.ErrorCount)
}

func TestSendNotificationsWithIntervals_ContextCancelled(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    notifications := []*domain.Notification{
        {ChatID: "123", Text: "Msg1"},
    }
    jobs := make(chan *domain.Notification, 1)
    results := make(chan *workerResult, 1)

    // Запускаем горутину
    go svc.sendNotificationsWithIntervals(ctx, notifications, jobs, 1*time.Millisecond)

    // ❌ УБРАТЬ: close(jobs)
    // Канал будет закрыт внутри sendNotificationsWithIntervals при ctx.Done()

    // Вместо этого — ждём немного, чтобы функция успела обработать отмену
    time.Sleep(10 * time.Millisecond)

    // Закрываем results, чтобы избежать утечки
    close(results)

    // Проверяем, что jobs можно читать, но пустой
    select {
    case _, ok := <-jobs:
        if ok {
            t.Fatal("jobs channel should be closed")
        }
    default:
        t.Fatal("jobs channel not closed yet")
    }
}

// === Тесты для core/errors.go ===

func TestCoreError_Error(t *testing.T) {
    // Создаём доменную ошибку
    domainErr := domain.NewDomainError(domain.ErrValidation, "invalid input", nil)

    // Создаём core.Error
    coreErr := NewCoreError("SendNotification", domainErr)

    // Проверяем, что Error() возвращает текст доменной ошибки
    assert.Equal(t, domainErr.Error(), coreErr.Error())
}

func TestCoreError_Unwrap(t *testing.T) {
    domainErr := domain.NewDomainError(domain.ErrValidation, "invalid input", nil)
    coreErr := NewCoreError("SendNotification", domainErr)

    // Unwrap должен вернуть *domain.DomainError
    unwrapped := coreErr.Unwrap()
    assert.Equal(t, domainErr, unwrapped)
}

func TestNewCoreError(t *testing.T) {
    domainErr := domain.NewDomainError(domain.ErrExternalService, "service unavailable", nil)
    coreErr := NewCoreError("HealthCheck", domainErr)

    assert.Equal(t, "HealthCheck", coreErr.Operation)
    assert.Equal(t, domainErr, coreErr.domainErr)
}

func TestIsValidationError_WithDomainError(t *testing.T) {
    // Ошибка валидации
    validationErr := domain.NewDomainError(domain.ErrValidation, "invalid", nil)
    assert.True(t, IsValidationError(validationErr))

    // Ошибка другого типа
    repoErr := domain.NewDomainError(domain.ErrRepository, "repo error", nil)
    assert.False(t, IsValidationError(repoErr))
}

func TestIsValidationError_WithCoreError(t *testing.T) {
    domainErr := domain.NewDomainError(domain.ErrValidation, "invalid", nil)
    coreErr := NewCoreError("SendNotification", domainErr)

    assert.True(t, IsValidationError(coreErr))

    // Не ошибка валидации
    domainErr2 := domain.NewDomainError(domain.ErrExternalService, "external", nil)
    coreErr2 := NewCoreError("SendNotification", domainErr2)
    assert.False(t, IsValidationError(coreErr2))
}

func TestIsValidationError_WithInvalidInput(t *testing.T) {
    // ErrInvalidInput тоже считается валидационной
    domainErr := domain.NewDomainError(domain.ErrInvalidInput, "bad input", nil)
    assert.True(t, IsValidationError(domainErr))

    // Через CoreError
    coreErr := NewCoreError("Process", domainErr)
    assert.True(t, IsValidationError(coreErr))
}

func TestIsExternalServiceError_WithDomainError(t *testing.T) {
    extErr := domain.NewDomainError(domain.ErrExternalService, "timeout", nil)
    assert.True(t, IsExternalServiceError(extErr))

    valErr := domain.NewDomainError(domain.ErrValidation, "invalid", nil)
    assert.False(t, IsExternalServiceError(valErr))
}

func TestIsExternalServiceError_WithCoreError(t *testing.T) {
    domainErr := domain.NewDomainError(domain.ErrExternalService, "timeout", nil)
    coreErr := NewCoreError("Send", domainErr)

    assert.True(t, IsExternalServiceError(coreErr))

    domainErr2 := domain.NewDomainError(domain.ErrValidation, "invalid", nil)
    coreErr2 := NewCoreError("Send", domainErr2)
    assert.False(t, IsExternalServiceError(coreErr2))
}

func TestIsValidationError_NilAndOther(t *testing.T) {
    // nil
    assert.False(t, IsValidationError(nil))

    // Любой другой тип ошибки
    otherErr := fmt.Errorf("some error")
    assert.False(t, IsValidationError(otherErr))
}

func TestIsExternalServiceError_NilAndOther(t *testing.T) {
    // nil
    assert.False(t, IsExternalServiceError(nil))

    // Любой другой тип ошибки
    otherErr := fmt.Errorf("some error")
    assert.False(t, IsExternalServiceError(otherErr))
}

func TestProcessEntity_ContextCancelled(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    err := svc.ProcessEntity(ctx, &domain.Notification{ChatID: "123", Text: "Test"})

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "operation cancelled")
}

func TestProcessEntity_StoreEntityError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    notification := &domain.Notification{ChatID: "123", Text: "Test"}

    repo.On("Store", notification).Return(assert.AnError).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to store entity")
    // Убрали сложную проверку
    repo.AssertExpectations(t)
}

func TestProcessEntity_StoreSentNotificationError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    notification := &domain.Notification{ChatID: "123", Text: "Test"}
    sentNotif := &domain.SentNotification{MessageID: 1, ChatID: 123, SentAt: time.Now()}

    // Store(notification) → OK
    repo.On("Store", notification).Return(nil).Once()

    // Send → OK
    sender.On("Send", mock.Anything, &domain.Notification{
        ChatID: "123",
        Text:   "Test",
    }).Return(sentNotif, nil).Once()

    // Store(sentNotif) → ошибка (только лог)
    repo.On("Store", sentNotif).Return(assert.AnError).Once()

    err := svc.ProcessEntity(context.Background(), notification)

    // Ошибки нет — только лог
    assert.NoError(t, err)
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}
func TestProcessEntity_SentNotification_Logs(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    sent := &domain.SentNotification{MessageID: 123, ChatID: 123}

    // Store → OK
    repo.On("Store", sent).Return(nil).Once()

    err := svc.ProcessEntity(context.Background(), sent)

    assert.NoError(t, err)
    repo.AssertExpectations(t)
}

func TestProcessEntity_UnsupportedType(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    // Store вызывается, но мы не заботимся о типе
    repo.On("Store", mock.Anything).Return(nil).Once()

    err := svc.ProcessEntity(context.Background(), "unsupported")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unsupported entity type")
    repo.AssertExpectations(t)
}

func TestSendNotification_EmptyText(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    sent, err := svc.SendNotification(context.Background(), "123", "")

    assert.Error(t, err)
    assert.Nil(t, sent)
    assert.Contains(t, err.Error(), "text cannot be empty")
    assert.True(t, IsValidationError(err))
}

func TestSendNotification_EmptyChatID(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    sent, err := svc.SendNotification(context.Background(), "", "Hello")

    assert.Error(t, err)
    assert.Nil(t, sent)
    assert.Contains(t, err.Error(), "chat_id cannot be empty")
    assert.True(t, IsValidationError(err))
}

func TestSendNotification_StoreError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    repo.On("Store", mock.MatchedBy(func(n *domain.Notification) bool {
        return n.ChatID == "123" && n.Text == "Hello"
    })).Return(assert.AnError).Once()

    sent, err := svc.SendNotification(context.Background(), "123", "Hello")

    assert.Error(t, err)
    assert.Nil(t, sent)
    assert.Contains(t, err.Error(), "failed to store notification")
    repo.AssertExpectations(t)
}
func TestSendNotification_StoreSentError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    // notification — это *domain.Notification
    notification := domain.NewNotification("123", "Hello")
    sentNotif := &domain.SentNotification{MessageID: 1, ChatID: 123, SentAt: time.Now()}

    // ✅ Передаём notification (указатель), а не &notification
    repo.On("Store", notification).Return(nil).Once()

    // В sender.Send — тоже передаём notification
    sender.On("Send", mock.Anything, notification).Return(sentNotif, nil).Once()

    // Ошибка при сохранении sentNotif
    repo.On("Store", sentNotif).Return(assert.AnError).Once()

    sent, err := svc.SendNotification(context.Background(), "123", "Hello")

    assert.NoError(t, err)
    assert.NotNil(t, sent)
    assert.Equal(t, int64(1), sent.MessageID)
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

func TestSendNotification_SendError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    notification := domain.NewNotification("123", "Hello")

    repo.On("Store", notification).Return(nil).Once()

    sender.On("Send", mock.Anything, notification).Return((*domain.SentNotification)(nil), assert.AnError).Once()

    sent, err := svc.SendNotification(context.Background(), "123", "Hello")

    assert.Error(t, err)
    assert.Nil(t, sent)
    assert.Contains(t, err.Error(), "failed to send notification")
    repo.AssertExpectations(t)
    sender.AssertExpectations(t)
}

func TestSendNotificationsWithIntervals_AllQueued(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    notifications := []*domain.Notification{
        {ChatID: "123", Text: "Msg1"},
        {ChatID: "123", Text: "Msg2"},
    }
    jobs := make(chan *domain.Notification, 2)

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    go svc.sendNotificationsWithIntervals(ctx, notifications, jobs, 1*time.Millisecond)

    // Собираем все уведомления
    var sent []*domain.Notification
    for n := range jobs {
        sent = append(sent, n)
    }

    assert.Equal(t, 2, len(sent))
    assert.Equal(t, "Msg1", sent[0].Text)
    assert.Equal(t, "Msg2", sent[1].Text)
}

func TestNotificationWorker_ProcessEntityError(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx := context.Background()
    jobs := make(chan *domain.Notification, 1)
    results := make(chan *workerResult, 1)
    var wg sync.WaitGroup

    notification := &domain.Notification{ChatID: "123", Text: "Test"}

    // ProcessEntity → ошибка
    repo.On("Store", notification).Return(assert.AnError).Once()

    wg.Add(1)
    go svc.notificationWorker(ctx, 1, &wg, jobs, results)

    jobs <- notification
    close(jobs)

    var result *workerResult
    select {
    case r := <-results:
        result = r
    case <-time.After(100 * time.Millisecond):
        t.Fatal("Не получен результат")
    }

    assert.Equal(t, "Test", result.Text)
    assert.Error(t, result.Error)

    wg.Wait()
    close(results)
    repo.AssertExpectations(t)
}

func TestProcessResults_ContextDone_WithTimeout(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    results := make(chan *workerResult, 1)
    done := make(chan bool) // не закрываем → сработает таймаут

    close(results)

    res := svc.processResults(ctx, results, done)

    assert.Equal(t, 0, res.SuccessCount)
    assert.Equal(t, 0, res.ErrorCount)
}

func TestProcessResults_Success(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    results := make(chan *workerResult, 2)
    done := make(chan bool)

    results <- &workerResult{Text: "OK", Error: nil}
    results <- &workerResult{Text: "Err", Error: assert.AnError}
    close(results)
    close(done)

    res := svc.processResults(context.Background(), results, done)

    assert.Equal(t, 1, res.SuccessCount)
    assert.Equal(t, 1, res.ErrorCount)
}

func TestProcessResults_ResultsOnly(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    results := make(chan *workerResult, 2)
    done := make(chan bool, 1)

    results <- &workerResult{Text: "OK", Error: nil}
    results <- &workerResult{Text: "Err", Error: assert.AnError}
    close(results)

    // Ждём завершения
    go func() {
        time.Sleep(10 * time.Millisecond)
        close(done)
    }()

    res := svc.processResults(context.Background(), results, done)

    assert.Equal(t, 1, res.SuccessCount)
    assert.Equal(t, 1, res.ErrorCount)
}

func TestProcessResults_ContextDone(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    results := make(chan *workerResult, 1)
    done := make(chan bool, 1)

    go func() {
        time.Sleep(10 * time.Millisecond)
        close(done)
    }()

    res := svc.processResults(ctx, results, done)

    assert.Equal(t, 0, res.SuccessCount)
    assert.Equal(t, 0, res.ErrorCount)
}

func TestProcessResults_ContextDone_Timeout(t *testing.T) {
    repo := &MockNotificationRepository{}
    sender := &MockNotificationSender{}
    svc := NewNotificationService(repo, sender, nil)

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    results := make(chan *workerResult, 1)
    done := make(chan bool) // не закрываем → сработает таймаут

    close(results)

    res := svc.processResults(ctx, results, done)

    assert.Equal(t, 0, res.SuccessCount)
    assert.Equal(t, 0, res.ErrorCount)
}