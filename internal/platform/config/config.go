package config

import "os"

type Config struct {
	BackendID string
	HTTPAddr  string
	RedisAddr string
	JWTSecret string
	DBURL     string
}

func Load() *Config {
	return &Config{
		BackendID: getEnv("BACKEND_ID", "backend-1"),
		HTTPAddr:  getEnv("HTTP_ADDR", ":8080"),
		RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret: "A_very_long_string",
		DBURL:     "postgres://chatuser:chatpass@localhost:5432/chatdb?sslmode=disable",
	}
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)

	if val == "" {
		return fallback
	}

	return val
}
