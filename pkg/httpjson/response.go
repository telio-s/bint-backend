package httpjson

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

const (
	SuccessCode      = 0
	UnknownErrorCode = 1
)

type SuccessResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   any    `json:"error"`
}

// ErrorDefinition describes the HTTP status and public envelope fields for an
// error response. The diagnostic error detail is supplied when it is written.
type ErrorDefinition struct {
	Status  int
	Code    int
	Message string
}

type Responder struct {
	logger *slog.Logger
}

func NewResponder(logger *slog.Logger) *Responder {
	return &Responder{logger: logger.With("component", "http_json_responder")}
}

func NewSuccess[T any](message string, data T) SuccessResponse[T] {
	return SuccessResponse[T]{Code: SuccessCode, Message: message, Data: data}
}

func NewError(code int, message string, detail any) ErrorResponse {
	if code == SuccessCode {
		code = UnknownErrorCode
	}
	if err, ok := detail.(error); ok {
		detail = err.Error()
	}
	return ErrorResponse{Code: code, Message: message, Error: detail}
}

func (r *Responder) Success(
	ctx context.Context,
	writer http.ResponseWriter,
	status int,
	message string,
	data any,
) {
	r.write(ctx, writer, status, NewSuccess(message, data))
}

func (r *Responder) Failure(
	ctx context.Context,
	writer http.ResponseWriter,
	status, code int,
	message string,
	detail any,
) {
	r.write(ctx, writer, status, NewError(code, message, detail))
}

func (r *Responder) write(
	ctx context.Context,
	writer http.ResponseWriter,
	status int,
	payload any,
) {
	body, err := json.Marshal(payload)
	if err != nil {
		r.logger.ErrorContext(ctx, "JSON response serialization failed", "error", err)
		status = http.StatusInternalServerError
		body, _ = json.Marshal(
			NewError(UnknownErrorCode, "The response could not be serialized.", err),
		)
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	if _, err := writer.Write(append(body, '\n')); err != nil {
		r.logger.ErrorContext(ctx, "JSON response write failed", "error", err)
	}
}
