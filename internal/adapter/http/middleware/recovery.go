package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/telio-s/bint-backend.git/internal/adapter/http/apierror"
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
					apierror.WriteDefinition(
						c,
						responder,
						apierror.Internal,
						err,
					)
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
