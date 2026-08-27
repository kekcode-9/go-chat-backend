package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
)

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

	presenceTTL = 2 * pongWait // 120s

	maxMessageSize = 1024 * 64
)

type WsClient struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID

	Conn *websocket.Conn

	WsConManager *WsConManager

	MessageHandler IncomingMessageHandler

	Send chan []byte

	closeOnce sync.Once
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
	c.refreshPresence()

	go c.WritePump()
	go c.ReadPump()
}

func (c *WsClient) refreshPresence() {
	if err := c.WsConManager.redisClient.Set(
		context.Background(),
		"presence:user:"+c.UserID.String(),
		"online",
		presenceTTL,
	).Err(); err != nil {
		log.Printf("failed to refresh presence for user %s: %v", c.UserID, err)
	}
}

func (c *WsClient) Close() {
	c.closeOnce.Do(func() {
		c.Conn.Close()
	})
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
		c.refreshPresence()
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

		if req.Type == "" {
			log.Println("Invalid websocket payload: missing type")
			continue
		}

		if req.Type == "message" {
			if req.Payload == "" {
				log.Println("Invalid websocket payload: missing payload")
				errMssg := ErrorMessage{
					Type:    "error",
					Code:    "missing_payload",
					Message: "missing payload",
				}
				select {
				case c.Send <- []byte(errMssg.Message):
				default:
					log.Printf("send buffer full for device %s; dropping error message", c.DeviceID)
				}
				continue
			}

			if req.ConversationID == uuid.Nil {
				log.Println("Invalid websocket payload: missing conversation_id")
				errMssg := ErrorMessage{
					Type:    "error",
					Code:    "missing_conversation_id",
					Message: "missing conversation_id",
				}
				select {
				case c.Send <- []byte(errMssg.Message):
				default:
					log.Printf("send buffer full for device %s; dropping error message", c.DeviceID)
				}
				continue
			}

			if req.ClientMessageID == uuid.Nil {
				log.Println("Invalid websocket payload: missing client_message_id")
				errMssg := ErrorMessage{
					Type:    "error",
					Code:    "missing_client_message_id",
					Message: "missing client_message_id",
				}
				select {
				case c.Send <- []byte(errMssg.Message):
				default:
					log.Printf("send buffer full for device %s; dropping error message", c.DeviceID)
				}
				continue
			}

			err = c.MessageHandler.InMssgHandler(
				req.Payload,
				time.Now(),
				c.UserID,
				c.DeviceID,
				req.ConversationID,
				req.ClientMessageID,
			)

			if err != nil {
				if errors.Is(err, errors.New("message exists")) {
					errMssg := ErrorMessage{
						Type:    "error",
						Code:    "message_exists",
						Message: "message already exists",
					}
					select {
					case c.Send <- []byte(errMssg.Message):
					default:
						log.Printf("send buffer full for device %s; dropping error message", c.DeviceID)
					}
					continue
				}
				log.Println("incoming message:", err)
			} else {
				ack := SendMessageAck{
					Type:            "message_ack",
					ClientMessageID: req.ClientMessageID,
				}

				select {
				case c.Send <- []byte(ack.ClientMessageID.String()):
				default:
					log.Printf("send buffer full for device %s; dropping message ack", c.DeviceID)
				}
			}
		}

		if req.Type == "read_receipt" {
			err = c.MessageHandler.ReadReceiptHandler(
				req.ConversationID,
				req.ReadMessageSeqNo,
				req.ReadMessageID,
				c.UserID,
			)
		}

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
				// TODO: again put the message in send channel of this client?
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
