package apierror

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/pkg/httpjson"
)

// Definition is the API error catalog's name for the reusable HTTP JSON error
// definition.
type Definition = httpjson.ErrorDefinition

var (
	InvalidRequest = Definition{
		Status:  http.StatusBadRequest,
		Code:    1001,
		Message: "The request is invalid.",
	}
	InvalidCredentials = Definition{
		Status:  http.StatusUnauthorized,
		Code:    1002,
		Message: "The email or password is incorrect.",
	}
	InvalidToken = Definition{
		Status:  http.StatusUnauthorized,
		Code:    1003,
		Message: "The authentication token is invalid or expired.",
	}
	Conflict = Definition{
		Status:  http.StatusConflict,
		Code:    1004,
		Message: "An account already exists for this email.",
	}
	ResourceNotFound = Definition{
		Status:  http.StatusNotFound,
		Code:    1005,
		Message: "The requested resource was not found.",
	}
	RouteNotFound = Definition{
		Status:  http.StatusNotFound,
		Code:    1005,
		Message: "The requested route was not found.",
	}
	MethodNotAllowed = Definition{
		Status:  http.StatusMethodNotAllowed,
		Code:    1006,
		Message: "The HTTP method is not allowed for this route.",
	}
	Internal = Definition{
		Status:  http.StatusInternalServerError,
		Code:    1500,
		Message: "An unexpected error occurred.",
	}
)

// From maps a domain error to its HTTP representation. It uses errors.Is so
// callers can add context without losing the mapping.
func From(err error) Definition {
	switch {
	case errors.Is(err, apperror.ErrInvalidInput):
		return InvalidRequest
	case errors.Is(err, apperror.ErrInvalidCredentials):
		return InvalidCredentials
	case errors.Is(err, apperror.ErrInvalidToken):
		return InvalidToken
	case errors.Is(err, apperror.ErrConflict):
		return Conflict
	case errors.Is(err, apperror.ErrNotFound):
		return ResourceNotFound
	default:
		return Internal
	}
}

// Write maps err and writes the standard JSON error envelope.
func Write(c *gin.Context, responder *httpjson.Responder, err error) {
	WriteDefinition(c, responder, From(err), err)
}

// WriteDefinition writes a known API error definition and its diagnostic detail.
func WriteDefinition(
	c *gin.Context,
	responder *httpjson.Responder,
	definition Definition,
	detail any,
) {
	responder.Failure(
		c.Request.Context(),
		c.Writer,
		definition.Status,
		definition.Code,
		definition.Message,
		detail,
	)
}
