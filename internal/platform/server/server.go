package server

import (
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"
)

func CreateServer(
	cfg *config.Config,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
) *http.Server {
	router := NewRouter(
		wsConManager,
		messageService,
	)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
	}
}

func NewRouter(
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
) *http.ServeMux {
	router := http.NewServeMux()

	registerRoutes(
		router,
		wsConManager,
		messageService,
	)

	return router
}

func registerRoutes(
	router *http.ServeMux,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
) {
	registerAuthRoutes(router)

	registerUserRoutes(router)

	registerConversationRoutes(router)

	registerWebsocketRoutes(
		router,
		wsConManager,
		messageService,
	)
}
