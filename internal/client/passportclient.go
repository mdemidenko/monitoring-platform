package client

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "time"
)

type PassportClient struct {
    baseURL string
    client  *http.Client
}

type ReleaseInfo struct {
    ID         int      `json:"id"`
    Status     string   `json:"status"`
    Clusters   []string `json:"clusters"`
    DockerImage string  `json:"dockerImage"`
    // Добавь другие поля при необходимости
}

func NewPassportClient(baseURL string) *PassportClient {
    return &PassportClient{
        baseURL: baseURL,
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (c *PassportClient) GetLatestReleases(ctx context.Context, tenant, service string) ([]ReleaseInfo, error) {
    // Экранируем tenant и service на случай спецсимволов
    encodedTenant := url.QueryEscape(tenant)
    encodedService := url.QueryEscape(service)

    // Формируем URL
    requestURL := fmt.Sprintf(
        "%s/services/tenant/%s/service/%s/release?page=1&per_page=5",
        c.baseURL, encodedTenant, encodedService,
    )

    log.Printf("🔍 Запрос к внешней системе: GET %s", requestURL)

    req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
    if err != nil {
        log.Printf("❌ Ошибка создания запроса: %v", err)
        return nil, err
    }
    req.Header.Set("Accept", "application/json")

    resp, err := c.client.Do(req)
    if err != nil {
        log.Printf("❌ Ошибка выполнения запроса: %v", err)
        return nil, err
    }
    defer resp.Body.Close()

    // Читаем тело ответа
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("❌ Ошибка чтения тела ответа: %v", err)
        return nil, err
    }

    log.Printf("📩 Ответ от внешней системы: [%d] %s", resp.StatusCode, string(body))

    // Проверяем статус
    if resp.StatusCode != http.StatusOK {
        // Даже если ошибка — попробуем прочитать JSON ошибки
        if len(body) > 0 {
            var errorResp interface{}
            if json.Unmarshal(body, &errorResp) == nil {
                log.Printf("❌ Тело ошибки: %+v", errorResp)
            }
        }
        return nil, fmt.Errorf("passport API returned %d", resp.StatusCode)
    }

    // Пустой ответ
    if len(body) == 0 {
        log.Printf("⚠️  Получен пустой ответ для %s/%s", tenant, service)
        return []ReleaseInfo{}, nil
    }

    // Парсим JSON
    var releases []ReleaseInfo
    if err := json.Unmarshal(body, &releases); err != nil {
        log.Printf("❌ Ошибка парсинга JSON: %v\nТело: %s", err, string(body))
        return nil, err
    }

    log.Printf("✅ Успешно получено %d релизов для %s/%s", len(releases), tenant, service)

    return releases, nil
}