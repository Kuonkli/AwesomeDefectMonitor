package main

import (
	"awesome-defect-tracker/projects-service/internal/models"
	"log"
	"net"
	"os"

	"awesome-defect-tracker/projects-service/internal/clients"
	"awesome-defect-tracker/projects-service/internal/repository"
	"awesome-defect-tracker/projects-service/internal/service"
	"awesome-defect-tracker/projects-service/pkg/database"
	pb "awesome-defect-tracker/shared/protogen/project"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	// Загрузка .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Подключение к БД
	dbConfig := database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "projects_db"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Автомиграция
	if err := db.AutoMigrate(
		&models.Project{},
		&models.ProjectEngineer{},
		&models.Defect{},
		&models.Comment{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database connected and migrated")

	// Подключаемся к User Service
	userServiceAddr := getEnv("USER_SERVICE_ADDR", "localhost:50051")
	userClient, err := clients.NewUserClient(userServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to User Service: %v", err)
		log.Println("Project Service will start without User Service integration")
	}

	// Инициализация репозитория и сервиса
	repo := repository.NewRepository(db)
	projectService := service.NewProjectService(repo, userClient)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()
	pb.RegisterProjectServiceServer(grpcServer, projectService)

	// Запуск сервера
	port := getEnv("PORT", "50052")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Project Service (gRPC) listening on :%s", port)
	log.Printf("Connected to User Service at: %s", userServiceAddr)

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
