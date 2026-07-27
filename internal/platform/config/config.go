package config

import "os"

type Config struct {
	BackendID string
	HTTPAddr  string
	RedisAddr string
	JWTSecret []byte
	DBURL     string
}

func Load() *Config {
	return &Config{
		BackendID: getEnv("BACKEND_ID", "backend-1"),

		HTTPAddr: getEnv(
			"HTTP_ADDR",
			":8080",
		),

		// Redis running in Docker, backend running on host.
		RedisAddr: getEnv(
			"REDIS_ADDR",
			"localhost:6379",
		),

		// JWT HMAC secret.
		JWTSecret: []byte(getEnv(
			"JWT_SECRET",
			"A_very_long_string",
		)),

		// PostgreSQL running in Docker, backend running on host.
		DBURL: getEnv(
			"DB_URL",
			"postgres://chatuser:chatpass@localhost:5432/chatdb?sslmode=disable",
		),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return fallback
}
