package http

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/telio-s/bint-backend.git/internal/adapter/http/apierror"
	"github.com/telio-s/bint-backend.git/internal/adapter/http/handler"
	"github.com/telio-s/bint-backend.git/internal/adapter/http/middleware"
	"github.com/telio-s/bint-backend.git/internal/docs"
	"github.com/telio-s/bint-backend.git/pkg/httpjson"
)

func NewRouter(
	logger *slog.Logger,
	responder *httpjson.Responder,
	auth *handler.AuthHandler,
) *gin.Engine {
	routerLogger := logger.With("component", "http_router")
	router := gin.New()
	router.Use(
		middleware.NewRequestLogger(logger),
		middleware.NewRecovery(logger, responder),
	)
	router.HandleMethodNotAllowed = true

	router.GET("/health", func(c *gin.Context) {
		data := struct {
			Status string `json:"status"`
		}{Status: "ok"}
		responder.Success(c.Request.Context(), c.Writer, http.StatusOK, "Service is healthy.", data)
	})
	router.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPI)
	})
	router.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", docs.ScalarHTML)
	})

	api := router.Group("/api/v1")
	registerAuthRoutes(api, auth)
	router.NoRoute(func(c *gin.Context) {
		apierror.WriteDefinition(
			c,
			responder,
			apierror.RouteNotFound,
			errors.New("route not found"),
		)
	})
	router.NoMethod(func(c *gin.Context) {
		apierror.WriteDefinition(
			c,
			responder,
			apierror.MethodNotAllowed,
			errors.New("method not allowed"),
		)
	})
	routerLogger.Debug("HTTP routes registered")

	return router
}

func registerAuthRoutes(api *gin.RouterGroup, auth *handler.AuthHandler) {
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/email/register", auth.RegisterEmail)
		authRoutes.POST("/email/login", auth.LoginEmail)
		authRoutes.GET("/google", auth.StartGoogle)
		authRoutes.GET("/google/callback", auth.GoogleCallback)
		authRoutes.POST("/refresh", auth.Refresh)
		authRoutes.POST("/logout", auth.Logout)
		authRoutes.GET("/session", auth.Authenticate, auth.CurrentSession)

		// TODO: Add email password-reset request and confirmation endpoints.
	}
}
