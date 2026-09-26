package apperror

import "errors"

var (
	ErrConflict           = errors.New("resource already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidToken       = errors.New("invalid token")
	ErrNotFound           = errors.New("resource not found")
)
