package main

import (
	"log"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"
	"health-go-backend/routes"
	"health-go-backend/services"
)

func main() {
	cfg := config.LoadConfig()
	db := config.ConnectDatabase(cfg)

	if err := db.AutoMigrate(&models.User{}, &models.Reading{}, &models.Report{}, &models.AsyncJob{}); err != nil {
		log.Fatalf("failed to run automigrate: %v", err)
	}

	aiService := services.NewAIService(cfg, db)
	pdfService := services.NewPDFService(cfg, db)
	jobService := services.NewAsyncJobService(db, aiService, pdfService)

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := jobService.ProcessDueJobs(25); err != nil {
				log.Printf("async job processor error: %v", err)
			}
		}
	}()

	r := routes.NewRouter(cfg, db)
	log.Printf("smart health backend listening on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
