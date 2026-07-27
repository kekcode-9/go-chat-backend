package users

import "errors"

var (
	ErrUserNotFound = errors.New("user not found")
	ErrMissingQuery = errors.New("either email or username must be provided")
)
