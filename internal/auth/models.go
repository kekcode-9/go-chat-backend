package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// -----------------------------------------
// Claims model
// -----------------------------------------
type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	DeviceID uuid.UUID `json:"device_id"`

	jwt.RegisteredClaims
}

// -----------------------------------------
// Sign up models
// -----------------------------------------
type SignupRequest struct {
	UserName string `json:"user_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignupResponse struct {
	AccessToken  string
	RefreshToken string
}

// ----------------------------------------
// Login models
// ----------------------------------------
type LoginRequest struct {
	Email    string     `json:"email"`
	Password string     `json:"password"`
	DeviceID *uuid.UUID `json:"device_id"`
}

type LoginResponse struct {
	AccessToken  string
	RefreshToken string
	DeviceID     uuid.UUID
}

// ----------------------------------------
// Refresh request models
// ----------------------------------------

type RefreshResponse struct {
	AccessToken  string
	RefreshToken string
}

// ---------------------------------------
// Session query model
// ---------------------------------------
type RefreshSession struct {
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	SessionID uuid.UUID
	ExpiresAt time.Time
	Revoked   bool
}
