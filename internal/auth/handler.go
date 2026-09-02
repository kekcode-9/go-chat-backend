package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/mileusna/useragent"
)

const refreshTokenHeader = "X-Refresh-Token"

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
	userID *uuid.UUID,
	deviceID *uuid.UUID,
) {
	w.Header().Set(refreshTokenHeader, refreshToken)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := AuthResponse{
		AccessToken: accessToken,
		UserID:      userID,
		DeviceID:    deviceID,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("failed to encode auth response: %v", err)
	}
}

// signupHandler godoc
//
// @Summary Sign up a user
// @Description Creates a new user account and returns an access token. The refresh token is returned in the X-Refresh-Token response header.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body SignupRequest true "Signup request"
// @Success 201 {object} AuthResponse
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
		&resp.UserID,
		&resp.DeviceID,
	)
}

// loginHandler godoc
//
// @Summary Log in a user
// @Description Authenticates a user and returns an access token. The refresh token is returned in the X-Refresh-Token response header. When logging in from a new device, the device_id should be omitted from the request body. The server will create a new device_id in such cases and retrun it to frontend. From subsequent logins from the same device, the device_id should be sent in the request body.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} AuthResponse
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
		&resp.UserID,
		&resp.DeviceID,
	)
}

// refreshHandler godoc
//
// @Summary Refresh access token
// @Description Uses the X-Refresh-Token request header to issue a new access token and rotate the refresh token. The new refresh token is returned in the X-Refresh-Token response header.
// @Tags auth
// @Produce json
// @Param X-Refresh-Token header string true "Refresh token"
// @Success 200 {object} AuthResponse
// @Failure 401 {string} string "missing, invalid, expired, or reused refresh token"
// @Failure 500 {string} string "internal server error"
// @Router /auth/refresh/ [post]
func (a *AuthService) refreshHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	refreshToken := r.Header.Get(refreshTokenHeader)
	if refreshToken == "" {
		http.Error(
			w,
			"refresh token header not found",
			http.StatusUnauthorized,
		)
		return
	}

	resp, err := a.refresh(refreshToken)
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
		nil,
		nil,
	)
}

// logoutHandler godoc
//
// @Summary Log out current device
// @Description Revokes the current device session.
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

	w.WriteHeader(http.StatusNoContent)
}
