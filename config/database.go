package config

import (
	"log"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectDatabase(cfg Config) *gorm.DB {
	// Add Supabase connection pool parameters to DSN
	dsn := cfg.DBDSN
	if !containsParam(dsn, "pool_mode") {
		if strings.Contains(dsn, "?") {
			dsn += "&pool_mode=transaction"
		} else {
			dsn += "?pool_mode=transaction"
		}
	}
	if !containsParam(dsn, "connection_limit") {
		dsn += "&connection_limit=10"
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		PrepareStmt: false,
		// Raise slow query threshold to 500ms to reduce log spam (was 200ms)
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to access sql db: %v", err)
	}

	// Optimize connection pool for Supabase PgBouncer
	sqlDB.SetMaxOpenConns(10)      // Matches connection_limit parameter
	sqlDB.SetMaxIdleConns(5)       // Keep 5 idle connections ready
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // Recycle connections

	log.Println("✅ Database connected with Supabase optimizations:")
	log.Println("   - Pool mode: transaction (PgBouncer)")
	log.Println("   - Connection limit: 10")
	log.Println("   - Partial index: idx_async_jobs_pending (status='pending')")
	log.Println("   - Job poll interval: 15s (reduced from 5s)")
	log.Println("   - Slow query threshold: 500ms (reduced log spam)")

	return db
}

// containsParam checks if DSN already contains a specific parameter
func containsParam(dsn, param string) bool {
	return strings.Contains(dsn, "?"+param+"=") || strings.Contains(dsn, "&"+param+"=")
}
