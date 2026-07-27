package auth

import "errors"

var (
	ErrEmailAlreadyExists  = errors.New("email already exists.")
	ErrUnknownDevice       = errors.New("Unknown device id.")
	ErrInvalidCredentials  = errors.New("invalid credentials.")
	ErrUserNotFound        = errors.New("Not existing user.")
	ErrInvalidRefreshToken = errors.New("Invalid refresh token.")
	ErrExpiredRefreshToken = errors.New("Session expired. Please Log in.")
	ErrReusedRefreshToken  = errors.New("Suspicious token reuse.")
)
