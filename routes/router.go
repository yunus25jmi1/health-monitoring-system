package routes

import (
	"net/http"

	"health-go-backend/config"
	"health-go-backend/handlers"
	"health-go-backend/middleware"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.RateLimit(cfg.RateLimitRPM))
	corsConfig := cors.Config{
		AllowMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization", "X-Device-Key"},
	}
	if len(cfg.AllowedOrigin) == 1 && cfg.AllowedOrigin[0] == "*" {
		corsConfig.AllowAllOrigins = true
	} else {
		corsConfig.AllowOrigins = cfg.AllowedOrigin
	}
	r.Use(cors.New(corsConfig))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := handlers.NewAuthHandler(cfg, db)
	aiService := services.NewAIService(cfg, db)
	pdfService := services.NewPDFService(cfg, db)
	jobService := services.NewAsyncJobService(db, aiService, pdfService)
	readingHandler := handlers.NewReadingHandler(db, aiService, jobService)
	reportHandler := handlers.NewReportHandler(cfg, db, pdfService, jobService)
	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
		}

		api.POST("/readings", middleware.DeviceAuth(cfg.DeviceSecret), readingHandler.CreateReading)
		api.GET("/readings/:patient_id", middleware.JWTAuth(cfg, models.RoleDoctor), readingHandler.ListByPatient)
		api.GET("/readings/latest/:patient_id", middleware.JWTAuth(cfg, models.RoleDoctor), readingHandler.LatestByPatient)

		reports := api.Group("/reports", middleware.JWTAuth(cfg, models.RoleDoctor))
		{
			reports.GET("/pending", reportHandler.Pending)
			reports.GET("/:id", reportHandler.GetByID)
			reports.PATCH("/:id", reportHandler.Patch)
			reports.POST("/:id/approve", reportHandler.Approve)
			reports.GET("/patient/:patient_id", reportHandler.ListByPatient)
		}

		api.GET("/reports/:id/pdf", middleware.JWTAuth(cfg, models.RoleDoctor, models.RolePatient), reportHandler.StreamPDF)
	}

	return r
}
