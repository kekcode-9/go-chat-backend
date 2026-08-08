package server

import (
	"context"
	"log"
	"net/http"

	ws "github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"
)

/*
Upgrades HTTP connection to websocket.
Creates the websocket client and runs the ReadPump and WritePump
*/

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	// For the PoC only
	// Tighten this in production
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func registerWebsocketRoutes(
	backendID string,
	redisClient *goredis.Client,
	mux *http.ServeMux,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
) {
	mux.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(
			backendID,
			redisClient,
			wsConManager,
			messageService,
			w,
			r,
		)
	})
}

/*
when ServeWs is called it is actually passed a *message.MessageService but the MessageService itself
satisfies the IncomingMessageHandler interface because it implements the InMssgHandler method and therefor
ServeWs function definition can receive messageService as a IncomingMessageHandler type
*/
// serveWs godoc
//
// @Summary Open websocket connection
// @Description Upgrades an authenticated HTTP request to a websocket connection for chat messages and read receipts.
// @Tags websocket
// @Security BearerAuth
// @Success 101 "Switching Protocols"
// @Failure 401 {string} string "missing auth claims"
// @Failure 500 {string} string "internal server error"
// @Router /ws/ [get]
func serveWs(
	backendID string,
	redisClient *goredis.Client,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
	w http.ResponseWriter,
	r *http.Request,
) {
	// -----------------------------------------
	// Get authenticated user/device
	// -----------------------------------------

	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(
			w,
			"missing auth claims",
			http.StatusUnauthorized,
		)
		return
	}

	userID := claims.UserID
	deviceID := claims.DeviceID

	// -----------------------------------------
	// Register this device with the backend
	// -----------------------------------------

	err := redisClient.HSet(
		context.Background(),
		"DeviceConRegistry",
		deviceID.String(),
		backendID,
	).Err()

	if err != nil {
		log.Printf("failed to register device in redis: %v", err)

		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	// -----------------------------------------
	// Upgrade HTTP -> WebSocket
	// -----------------------------------------

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket upgrader:", err)
		return
	}

	client := websocket.NewWsClient(
		userID,
		deviceID,
		conn,
		wsConManager,
		messageService,
	)

	client.Run()
}

/*
func main() {
	// 1. Create a new, isolated request router
	mux := http.NewServeMux()

	// 2. Register patterns and their handler functions
	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Welcome to the home page!")
	})

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.9Request) {
		fmt.Fprint(w, "User list API endpoint")
	})

	// 3. Pass your custom mux to the server
	http.ListenAndServe(":8080", mux)
}
*/
