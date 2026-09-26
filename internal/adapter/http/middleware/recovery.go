package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/telio-s/bint-backend.git/pkg/httpjson"
)

func NewRecovery(logger *slog.Logger, responder *httpjson.Responder) gin.HandlerFunc {
	logger = logger.With("component", "http_recovery")

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("panic: %v", recovered)
				logger.ErrorContext(
					c.Request.Context(),
					"HTTP handler panic recovered",
					"error",
					err,
					"stack",
					string(debug.Stack()),
				)
				if !c.Writer.Written() {
					responder.Failure(
						c.Request.Context(),
						c.Writer,
						http.StatusInternalServerError,
						1500,
						"An unexpected error occurred.",
						err,
					)
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
