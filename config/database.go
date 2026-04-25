package config

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDatabase(cfg Config) *gorm.DB {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DBDSN,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		PrepareStmt: false,
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to access sql db: %v", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db
}
