package message

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/kekcode-9/go-chat-backend/internal/auth"

	"github.com/google/uuid"
)

func (m *MessageService) RegisterRoutes(
	mux *http.ServeMux,
) {
	// To fetch all messages of a conversation
	mux.HandleFunc("GET /conversations/{id}/messages", m.getMessagesHandler)

	// handle read receipt submission over api call (for non-live messages)
	mux.HandleFunc("POST /conversations/{id}/messages/read-receipt", m.postReadReceiptHandler)
}

// getMessagesHandler godoc
//
// @Summary List conversation messages
// @Description Returns messages for a conversation. If before_seq is provided, messages before that sequence are returned. If after_seq is provided, messages after that sequence are returned. If neither is provided, the latest messages are returned.
// @Tags messages
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Param before_seq query int false "Return messages before this sequence number"
// @Param after_seq query int false "Return messages after this sequence number"
// @Param limit query int false "Maximum number of messages to return. Defaults to 100"
// @Success 200 {object} GetConversationMessagesResponse
// @Failure 400 {string} string "invalid request parameters"
// @Failure 401 {string} string "missing or invalid access token"
// @Failure 500 {string} string "internal server error"
// @Router /conversations/{id}/messages [get]
func (m *MessageService) getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* possible query params:
	* before_seq
	* after_seq
	* limit
	* mandatory path param:
	* id
	* --------------------------------
	* Behavior
	* If limit is not present then set default value to 100, else read value from query param
	* read conversation_id from the id path param
	* if neither before_seq, nor after_seq is present then call m.getMessages(conversation_id, limit, nil, true)
	* if either before_seq or after_seq is present then pass that as the third argument and if it
	* is before_seq then pass the fourth argument as false and if it is before_seq then pass the fourth argument
	* as true
	* if both before_seq and after_seq are present then send http error
	* if id is not present in the path then send http error
	* in success scenario, expect a response of type GetConversationMessagesResponse (from models.go of this package)
	 */
	query := r.URL.Query()

	conversationIDParam := r.PathValue("id")
	if conversationIDParam == "" {
		http.Error(
			w,
			"id path parameter is required",
			http.StatusBadRequest,
		)
		return
	}

	conversationID, err := uuid.Parse(conversationIDParam)
	if err != nil {
		http.Error(
			w,
			"invalid conversation_id",
			http.StatusBadRequest,
		)
		return
	}

	limit := 100
	if limitParam := query.Get("limit"); limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil || parsedLimit <= 0 {
			http.Error(
				w,
				"invalid limit",
				http.StatusBadRequest,
			)
			return
		}

		limit = parsedLimit
	}

	beforeSeqParam := query.Get("before_seq")
	afterSeqParam := query.Get("after_seq")

	if beforeSeqParam != "" && afterSeqParam != "" {
		http.Error(
			w,
			"before_seq and after_seq cannot both be present",
			http.StatusBadRequest,
		)
		return
	}

	var seqNo *int64
	getAfter := true

	if beforeSeqParam != "" {
		parsedSeqNo, err := strconv.ParseInt(beforeSeqParam, 10, 64)
		if err != nil {
			http.Error(
				w,
				"invalid before_seq",
				http.StatusBadRequest,
			)
			return
		}

		seqNo = &parsedSeqNo
		getAfter = false
	}

	if afterSeqParam != "" {
		parsedSeqNo, err := strconv.ParseInt(afterSeqParam, 10, 64)
		if err != nil {
			http.Error(
				w,
				"invalid after_seq",
				http.StatusBadRequest,
			)
			return
		}

		seqNo = &parsedSeqNo
		getAfter = true
	}

	messages, err := m.getMessages(
		conversationID,
		limit,
		seqNo,
		getAfter,
	)
	if err != nil {
		log.Printf("get messages failed: %v", err)

		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	resp := GetConversationMessagesResponse{
		ConversationID: conversationID,
		Messages:       messages,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("failed to encode get messages response: %v", err)
	}
}

func (m *MessageService) postReadReceiptHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(
			w,
			"missing auth claims",
			http.StatusUnauthorized,
		)
		return
	}

	conversationIDParam := r.PathValue("id")
	if conversationIDParam == "" {
		http.Error(
			w,
			"id path parameter is required",
			http.StatusBadRequest,
		)
		return
	}

	conversationID, err := uuid.Parse(conversationIDParam)
	if err != nil {
		http.Error(
			w,
			"invalid conversation_id",
			http.StatusBadRequest,
		)
		return
	}

	var req PostReadReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if req.MessageID == uuid.Nil {
		http.Error(
			w,
			"message_id is required",
			http.StatusBadRequest,
		)
		return
	}

	if req.SequenceNo <= 0 {
		http.Error(
			w,
			"sequence_no must be positive",
			http.StatusBadRequest,
		)
		return
	}

	if err := m.postReadReceipt(
		conversationID,
		claims.UserID,
		req.MessageID,
		req.SequenceNo,
	); err != nil {
		log.Printf("post read receipt failed: %v", err)

		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
