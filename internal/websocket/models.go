package websocket

import (
	"github.com/google/uuid"
)

/*
* Code options:
*
 */

type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SendMessageAck struct {
	Type            string    `json:"type"`
	ClientMessageID uuid.UUID `json:"client_message_id"`
}
