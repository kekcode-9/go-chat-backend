package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	UserName  string
	Email     string
	Password  string
	CreatedAt time.Time
}
