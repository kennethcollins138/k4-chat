package main

import (
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/kdot/k4-chat/backend/configs"
	"github.com/kdot/k4-chat/backend/internal/database"
)

func main() {
	// Configs and logger
	if err := configs.LoadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	config := configs.GetConfig()
	dbConfig := configs.GetDatabaseConfig()
	redisConfig := configs.GetRedisConfig()

	logger, err := NewLogger(config.Logger)
	if err != nil {
		log.Fatalf("Failed to initialize logger")
	}
	defer logger.Sync()

	// Database Initialization
	postgresDB, err := database.NewDB(dbConfig.Postgres, config.Envs.Postgres)
	if err != nil {
		logger.Sugar().Fatalf("Failed to establish Postgres Connection: %s\n", err)
	}
	defer postgresDB.Close()

	redisClient, err := database.NewClient(redisConfig, logger)
	if err != nil {
		logger.Sugar().Fatalf("Failed to initialize Redis Client: %s\n", err)
	}
	defer redisClient.Close()

	logger.Info("Application started successfully")
}

/*
===============================================================================
						UTIL
===============================================================================
*/

func NewLogger(cfg configs.LoggerConfig) (*zap.Logger, error) {
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(toZapLevel(cfg.Level)),
		Encoding:    "json",
		OutputPaths: []string{cfg.Output},
	}

	return config.Build()
}

func toZapLevel(lvl string) zapcore.Level {
	switch lvl {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
