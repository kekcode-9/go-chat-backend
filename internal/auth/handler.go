package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mileusna/useragent"
)

func (a *AuthService) RegisterRoutes(
	mux *http.ServeMux,
) {
	mux.HandleFunc("POST /auth/signup/", a.signupHandler)

	mux.HandleFunc("POST /auth/login/", a.loginHandler)

	mux.HandleFunc("POST /auth/refresh/", a.refreshHandler)

	mux.Handle("POST /auth/logout/", AuthMiddleware(http.HandlerFunc(a.logoutHandler)))
}

func (a *AuthService) signupHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req SignupRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	ua := useragent.Parse(r.UserAgent())

	deviceName := ua.Name +
		" - " + ua.Version +
		" - " + ua.OS +
		" - " + ua.Device

	deviceType := ua.Device
	if deviceType == "" {
		deviceType = "Unknown"
	}

	resp, err := a.signup(
		req.UserName,
		deviceName,
		deviceType,
		req.Email,
		req.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailAlreadyExists):
			http.Error(
				w,
				err.Error(),
				http.StatusConflict,
			)
		default:
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(struct {
		AccessToken string `json:"access_token"`
	}{
		AccessToken: resp.AccessToken,
	})

	if err != nil {
		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
	}
}

/*
If user is logging in for the first time from a device then they are expected to not send
a device_id in request body. In such case server itself will create the device_id
*/
func (a *AuthService) loginHandler(w http.ResponseWriter, r *http.Request) {

	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ua := useragent.Parse(r.UserAgent())

	deviceName := ua.Name +
		" - " + ua.Version +
		" - " + ua.OS +
		" - " + ua.Device

	deviceType := ua.Device
	if deviceType == "" {
		deviceType = "Unknown"
	}

	resp, err := a.login(
		req.Email,
		req.Password,
		req.DeviceID,
		&deviceName,
		&deviceType,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			http.Error(
				w,
				err.Error(),
				http.StatusNotFound,
			)
		case errors.Is(err, ErrInvalidCredentials):
			http.Error(
				w,
				"Invalid credentials",
				http.StatusUnauthorized,
			)
		case errors.Is(err, ErrUnknownDevice):
			http.Error(
				w,
				"Unknown device.",
				http.StatusUnauthorized,
			)
		default:
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(w).Encode(struct {
		AccessToken string `json:"access_token"`
	}{
		AccessToken: resp.AccessToken,
	})

	if err != nil {
		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
	}
}

func (a *AuthService) refreshHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")

	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "Refresh token cookie not found", http.StatusUnauthorized)
			return
		}
		// Handle any other unexpected errors
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp, err := a.refresh(cookie.Value)

	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRefreshToken):
			http.Error(
				w,
				err.Error(),
				http.StatusUnauthorized,
			)
		case errors.Is(err, ErrExpiredRefreshToken):
			http.Error(
				w,
				err.Error(),
				http.StatusUnauthorized,
			)
		case errors.Is(err, ErrReusedRefreshToken):
			http.Error(
				w,
				err.Error(),
				http.StatusUnauthorized,
			)
		default:
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(struct {
		AccessToken string `json:"access_token"`
	}{
		AccessToken: resp.AccessToken,
	})

	if err != nil {
		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
	}
}

func (a *AuthService) logoutHandler(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "missing auth claims", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID
	deviceID := claims.DeviceID

	err := a.logout(
		userID,
		deviceID,
	)

	if err != nil {
		http.Error(
			w,
			"Action failed",
			http.StatusInternalServerError,
		)
		return
	}

	// Delete the refresh token cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusNoContent)
}
