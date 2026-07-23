package server

import (
	"log"
	"net/http"

	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"

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
	mux *http.ServeMux,
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
) {
	mux.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(
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
func serveWs(
	wsConManager *websocket.WsConManager,
	messageService IncomingMessageHandler,
	w http.ResponseWriter,
	r *http.Request,
) {
	// -----------------------------------------
	// Upgrade HTTP -> WebScoket
	// -----------------------------------------

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket upgrader:", err)
		return
	}

	// -----------------------------------------
	// For this iteration these come from query params
	// Later they will come from authentication / sessions
	// -----------------------------------------

	userIDStr := r.URL.Query().Get("user_id")
	deviceIDStr := r.URL.Query().Get("device_id")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		conn.Close()
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		conn.Close()
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
