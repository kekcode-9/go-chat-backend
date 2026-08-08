package redis

import (
	"context"
	"log"

	goredis "github.com/redis/go-redis/v9" // alias the package by name: goredis

	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
)

func NewClient(cfg *config.Config) *goredis.Client {
	client := goredis.NewClient(&goredis.Options{
		Addr: cfg.RedisAddr,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Unable to connect to Redis: %v", err)
	}

	log.Printf("Connected to Redis at %s", cfg.RedisAddr)

	return client
}