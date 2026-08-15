package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
	"github.com/kekcode-9/go-chat-backend/internal/conversations"
	"github.com/kekcode-9/go-chat-backend/internal/message"
	"github.com/kekcode-9/go-chat-backend/internal/users"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"

	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
	"github.com/kekcode-9/go-chat-backend/internal/platform/db"
	"github.com/kekcode-9/go-chat-backend/internal/platform/redis"
	"github.com/kekcode-9/go-chat-backend/internal/platform/server"

	_ "github.com/kekcode-9/go-chat-backend/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Go Chat Backend API
// @version 1.0
// @description API documentation for Go Chat Backend.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your bearer token in the format: Bearer <token>
func main() {
	// --------------------------------------------
	// Load configuration
	// --------------------------------------------

	cfg := config.Load()

	// --------------------------------------------
	// Create shared dependencies
	// --------------------------------------------

	redisClient := redis.NewClient(cfg)

	pool, err := db.NewPool(context.Background(), cfg)

	if err != nil {
		log.Fatal(err)
	}

	defer pool.Close()

	// -------------------------------------------------
	// Wire in services with dependencies
	// -------------------------------------------------
	auth.Init(cfg)

	wsConManager := websocket.NewWsConManager(redisClient)

	conversationService := conversations.NewConversationService(pool)

	userService := users.NewUserService(pool)

	messageService := message.NewMessageService(
		cfg.BackendID,
		pool,
		redisClient,
		wsConManager,
		userService.Repo,
		conversationService.Repo,
	)

	AuthService := auth.NewAuthService(pool, userService.Repo)

	// --------------------------------------------
	// Start long-running background workers
	// --------------------------------------------

	go wsConManager.Run()

	go messageService.Run(context.Background())

	// --------------------------------------------
	// Configure HTTP routes
	// --------------------------------------------

	httpServer := server.CreateServer(
		cfg,
		redisClient,
		wsConManager,
		messageService,
		userService,
		AuthService,
		conversationService,
	)

	swaggerMux := http.NewServeMux()
	swaggerMux.Handle("GET /swagger/", httpSwagger.WrapHandler)
	swaggerMux.Handle("/", httpServer.Handler)
	httpServer.Handler = swaggerMux

	go func() {
		log.Printf("Backend %s listening on %s",
			cfg.BackendID,
			cfg.HTTPAddr,
		)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// --------------------------------------------
	// Graceful shutdown
	// --------------------------------------------

	stop := make(chan os.Signal, 1)

	signal.Notify(
		stop,
		os.Interrupt,
		syscall.SIGTERM,
	)

	<-stop

	log.Println("Shutting down...")

	httpServer.Shutdown(context.Background())

	redisClient.Close()
}
