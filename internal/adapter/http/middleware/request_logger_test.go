package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerLogsRequestAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))

	router := gin.New()
	router.Use(NewRequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

	logs := output.String()
	for _, expected := range []string{
		`"msg":"HTTP request received"`,
		`"msg":"HTTP response sent"`,
		`"method":"GET"`,
		`"path":"/test"`,
		`"status":201`,
	} {
		if !strings.Contains(logs, expected) {
			t.Errorf("logs do not contain %q: %s", expected, logs)
		}
	}
}
