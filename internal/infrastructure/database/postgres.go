package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/syahrulfahmi/kolaboria-api/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewPostgresConnection(cfg config.DatabaseConfig, debug bool) (*gorm.DB, error) {
	var logLevel gormlogger.LogLevel
	if debug {
		logLevel = gormlogger.Info
	} else {
		logLevel = gormlogger.Error
	}

	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	}

	slog.Info("connecting to postgresql...", "host", cfg.Host, "port", cfg.Port, "database", cfg.Name)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Connection Pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test connection with ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("successfully connected to database")
	return db, nil
}
