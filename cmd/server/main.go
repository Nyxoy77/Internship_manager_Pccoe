package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/student-internship-manager/internal/client"
	"github.com/yourusername/student-internship-manager/internal/config"
	"github.com/yourusername/student-internship-manager/internal/database"
	"github.com/yourusername/student-internship-manager/internal/middleware"
	"github.com/yourusername/student-internship-manager/internal/service"
	"github.com/yourusername/student-internship-manager/internal/storage"
)

func main() {
	// Load configuration
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	fmt.Println(string(hash))

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection
	db, err := database.InitDB(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	log.Println("Database connection established successfully")

	// Initialize services
	studentService := service.NewStudentService(db)
	authService := service.NewAuthService(db, cfg.JWTSecret)
	internshipService := service.NewInternshipService(db, studentService)

	// Initialize handlers
	authHandler := client.NewAuthHandler(authService)
	studentHandler := client.NewStudentHandler(studentService)
	internshipHandler := client.NewInternshipHandler(internshipService)

	// Setup Gin router
	router := gin.Default()
	minioClient, err := storage.NewMinioClient()
	if err != nil {
		log.Fatalf("Error starting minio client: %v", err)
	}
	objectStorageService := service.NewObjectStorageService(db, minioClient)

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8081"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// API routes
	api := router.Group("/api")
	{
		// Public routes
		api.POST("/login", authHandler.Login)
		api.POST("/upload", objectStorageService.UploadCertificateHandler)
		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(authService))
		{
			// Student endpoints (accessible by both admin and manager)
			protected.GET("/student/:prn/summary", studentHandler.GetStudentSummary)
			protected.GET("/students", studentHandler.ListStudents)

			// Manager-only endpoints (create internships)
			managerRoutes := protected.Group("")
			managerRoutes.Use(middleware.RequireRole("manager"))
			{
				managerRoutes.POST("/internship", internshipHandler.CreateInternship)
				managerRoutes.POST("/internships/upload", internshipHandler.BatchUploadInternships)
			}

			// Admin-only endpoints (approve/reject internships)
			adminRoutes := protected.Group("")
			adminRoutes.Use(middleware.RequireRole("admin"))
			{
				adminRoutes.GET("/internships/pending", internshipHandler.GetPendingInternships)
				adminRoutes.POST("/internship/:id/approve", internshipHandler.ApproveInternship)
				adminRoutes.POST("/internship/:id/reject", internshipHandler.RejectInternship)
			}
		}
	}

	// Start server
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
