package websocket

import (
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

/*
Upgrades HTTP connection to websocket.
Creates the websocket client and runs the ReadPump and WritePump
*/

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	// For the PoC only
	// Tighten this in production
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

/*
when ServeWs is called it is actually passed a *message.MessageService but the MessageService itself
satisfies the IncomingMessageHandler interface because it implements the InMssgHandler method and therefor
ServeWs function definition can receive messageService as a IncomingMessageHandler type
*/
func ServeWs(
	wsConManager *WsConManager,
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

	// ------------------------------------------
	// Create websocket client
	// ------------------------------------------

	client := &WsClient{
		UserID:         userID,
		DeviceID:       deviceID,
		Conn:           conn,
		WsConManager:   wsConManager,
		MessageHandler: messageService,
		Send:           make(chan []byte, 256),
	}

	// ------------------------------------------
	// Register client
	// ------------------------------------------

	wsConManager.Register <- client

	// ------------------------------------------
	// Start pumps
	// ------------------------------------------

	go client.WritePump()

	go client.ReadPump()
}
