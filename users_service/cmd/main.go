package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	pb "awesome-defect-tracker/shared/protogen/user"
	"awesome-defect-tracker/user-service/internal/models"
	"awesome-defect-tracker/user-service/internal/repository"
	"awesome-defect-tracker/user-service/internal/service"
	"awesome-defect-tracker/user-service/pkg/database"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	log.Printf("Loading private key from: %s", path)

	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %v", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	log.Printf("PEM block type: %s", block.Type)

	if block.Type == "PRIVATE KEY" {
		log.Println("Parsing PKCS8 format...")
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8: %v", err)
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not RSA")
		}

		log.Println("Successfully loaded PKCS8 RSA private key")
		return rsaKey, nil
	}

	if block.Type == "RSA PRIVATE KEY" {
		log.Println("Parsing PKCS1 format...")
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS1: %v", err)
		}
		log.Println("Successfully loaded PKCS1 RSA private key")
		return key, nil
	}

	return nil, fmt.Errorf("unsupported key type: %s", block.Type)
}

func findPrivateKey() (*rsa.PrivateKey, string, error) {
	// Получаем абсолютный путь к исполняемому файлу
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)

	// Определяем корень проекта
	projectRoot := filepath.Join(exeDir, "..", "..")
	if _, err := os.Stat(filepath.Join(projectRoot, "keys")); os.IsNotExist(err) {
		// Если запускаем через go run из user-service/
		projectRoot = filepath.Join(exeDir, "..", "..", "..")
	}

	keysDir := filepath.Join(projectRoot, "keys")
	privateKeyPath := filepath.Join(keysDir, "private.pem")

	log.Printf("Looking for private key at: %s", privateKeyPath)

	// Проверяем существование файла
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		// Пробуем найти keys в разных местах
		possibleDirs := []string{
			filepath.Join(projectRoot, "keys"),
			filepath.Join(exeDir, "..", "..", "..", "keys"), // Из user-service/cmd/
			filepath.Join(exeDir, "..", "..", "keys"),       // Из user-service/
			filepath.Join(exeDir, "keys"),                   // В текущей директории
			"/home/artem/GoProjects/AwesomeDefectTracker/keys",
		}

		for _, dir := range possibleDirs {
			path := filepath.Join(dir, "private.pem")
			log.Printf("Trying: %s", path)
			if _, err := os.Stat(path); err == nil {
				privateKeyPath = path
				log.Printf("Found key at: %s", privateKeyPath)
				break
			}
		}
	}

	// Загружаем ключ
	key, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load private key from %s: %v", privateKeyPath, err)
	}

	return key, privateKeyPath, nil
}

func main() {
	// Загрузка .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Поиск приватного ключа
	privateKey, keyPath, err := findPrivateKey()
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}
	log.Printf("Loaded private key from: %s", keyPath)

	// Подключение к БД
	dbConfig := database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "users_db"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	var db *gorm.DB
	db, err = database.Connect(dbConfig)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v", err)
		return
	} else {
		if err := db.AutoMigrate(&models.User{}); err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}
		log.Println("Database connected and migrated")
	}

	// Инициализация репозитория
	userRepo := repository.NewUserRepository(db)

	// Создание сервиса
	userService := service.NewUserService(userRepo, privateKey)
	grpcAdapter := service.NewGrpcUserServiceAdapter(userService)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, grpcAdapter)

	// Запуск сервера
	port := getEnv("PORT", "50051")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("User Service (gRPC) listening on :%s", port)
	log.Printf("Database: %s@%s:%s/%s",
		dbConfig.User, dbConfig.Host, dbConfig.Port, dbConfig.DBName)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
