package auth

import (
	"database/sql"
	"go.uber.org/zap"
)

type Repository interface {
}

type repository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewRepository(db *sql.DB, logger *zap.Logger) Repository {
	return &repository{
		db:     db,
		logger: logger,
	}
}
