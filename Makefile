.PHONY: all build run run-http run-grpc run-all test clean proto client

all: proto build

# Генерация protobuf файлов
proto:
	@echo "🚀 Генерация protobuf файлов..."
	protoc -I api/proto \
	       --go_out=pkg/grpc \
	       --go_opt=paths=source_relative \
	       --go-grpc_out=pkg/grpc \
	       --go-grpc_opt=paths=source_relative \
	       api/proto/monitoring.proto
	@echo "✅ Protobuf файлы сгенерированы"

# Сборка всех компонентов
build:
	@echo "🔨 Сборка всех компонентов..."
	go build -o bin/http-server main.go
	go build -o bin/grpc-server cmd/grpc-server/main.go
	go build -o bin/all-servers cmd/all-servers/main.go
	go build -o bin/grpc-client cmd/grpc-client/main.go
	@echo "✅ Все компоненты собраны"

# Запуск только HTTP сервера
run: run-http

run-http:
	@echo "🚀 Запуск HTTP сервера..."
	go run main.go

# Запуск только gRPC сервера
run-grpc:
	@echo "🚀 Запуск gRPC сервера..."
	go run cmd/grpc-server/main.go

# Запуск всех серверов
run-all:
	@echo "🚀 Запуск HTTP и gRPC серверов..."
	go run cmd/all-servers/main.go

# Запуск gRPC клиента
client:
	@echo "🔧 Запуск gRPC клиента..."
	go run cmd/grpc-client/main.go

# Тестирование
test:
	@echo "🧪 Запуск тестов..."
	go test ./...

# Очистка
clean:
	@echo "🧹 Очистка..."
	rm -rf bin/
	rm -f pkg/grpc/*.pb.go
	@echo "✅ Очистка завершена"

# Пересборка
rebuild: clean proto build

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  make run-http    - запуск только HTTP сервера"
	@echo "  make run-grpc    - запуск только gRPC сервера"
	@echo "  make run-all     - запуск HTTP и gRPC серверов вместе"
	@echo "  make client      - запуск gRPC клиента для тестирования"
	@echo "  make build       - сборка всех компонентов"
	@echo "  make proto       - генерация protobuf файлов"
	@echo "  make test        - запуск тестов"
	@echo "  make clean       - очистка"