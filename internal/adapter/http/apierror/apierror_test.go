package apierror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
)

func TestFrom(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Definition
	}{
		{name: "invalid input", err: apperror.ErrInvalidInput, want: InvalidRequest},
		{name: "invalid credentials", err: apperror.ErrInvalidCredentials, want: InvalidCredentials},
		{name: "invalid token", err: apperror.ErrInvalidToken, want: InvalidToken},
		{name: "conflict", err: apperror.ErrConflict, want: Conflict},
		{name: "not found", err: apperror.ErrNotFound, want: ResourceNotFound},
		{
			name: "wrapped domain error",
			err:  fmt.Errorf("validate email: %w", apperror.ErrInvalidInput),
			want: InvalidRequest,
		},
		{name: "unknown error", err: errors.New("database unavailable"), want: Internal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := From(test.err); got != test.want {
				t.Fatalf("From() = %#v, want %#v", got, test.want)
			}
		})
	}
}
