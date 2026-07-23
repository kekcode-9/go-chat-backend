package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/kekcode-9/go-chat-backend/internal/models"
)

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

	maxMessageSize = 1024 * 64
)

type WsClient struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID

	Conn *websocket.Conn

	WsConManager *WsConManager

	MessageHandler IncomingMessageHandler

	Send chan []byte
}

func NewWsClient(
	userID uuid.UUID,
	deviceID uuid.UUID,
	conn *websocket.Conn,
	manager *WsConManager,
	handler IncomingMessageHandler,
) *WsClient {
	return &WsClient{
		UserID:         userID,
		DeviceID:       deviceID,
		Conn:           conn,
		WsConManager:   manager,
		MessageHandler: handler,
		Send:           make(chan []byte, 256),
	}
}

func (c *WsClient) Run() {
	c.WsConManager.Register <- c

	go c.WritePump()
	go c.ReadPump()
}

func (c *WsClient) ReadPump() {
	defer func() {
		c.WsConManager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)

	/*
		read deadline is a countdown timer that starts counting after the last message
		received from user over the socket and gets reset after the next message and
		starts counting again. If no message comes the connection is cut off.
	*/
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	/*
		But what if the user is sitting idle?
		To keep an idle connection alive the WritePump sends a ping to the user at every
		pingPeriod (this is a ping heartbeat).
		By websocket standard the user's websocket client has to respond with a pong and then
		the pong handler again resets the read deadline, not allowing the connection to die.
	*/
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.Conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Println("websocket read:", err)
			}

			break
		}

		var req models.IncomingMessage

		if err := json.Unmarshal(data, &req); err != nil {
			log.Println("Invalid websocket payload:", err)
			continue
		}

		err = c.MessageHandler.InMssgHandler(
			req.Payload,
			time.Now(),
			c.UserID,
			c.DeviceID,
			req.ConversationID,
		)

		if err != nil {
			log.Println("incoming message:", err)
		}
	}
}

func (c *WsClient) WritePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				c.Conn.WriteMessage(
					websocket.CloseMessage,
					[]byte{},
				)

				return
			}

			err := c.Conn.WriteMessage(
				websocket.TextMessage,
				message,
			)

			if err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err := c.Conn.WriteMessage(
				websocket.PingMessage,
				nil,
			); err != nil {
				return
			}
		}
	}
}
