package websocket

import (
	"time"

	"github.com/google/uuid"
)

/*
Anything satisfies the type IncomingMessageHandler as long as
it has the method InMssgHandler
*/
type IncomingMessageHandler interface {
	InMssgHandler(
		payload string,
		timestamp time.Time,
		senderUserID uuid.UUID,
		senderDeviceID uuid.UUID,
		conversationID uuid.UUID,
	) error

	ReadReceiptHandler(
		conversationID uuid.UUID,
		sequenceNo int64,
		mssgID uuid.UUID,
		senderUserID uuid.UUID,
	) error
}
