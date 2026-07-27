package users

import (
	"github.com/google/uuid"
)

type UserLookupResponse struct {
	ID       uuid.UUID `json:"id"`
	UserName string    `json:"user_name"`
	Email    string    `json:"email"`
}
