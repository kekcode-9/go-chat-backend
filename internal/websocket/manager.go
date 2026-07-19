package websocket

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	
	"github.com/kekcode-9/go-chat-backend/internal/models"
)

/*
Maintains device to ws client map,
registers/unregisters clients
takes messages from RouteMessage channel and
writes them to target clients' send channel
*/

type WsConManager struct{
	// device_id -> websocket client
	deviceWsCon map[uuid.UUID]*WsClient

	Register chan *WsClient

	Unregister chan *WsClient

	RouteMessage chan models.ChatMessage
}

func NewWsConManager() *WsConManager {
	return &WsConManager{
		deviceWsCon: make(map[uuid.UUID]*WsClient),

		Register: make(chan *WsClient),

		Unregister: make(chan *WsClient),

		RouteMessage: make(chan models.ChatMessage),
	}
}

func (m *WsConManager) Run() {
	for {
		select {
		case client := <-m.Register:
			m.deviceWsCon[client.DeviceID] = client

			log.Printf("Registered device %s", client.DeviceID,)

		case client := <-m.Unregister:
			if _, ok := m.deviceWsCon[client.DeviceID]; ok {
				delete(
					m.deviceWsCon,
					client.DeviceID,
				)

				close(client.Send)

				log.Printf("Unregistered device %s", client.DeviceID,)
			}

		case chatMessage := <-m.RouteMessage:
			for _, deviceID := range chatMessage.TargetDeviceIDs {
				client, ok := m.deviceWsCon[deviceID]

				if !ok {
					continue
				}

				payload, err := json.Marshal(chatMessage)

				if err != nil {
					log.Println("marshal error:", err)
					continue
				}

				select {
				case client.Send <- payload:

				default:
					log.Printf("send buffer full for device %s", deviceID)
				}
			}
		}
	}
}