package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dbURL := os.Getenv("DB_URL")

	if dbURL == "" {
		// return nil, fmt.Errorf("DB url not found")
		dbURL = cfg.DBURL
	}

	return pgxpool.New(ctx, dbURL)
}
