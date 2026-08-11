package server

import (
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
	"github.com/kekcode-9/go-chat-backend/internal/conversations"
	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
	"github.com/kekcode-9/go-chat-backend/internal/users"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"
	goredis "github.com/redis/go-redis/v9"
)

func CreateServer(
	cfg *config.Config,
	redisClient *goredis.Client,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
	userService *users.UserService,
	AuthService *auth.AuthService,
	conversationService *conversations.ConversationService,
) *http.Server {
	router := http.NewServeMux()

	registerRoutes(
		cfg,
		redisClient,
		router,
		wsConManager,
		messageService,
		userService,
		AuthService,
		conversationService,
	)

	handler := auth.AuthMiddleware(router)
	handler = CORSMiddleware(handler)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}
}

func registerRoutes(
	cfg *config.Config,
	redisClient *goredis.Client,
	router *http.ServeMux,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
	userService *users.UserService,
	AuthService *auth.AuthService,
	conversationService *conversations.ConversationService,
) {
	userService.RegisterRoutes(router)

	AuthService.RegisterRoutes(router)

	conversationService.RegisterRoutes(router)

	if messageRoutes, ok := messageService.(interface {
		RegisterRoutes(*http.ServeMux)
	}); ok {
		messageRoutes.RegisterRoutes(router)
	}

	registerWebsocketRoutes(
		cfg.BackendID,
		redisClient,
		router,
		wsConManager,
		messageService,
	)
}
