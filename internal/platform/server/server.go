package server

import (
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
	"github.com/kekcode-9/go-chat-backend/internal/conversations"
	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
	"github.com/kekcode-9/go-chat-backend/internal/users"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"
)

func CreateServer(
	cfg *config.Config,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
	userService *users.UserService,
	AuthService *auth.AuthService,
	conversationService *conversations.ConversationService,
) *http.Server {
	router := http.NewServeMux()

	registerRoutes(
		router,
		wsConManager,
		messageService,
		userService,
		AuthService,
		conversationService,
	)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
	}
}

func registerRoutes(
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

	registerWebsocketRoutes(
		router,
		wsConManager,
		messageService,
	)
}
