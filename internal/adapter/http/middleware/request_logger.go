package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func NewRequestLogger(logger *slog.Logger) gin.HandlerFunc {
	logger = logger.With("component", "http_request_logger")

	return func(c *gin.Context) {
		startedAt := time.Now()
		requestLogger := logger.With(
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		requestLogger.InfoContext(c.Request.Context(), "HTTP request received")

		c.Next()

		requestLogger.InfoContext(c.Request.Context(), "HTTP response sent",
			"status", c.Writer.Status(),
			"duration", time.Since(startedAt),
		)
	}
}
