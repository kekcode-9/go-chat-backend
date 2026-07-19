package server

import (
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/message"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"
)

func NewRouter(
	wsConManager *websocket.WsConManager,
	messageService *message.MessageService,
) http.Handler {
	/*
	creates an instance of a HTTP request router (officially called a 
	"multiplexer" or "mux" in Go terminology)
	It matches the URL path of incoming HTTP requests against a list of 
	patterns you have registered, and redirects the request to the correct 
	handler function.
	*/
	mux := http.NewServeMux()

	registerWebsocketRoutes(
		mux,
		wsConManager,
		messageService,
	)

	return mux
}

func registerWebsocketRoutes(
	mux *http.ServeMux,
	wsConManager *websocket.WsConManager,
	messageService *message.MessageService,
) {
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(
			wsConManager,
			messageService,
			w,
			r,
		)
	})
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