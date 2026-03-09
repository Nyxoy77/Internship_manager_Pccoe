package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/yourusername/student-internship-manager/internal/client"
	"github.com/yourusername/student-internship-manager/internal/config"
	"github.com/yourusername/student-internship-manager/internal/database"
	"github.com/yourusername/student-internship-manager/internal/middleware"
	"github.com/yourusername/student-internship-manager/internal/service"
	"github.com/yourusername/student-internship-manager/internal/storage"
)

func main() {
	// Load configuration
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

	minioClient, err := storage.NewMinioClient()
	if err != nil {
		log.Fatalf("Error starting minio client: %v", err)
	}
	objectStorageService := service.NewObjectStorageService(db, minioClient)
	// Initialize services
	studentService := service.NewStudentService(db)
	authService := service.NewAuthService(db, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	internshipService := service.NewInternshipService(db, studentService)
	analyticsSvc := service.NewAnalyticsService(db)
	adminSvc := service.NewAdminService(db)
	userService := service.NewUserService(db)
	// Initialize handlers
	authHandler := client.NewAuthHandler(authService)
	studentHandler := client.NewStudentHandler(studentService)
	internshipHandler := client.NewInternshipHandler(internshipService)
	objectStorageHandler := client.NewCertificateClient(objectStorageService)
	analyticsHandler := client.NewAnalyticsHandler(analyticsSvc)
	studentAdminHandler := client.NewStudentAdminHandler(adminSvc)
	userHandler := client.NewUserHandler(userService)
	// Setup Gin router
	router := gin.Default()
	router.Use(middleware.RequestIDMiddleware())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8081"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
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
		api.POST("/login", middleware.LoginRateLimitMiddleware(cfg.LoginRateLimit, time.Duration(cfg.LoginRateWindowSecs)*time.Second), authHandler.Login)
		api.POST("/refresh", authHandler.Refresh)
		api.POST("/logout", authHandler.Logout)

		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(authService))
		{
			protected.GET("/student/:prn/summary", studentHandler.GetStudentSummary)
			protected.GET("/students", studentHandler.ListStudents)
			protected.GET("/reports/student-credits", studentHandler.ListStudentCreditReport)
			protected.GET("/reports/student-credits/export.csv", studentHandler.ExportStudentCreditReportCSV)
			protected.GET("/reports/student-credits/export.pdf", studentHandler.ExportStudentCreditReportPDF)
			protected.GET("/internships", internshipHandler.ListInternships)
			protected.GET("/internships/export.csv", internshipHandler.ExportInternshipsCSV)
			protected.GET("/internships/export.pdf", internshipHandler.ExportInternshipsPDF)
			protected.GET("/internship/:id/audit", internshipHandler.GetInternshipAudit)
			protected.POST("/changePassword", userHandler.ChangePassword)

			certificateRoutes := protected.Group("")
			certificateRoutes.Use(middleware.RequireRole("manager", "admin"))
			{
				certificateRoutes.POST("/internships/:internshipId/certificate", objectStorageHandler.UploadCertificate)
				certificateRoutes.DELETE("/internships/:internshipId/certificate", objectStorageHandler.RemoveCertificate)
				certificateRoutes.GET("/internships/:internshipId/certificate", objectStorageHandler.DownloadViewCertificate)
			}

			managerRoutes := protected.Group("")
			managerRoutes.Use(middleware.RequireRole("manager"))
			{
				managerRoutes.POST("/internship", internshipHandler.CreateInternship)
				managerRoutes.POST("/internships/upload", internshipHandler.BatchUploadInternships)
			}

			adminRoutes := protected.Group("")
			adminRoutes.Use(middleware.RequireRole("admin"))
			{
				adminRoutes.GET("/internships/pending", internshipHandler.GetPendingInternships)
				adminRoutes.POST("/internships/bulk-review", internshipHandler.BulkReviewInternships)
				adminRoutes.POST("/internship/:id/approve", internshipHandler.ApproveInternship)
				adminRoutes.POST("/internship/:id/reject", internshipHandler.RejectInternship)
				adminRoutes.POST("/createStudent", studentAdminHandler.CreateStudent)
				adminRoutes.POST("/createStudents/upload", studentAdminHandler.BatchUploadStudents)
				adminRoutes.POST("/createUser", userHandler.CreateUser)

			}

			analytics := protected.Group("/analytics")
			{
				analytics.GET("/avg-stipend", analyticsHandler.AvgStipend)
				analytics.GET("/paid-percentage", analyticsHandler.PaidPercentage)
				analytics.GET("/mode-distribution", analyticsHandler.ModeDistribution)
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
