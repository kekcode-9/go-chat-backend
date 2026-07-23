package server

import "net/http"

func registerAuthRoutes(
	mux *http.ServeMux,
) {
	mux.HandleFunc("POST /auth/signup/", func(w http.ResponseWriter, r *http.Request) {
		/*
		* r.Body should contain user_name, email, password and a device_id
		* New entries to be created in users, devices and refresh_sessions table
		 */
	})

	mux.HandleFunc("POST /auth/login/", func(w http.ResponseWriter, r *http.Request) {
		/*
		* r.Body should contain email, password and a device_id
		* Validate password and fetch user_id
		* A new entry to be created in refresh_sessions
		* if the device_id is not found in the devices table with this same user id then
		* create a new entry in devices table
		 */
	})

	mux.HandleFunc("POST /auth/refresh/", func(w http.ResponseWriter, r *http.Request) {})

	mux.HandleFunc("POST /auth/logout/", func(w http.ResponseWriter, r *http.Request) {})
}
