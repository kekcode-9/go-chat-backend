package models

import (
	"time"

	"github.com/google/uuid"
)

// ----------------------------------------
// User query model
// ----------------------------------------
type User struct {
	ID        uuid.UUID
	UserName  string
	Email     string
	Password  string
	CreatedAt time.Time
}
