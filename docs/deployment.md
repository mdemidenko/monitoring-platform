# Deployment Guide

## Локальная разработка

### Предварительные требования
- Docker 20.10+
- Docker Compose 2.0+
- Go 1.25.1+ (опционально для разработки)


### Быстрый старт
```bash
# Клонирование репозитория
git clone https://github.com/mdemidenko/monitoring-platform.git
cd monitoring-platform

# Запуск всех сервисов
docker-compose up -d

# Проверка работы
curl http://localhost:8080/api/v1/health
```

### Доступные сервисы
- **notifier**: http://localhost:8080
- **PostgreSQL**: http://localhost:5432
- **monitor**: http://localhost:8081


## Конфигурация

### Environment Variables

#### monitor
```bash
будет добавлено
```

#### notifier
```bash
будет добавлено
```


### Docker Compose Configuration
```yaml
Будет добавлено в процессе разработки
```

### Health Checks

Все сервисы предоставляют health endpoints:

#### HTTP Health Checks
```bash
# Основной health check
curl http://localhost:8080/health
curl http://localhost:8081/health

# Проверка готовности
curl http://localhost:8080/health/ready
curl http://localhost:8081/health/ready

# Проверка живучести
curl http://localhost:8080/health/live
curl http://localhost:8081/health/live
```
