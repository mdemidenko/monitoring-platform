# Monitoring platform

[![Go Version](https://img.shields.io/badge/Go-1.25.3+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)


Система мониторинга соответствия микросервисов политикам развёртывания в нескольких дата-центрах

## 🚀 Быстрый старт

```bash
# Клонирование репозитория
git clone https://github.com/mdemidenko/monitoring-platform.git
cd monitoring-platform

# Запуск в Docker
docker-compose up -d

# Проверка работы
curl http://localhost:8080/health
curl http://localhost:8081/health

```

## 📖 Документация

- [ Техническое задание ](docs/specification.md) - полное описание проекта и архитектуры
- [ API Description ](docs/api-description.md) - документация по API
- [ Deployment Guide ](docs/deployment.md) - инструкции по развертыванию

## 🏗️ Архитектура

Проект построен на основе микросервисной архитектуры с использованием:

- **Go 1.25.3+** с чистой архитектурой
- **PostgreSQL**  для хранения данных
- **Docker** для контейнеризации

## 🛠️ Основные сервисы

- **Notifier** - сервис отправки уведомлений
- **Мonitor** - сервис мониторинга


## 📊 Функциональность

- ✅ Получение информации о сервисах из внешних источников
- ✅ Проверка полученных данных на соответствие заданным условиям
- ✅ Выявление ситуаций, требующих уведомления
- ✅ Подготовка структурированных сообщений для отправки
- ✅ Передача сформированных сообщений через API
- ✅ Health checks и мониторинг

## 📚 API Документация

### REST API
- [Полная документация API](docs/api-description.md)
- URL сервиса нотификации: `http://localhost:8081/api/v1`
- Формат: JSON


### Health Checks
```bash
# HTTP health check
curl http://localhost:8080/health
curl http://localhost:8081/health

```
