package main

import (
	"awesome-defect-tracker/gateway-service/internal/clients"
	"awesome-defect-tracker/gateway-service/internal/config"
	"awesome-defect-tracker/gateway-service/internal/handlers"
	"awesome-defect-tracker/gateway-service/internal/middleware"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация gRPC клиентов
	userClient, err := clients.NewUserClient(cfg.UserServiceAddr)
	if err != nil {
		log.Fatalf("Failed to create user client: %v", err)
	}
	defer userClient.Close()

	projectClient, err := clients.NewProjectClient(cfg.ProjectServiceAddr)
	if err != nil {
		log.Fatalf("Failed to create project client: %v", err)
	}
	defer projectClient.Close()

	// Инициализация middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.PublicKey, userClient)

	// Инициализация handler'ов
	authHandler := handlers.NewAuthHandler(userClient)
	userHandler := handlers.NewUserHandler(userClient)
	projectHandler := handlers.NewProjectHandler(projectClient)
	defectHandler := handlers.NewDefectHandler(projectClient)

	// Настройка роутера
	r := gin.Default()

	// Публичные роуты
	public := r.Group("/api/auth")
	{
		public.POST("/register", authHandler.Register)
		public.POST("/login", authHandler.Login)
		public.POST("/refresh", authHandler.Refresh)
	}

	// Защищённые роуты
	api := r.Group("/api")
	api.Use(authMiddleware.Auth())
	{
		// Профиль пользователя
		api.GET("/profile", userHandler.GetProfile)
		api.PUT("/profile", userHandler.UpdateProfile)
		api.PUT("/change-password", userHandler.ChangePassword)

		// Управление пользователями (админ)
		admin := api.Group("/admin/users")
		admin.Use(authMiddleware.Role("manager", "supervisor"))
		{
			admin.GET("/", userHandler.ListUsers)
			admin.GET("/:id", userHandler.GetUser)
			admin.PUT("/:id/role", userHandler.UpdateUserRole)
			admin.PUT("/:id/deactivate", userHandler.DeactivateUser)
			admin.PUT("/:id/activate", userHandler.ActivateUser)
		}

		// Проекты
		projects := api.Group("/projects")
		{
			projects.GET("/", projectHandler.ListProjects)
			projects.POST("/", authMiddleware.Role("manager", "supervisor"), projectHandler.CreateProject)
			projects.GET("/:id", projectHandler.GetProject)
			projects.PUT("/:id", authMiddleware.Role("manager", "supervisor"), projectHandler.UpdateProject)
			projects.DELETE("/:id", authMiddleware.Role("manager", "supervisor"), projectHandler.DeleteProject)
			projects.POST("/:id/engineers", authMiddleware.Role("manager", "supervisor"), projectHandler.AddEngineers)
			projects.DELETE("/:id/engineers/:engineer_id", authMiddleware.Role("manager", "supervisor"), projectHandler.RemoveEngineer)
			projects.GET("/:id/validate-access", projectHandler.ValidateAccess)
		}

		// Дефекты
		defects := api.Group("/defects")
		{
			defects.GET("/", defectHandler.ListDefects)
			defects.POST("/", defectHandler.CreateDefect)
			defects.GET("/:id", defectHandler.GetDefect)
			defects.PUT("/:id", defectHandler.UpdateDefect)
			defects.DELETE("/:id", authMiddleware.Role("manager", "supervisor"), defectHandler.DeleteDefect)
			defects.PUT("/:id/status", defectHandler.UpdateDefectStatus)
			defects.PUT("/:id/priority", defectHandler.UpdateDefectPriority)
			defects.PUT("/:id/assign", authMiddleware.Role("manager"), defectHandler.AssignEngineer)
		}

		// Комментарии к дефектам
		defects = api.Group("/defects/:id/comments")
		{
			defects.GET("/", defectHandler.GetComments)
			defects.POST("/", defectHandler.AddComment)
			defects.PUT("/:comment_id", defectHandler.UpdateComment)
			defects.DELETE("/:comment_id", defectHandler.DeleteComment)
		}
	}

	// Health checks
	r.GET("/health", func(c *gin.Context) {
		status := "healthy"
		services := make(map[string]string)

		// Проверка User Service
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if _, err := userClient.ValidateToken(ctx, "test"); err != nil {
			if strings.Contains(err.Error(), "connection") {
				status = "degraded"
				services["user"] = "unavailable"
			} else {
				services["user"] = "available"
			}
		} else {
			services["user"] = "available"
		}

		// Проверка Project Service
		ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel2()

		if err := projectClient.HealthCheck(ctx2); err != nil {
			services["project"] = "unavailable"
			status = "degraded"
		} else {
			services["project"] = "available"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    status,
			"timestamp": time.Now().Format(time.RFC3339),
			"services":  services,
		})
	})

	// Graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Gateway Service starting on port %s", cfg.Port)
		log.Printf("Connected to User Service at: %s", cfg.UserServiceAddr)
		log.Printf("Connected to Project Service at: %s", cfg.ProjectServiceAddr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
