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
	"github.com/kekcode-9/go-chat-backend/internal/platform/repository"
	"github.com/kekcode-9/go-chat-backend/internal/platform/server"
)

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

	// TODO: remove this
	repo := repository.NewMockRepository()

	// TODO: remove later and have proper device registration mechanism
	err = redisClient.HSet(
		context.Background(),
		"DeviceConRegistry",

		repo.AliceAndroid.String(), "backend-1",
		repo.BobMacbook.String(), "backend-1",
		repo.BobIPhone.String(), "backend-1",
	).Err()

	if err != nil {
		log.Fatal(err)
	}

	// -------------------------------------------------
	// Wire in services with dependencies
	// -------------------------------------------------
	auth.Init(cfg)

	wsConManager := websocket.NewWsConManager()

	messageService := message.NewMessageService(
		cfg.BackendID,
		repo,
		redisClient,
		wsConManager,
	)

	userService := users.NewUserService(pool)

	AuthService := auth.NewAuthService(pool, userService.Repo)

	conversationService := conversations.NewConversationService()

	// TODO: remove this
	log.Println("========== TEST USERS ==========")

	log.Println("Conversation :", repo.ConversationID)

	log.Println("Alice User   :", repo.AliceUserID)
	log.Println("Alice Device :", repo.AliceAndroid)

	log.Println("Bob User     :", repo.BobUserID)
	log.Println("Bob Macbook  :", repo.BobMacbook)
	log.Println("Bob iPhone   :", repo.BobIPhone)

	log.Println("===============================")

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
		wsConManager,
		messageService,
		userService,
		AuthService,
		conversationService,
	)

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
