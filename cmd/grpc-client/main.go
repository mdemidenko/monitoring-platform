package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mdemidenko/monitoring-platform/config"
	monitoringGrpc "github.com/mdemidenko/monitoring-platform/pkg/grpc"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	// Парсим аргументы командной строки
	var configPath string
	var serverAddr string
	
	flag.StringVar(&configPath, "config", "", "Path to config.yml file")
	flag.StringVar(&serverAddr, "server", "localhost:9090", "gRPC server address")
	flag.Parse()

	// Загружаем конфигурацию
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Не удалось загрузить конфигурацию: %v", err)
	}

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// grpc.WithBlock(), // Убираем deprecated опцию
	)
	if err != nil {
		log.Fatalf("❌ Не удалось подключиться к серверу: %v", err)
	}
	defer func() {
    if err := conn.Close(); err != nil {
        log.Printf("Failed to close connection: %v", err)
    }
	}()

	// Создаем клиент
	client := monitoringGrpc.NewMonitoringServiceClient(conn)
	
	// Тестовый контекст
	testCtx, testCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer testCancel()

	// Тест 1: Аутентификация с данными из конфига
	fmt.Println("=== Тест 1: Аутентификация ===")
	loginResp, err := client.Login(testCtx, &monitoringGrpc.LoginRequest{
		Username: cfg.Auth.Login,
		Password: cfg.Auth.Password,
	})
	if err != nil {
		log.Fatalf("❌ Ошибка аутентификации: %v", err)
	}
	fmt.Printf("✅ Токен получен: %s\n", loginResp.Token)
	fmt.Printf("   Срок действия: %s\n", loginResp.ExpiresAt)
	fmt.Printf("   Пользователь: %s\n", cfg.Auth.Login)

	// Создаем контекст с метаданными для аутентификации
	authCtx := metadata.NewOutgoingContext(testCtx, metadata.Pairs(
		"authorization", "Bearer "+loginResp.Token,
	))

	// Запускаем тесты
	runAllTests(client, authCtx, cfg)
}

// loadConfig загружает конфигурацию
func loadConfig(configPath string) (*config.Config, error) {
	if configPath == "" {
		// Пробуем найти конфиг в стандартных местах
		return config.LoadConfig("")
	}
	return config.LoadConfig(configPath)
}

// runAllTests выполняет все тесты
func runAllTests(client monitoringGrpc.MonitoringServiceClient, authCtx context.Context, cfg *config.Config) {
	// Тест 2: Создание уведомления
	fmt.Println("\n=== Тест 2: Создание уведомления ===")
	notification := &monitoringGrpc.Notification{
		ChatId: cfg.Telegram.ChatID, // Используем chat_id из конфига
		Text:   "Тестовое сообщение через gRPC клиент",
	}
	
	createResp, err := client.CreateNotification(authCtx, notification)
	if err != nil {
		log.Printf("⚠️ Ошибка создания уведомления: %v", err)
	} else {
		fmt.Printf("✅ Уведомление создано: %s\n", createResp.Message)
		fmt.Printf("   Chat ID из конфига: %s\n", cfg.Telegram.ChatID)
	}

	// Тест 3: Получение списка уведомлений
	fmt.Println("\n=== Тест 3: Список уведомлений ===")
	listResp, err := client.ListNotifications(authCtx, &emptypb.Empty{})
	if err != nil {
		log.Printf("⚠️ Ошибка получения списка уведомлений: %v", err)
	} else {
		fmt.Printf("✅ Найдено уведомлений: %d\n", listResp.Count)
		for i, n := range listResp.Notifications {
			fmt.Printf("   %d. ChatID: %s, Text: %s\n", i+1, n.ChatId, n.Text)
		}
	}

	// Тест 4: Отправка уведомления
	fmt.Println("\n=== Тест 4: Отправка уведомления ===")
	sendResp, err := client.SendNotification(authCtx, &monitoringGrpc.SendRequest{
		ChatId: cfg.Telegram.ChatID, // Используем chat_id из конфига
		Text:   fmt.Sprintf("Тест отправки через gRPC клиент. Приложение: %s v%s", 
			cfg.App.Name, cfg.App.Version),
	})
	if err != nil {
		log.Printf("⚠️ Ошибка отправки уведомления: %v", err)
	} else {
		fmt.Printf("✅ Уведомление отправлено: %s\n", sendResp.Message)
		fmt.Printf("   Message ID: %d\n", sendResp.MessageId)
		fmt.Printf("   Chat ID: %s\n", sendResp.ChatId)
	}

	// Тест 5: Список отправленных уведомлений
	fmt.Println("\n=== Тест 5: Список отправленных уведомлений ===")
	sentListResp, err := client.ListSentNotifications(authCtx, &emptypb.Empty{})
	if err != nil {
		log.Printf("⚠️ Ошибка получения списка отправленных уведомлений: %v", err)
	} else {
		fmt.Printf("✅ Найдено отправленных уведомлений: %d\n", sentListResp.Count)
		for i, n := range sentListResp.SentNotifications {
			fmt.Printf("   %d. MessageID: %d, ChatID: %d\n", i+1, n.MessageId, n.ChatId)
		}
	}

	// Тест 6: Работа с Service
	fmt.Println("\n=== Тест 6: Создание сервиса ===")
	serviceResp, err := client.CreateService(authCtx, &monitoringGrpc.Service{
		Id:             1,
		Name:           fmt.Sprintf("Service from %s", cfg.App.Name),
		Tenant:         "Test Tenant",
		DeprecatedDate: "2024-12-31",
		BusinessLine:   "Test Business Line",
	})
	if err != nil {
		log.Printf("⚠️ Ошибка создания сервиса: %v", err)
	} else {
		fmt.Printf("✅ Сервис создан: %s\n", serviceResp.Message)
	}

	// Тест 7: Пакетная отправка
	fmt.Println("\n=== Тест 7: Пакетная отправка ===")
	batchResp, err := client.BatchSend(authCtx, &monitoringGrpc.BatchSendRequest{
		Messages: []*monitoringGrpc.SendRequest{
			{
				ChatId: cfg.Telegram.ChatID,
				Text:   fmt.Sprintf("Пакет 1 от %s", cfg.App.Name),
			},
			{
				ChatId: cfg.Telegram.ChatID,
				Text:   fmt.Sprintf("Пакет 2 от %s v%s", cfg.App.Name, cfg.App.Version),
			},
			{
				ChatId: cfg.Telegram.ChatID,
				Text:   fmt.Sprintf("Пакет 3. Окружение: %s", cfg.App.Environment),
			},
		},
		IntervalMs: 1000,
		Workers:    2,
	})
	if err != nil {
		log.Printf("⚠️ Ошибка пакетной отправки: %v", err)
	} else {
		fmt.Printf("✅ Пакетная отправка завершена: %s\n", batchResp.Message)
		fmt.Printf("   Всего: %d, Успешно: %d, Ошибок: %d\n", 
			batchResp.Total, batchResp.SuccessCount, batchResp.ErrorCount)
	}

	// Тест 8: Получение информации о приложении
	fmt.Println("\n=== Тест 8: Информация о приложении ===")
	fmt.Printf("   Название: %s\n", cfg.App.Name)
	fmt.Printf("   Версия: %s\n", cfg.App.Version)
	fmt.Printf("   Окружение: %s\n", cfg.App.Environment)
	fmt.Printf("   Telegram Chat ID: %s\n", cfg.Telegram.ChatID)
	fmt.Printf("   JWT Expiration: %d часов\n", cfg.Auth.JWTExpiration)

	fmt.Println("\n✅ Все тесты завершены!")
}