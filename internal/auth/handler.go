package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/mileusna/useragent"
)

func (a *AuthService) RegisterRoutes(
	mux *http.ServeMux,
) {
	mux.HandleFunc("POST /auth/signup/", a.signupHandler)
	mux.HandleFunc("POST /auth/login/", a.loginHandler)
	mux.HandleFunc("POST /auth/refresh/", a.refreshHandler)
	mux.HandleFunc("POST /auth/logout/", a.logoutHandler)
}

func parseDeviceInfo(r *http.Request) (string, string) {
	ua := useragent.Parse(r.UserAgent())

	deviceName := ua.Name +
		" - " + ua.Version +
		" - " + ua.OS +
		" - " + ua.Device

	deviceType := ua.Device
	if deviceType == "" {
		deviceType = "Unknown"
	}

	return deviceName, deviceType
}

func writeAuthResponse(
	w http.ResponseWriter,
	status int,
	accessToken string,
	refreshToken string,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   false, // true in production
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(struct {
		AccessToken string `json:"access_token"`
	}{
		AccessToken: accessToken,
	}); err != nil {
		log.Printf("failed to encode auth response: %v", err)
	}
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false, // true in production
		SameSite: http.SameSiteStrictMode,
	})
}

// signupHandler godoc
//
// @Summary Sign up a user
// @Description Creates a new user account and returns an access token. The refresh token is set in an HttpOnly cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body SignupRequest true "Signup request"
// @Success 201 {object} map[string]string
// @Failure 400 {string} string "invalid request body"
// @Failure 409 {string} string "email already exists"
// @Failure 500 {string} string "internal server error"
// @Router /auth/signup/ [post]
func (a *AuthService) signupHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req SignupRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	deviceName, deviceType := parseDeviceInfo(r)

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
			log.Printf("signup failed: %v", err)
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	writeAuthResponse(
		w,
		http.StatusCreated,
		resp.AccessToken,
		resp.RefreshToken,
	)
}

// loginHandler godoc
//
// @Summary Log in a user
// @Description Authenticates a user and returns an access token. The refresh token is set in an HttpOnly cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "invalid request body"
// @Failure 401 {string} string "invalid credentials or unknown device"
// @Failure 404 {string} string "user not found"
// @Failure 500 {string} string "internal server error"
// @Router /auth/login/ [post]
func (a *AuthService) loginHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	/*
		If user is logging in for the first time from a device then they are expected to not send
		a device_id in request body. In such case server itself will create the device_id.
	*/
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	deviceName, deviceType := parseDeviceInfo(r)

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
				"invalid credentials",
				http.StatusUnauthorized,
			)
		case errors.Is(err, ErrUnknownDevice):
			http.Error(
				w,
				"unknown device",
				http.StatusUnauthorized,
			)
		default:
			log.Printf("login failed: %v", err)
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	writeAuthResponse(
		w,
		http.StatusOK,
		resp.AccessToken,
		resp.RefreshToken,
	)
}

// refreshHandler godoc
//
// @Summary Refresh access token
// @Description Uses the refresh_token cookie to issue a new access token and rotate the refresh token cookie.
// @Tags auth
// @Produce json
// @Param Cookie header string true "refresh_token cookie"
// @Success 200 {object} map[string]string
// @Failure 401 {string} string "missing, invalid, expired, or reused refresh token"
// @Failure 500 {string} string "internal server error"
// @Router /auth/refresh/ [post]
func (a *AuthService) refreshHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			http.Error(
				w,
				"refresh token cookie not found",
				http.StatusUnauthorized,
			)
			return
		}

		log.Printf("failed reading refresh cookie: %v", err)

		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
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
			log.Printf("refresh failed: %v", err)
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	writeAuthResponse(
		w,
		http.StatusOK,
		resp.AccessToken,
		resp.RefreshToken,
	)
}

// logoutHandler godoc
//
// @Summary Log out current device
// @Description Revokes the current device session and clears the refresh token cookie.
// @Tags auth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {string} string "missing auth claims"
// @Failure 500 {string} string "action failed"
// @Router /auth/logout/ [post]
func (a *AuthService) logoutHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(
			w,
			"missing auth claims",
			http.StatusUnauthorized,
		)
		return
	}

	if err := a.logout(
		claims.UserID,
		claims.DeviceID,
	); err != nil {
		log.Printf(
			"logout failed for user %s device %s: %v",
			claims.UserID,
			claims.DeviceID,
			err,
		)

		http.Error(
			w,
			"action failed",
			http.StatusInternalServerError,
		)
		return
	}

	clearRefreshCookie(w)

	w.WriteHeader(http.StatusNoContent)
}
