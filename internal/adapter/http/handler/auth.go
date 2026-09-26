package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/telio-s/bint-backend.git/internal/adapter/http/dto"
	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/internal/domain/model"
	"github.com/telio-s/bint-backend.git/internal/port"
	"github.com/telio-s/bint-backend.git/pkg/httpjson"
)

const (
	accessCookieName        = "bint_access"
	refreshCookieName       = "bint_refresh"
	oauthStateCookieName    = "bint_oauth_state"
	oauthNonceCookieName    = "bint_oauth_nonce"
	oauthVerifierCookieName = "bint_oauth_verifier"
	userIDContextKey        = "authenticated_user_id"
)

type AuthConfig struct {
	CookieDomain            string
	CookieSecure            bool
	RefreshTokenTTL         time.Duration
	OAuthTemporaryCookieTTL time.Duration
	SuccessRedirectURL      string
	FailureRedirectURL      string
}

type AuthHandler struct {
	logger    *slog.Logger
	responder *httpjson.Responder
	service   port.AuthService
	tokens    port.TokenManager
	config    AuthConfig
}

func NewAuthHandler(
	logger *slog.Logger,
	responder *httpjson.Responder,
	service port.AuthService,
	tokens port.TokenManager,
	config AuthConfig,
) *AuthHandler {
	return &AuthHandler{
		logger:    logger.With("component", "auth_handler"),
		responder: responder,
		service:   service,
		tokens:    tokens,
		config:    config,
	}
}

func (h *AuthHandler) RegisterEmail(c *gin.Context) {
	var request dto.RegisterEmailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeError(c, apperror.ErrInvalidInput)
		return
	}
	result, err := h.service.RegisterEmail(
		c.Request.Context(),
		request.Email,
		request.Password,
		request.DisplayName,
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.setAuthCookies(c, result)
	h.responder.Success(
		c.Request.Context(),
		c.Writer,
		http.StatusCreated,
		"Account created successfully.",
		dto.AuthResponseFromModel(result),
	)
}

func (h *AuthHandler) LoginEmail(c *gin.Context) {
	var request dto.LoginEmailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.writeError(c, apperror.ErrInvalidInput)
		return
	}
	result, err := h.service.LoginEmail(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.setAuthCookies(c, result)
	h.responder.Success(
		c.Request.Context(),
		c.Writer,
		http.StatusOK,
		"Authenticated successfully.",
		dto.AuthResponseFromModel(result),
	)
}

func (h *AuthHandler) StartGoogle(c *gin.Context) {
	state, err := secureRandomString(32)
	if err != nil {
		h.writeError(c, err)
		return
	}
	nonce, err := secureRandomString(32)
	if err != nil {
		h.writeError(c, err)
		return
	}
	verifier, err := secureRandomString(64)
	if err != nil {
		h.writeError(c, err)
		return
	}
	challengeHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])

	h.setTemporaryCookie(c, oauthStateCookieName, state)
	h.setTemporaryCookie(c, oauthNonceCookieName, nonce)
	h.setTemporaryCookie(c, oauthVerifierCookieName, verifier)
	c.Redirect(http.StatusFound, h.service.GoogleAuthorizationURL(state, nonce, challenge))
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if oauthError := c.Query("error"); oauthError != "" {
		h.clearOAuthCookies(c)
		h.redirectFailure(c, oauthError)
		return
	}

	stateCookie, stateErr := c.Cookie(oauthStateCookieName)
	nonce, nonceErr := c.Cookie(oauthNonceCookieName)
	verifier, verifierErr := c.Cookie(oauthVerifierCookieName)
	state := c.Query("state")
	code := c.Query("code")
	h.clearOAuthCookies(c)
	if stateErr != nil || nonceErr != nil || verifierErr != nil || state == "" || code == "" ||
		!constantTimeEqual(stateCookie, state) {
		h.redirectFailure(c, "invalid_oauth_response")
		return
	}

	result, err := h.service.LoginGoogle(c.Request.Context(), code, verifier, nonce)
	if err != nil {
		h.logger.WarnContext(c.Request.Context(), "Google callback failed", "error", err)
		h.redirectFailure(c, "google_authentication_failed")
		return
	}
	h.setAuthCookies(c, result)
	c.Redirect(http.StatusFound, h.config.SuccessRedirectURL)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil {
		h.writeError(c, apperror.ErrInvalidToken)
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		h.clearAuthCookies(c)
		h.writeError(c, err)
		return
	}
	h.setAuthCookies(c, result)
	h.responder.Success(
		c.Request.Context(),
		c.Writer,
		http.StatusOK,
		"Session refreshed successfully.",
		dto.AuthResponseFromModel(result),
	)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie(refreshCookieName)
	if err := h.service.Logout(c.Request.Context(), refreshToken); err != nil {
		h.writeError(c, err)
		return
	}
	h.clearAuthCookies(c)
	h.responder.Success(
		c.Request.Context(),
		c.Writer,
		http.StatusOK,
		"Logged out successfully.",
		struct{}{},
	)
}

func (h *AuthHandler) CurrentSession(c *gin.Context) {
	userID, ok := c.Get(userIDContextKey)
	if !ok {
		h.writeError(c, apperror.ErrInvalidToken)
		return
	}
	user, err := h.service.CurrentUser(c.Request.Context(), userID.(string))
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.responder.Success(
		c.Request.Context(),
		c.Writer,
		http.StatusOK,
		"Session retrieved successfully.",
		dto.SessionResponse{
			Authenticated: true,
			User:          dto.UserResponseFromModel(user),
		},
	)
}

func (h *AuthHandler) Authenticate(c *gin.Context) {
	rawToken := bearerToken(c.GetHeader("Authorization"))
	if rawToken == "" {
		rawToken, _ = c.Cookie(accessCookieName)
	}
	claims, err := h.tokens.VerifyAccessToken(rawToken)
	if err != nil {
		h.writeError(c, apperror.ErrInvalidToken)
		c.Abort()
		return
	}
	c.Set(userIDContextKey, claims.UserID)
	c.Next()
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, result model.AuthResult) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		accessCookieName,
		result.AccessToken,
		maxAge(result.AccessExpiry),
		"/",
		h.config.CookieDomain,
		h.config.CookieSecure,
		true,
	)
	c.SetCookie(
		refreshCookieName,
		result.RefreshToken,
		int(h.config.RefreshTokenTTL.Seconds()),
		"/api/v1/auth",
		h.config.CookieDomain,
		h.config.CookieSecure,
		true,
	)
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessCookieName, "", -1, "/", h.config.CookieDomain, h.config.CookieSecure, true)
	c.SetCookie(
		refreshCookieName,
		"",
		-1,
		"/api/v1/auth",
		h.config.CookieDomain,
		h.config.CookieSecure,
		true,
	)
}

func (h *AuthHandler) setTemporaryCookie(c *gin.Context, name, value string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		name,
		value,
		int(h.config.OAuthTemporaryCookieTTL.Seconds()),
		"/api/v1/auth/google",
		h.config.CookieDomain,
		h.config.CookieSecure,
		true,
	)
}

func (h *AuthHandler) clearOAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	for _, name := range []string{oauthStateCookieName, oauthNonceCookieName, oauthVerifierCookieName} {
		c.SetCookie(
			name,
			"",
			-1,
			"/api/v1/auth/google",
			h.config.CookieDomain,
			h.config.CookieSecure,
			true,
		)
	}
}

func (h *AuthHandler) redirectFailure(c *gin.Context, code string) {
	target, err := url.Parse(h.config.FailureRedirectURL)
	if err != nil {
		h.writeError(c, errors.New("invalid configured failure redirect URL"))
		return
	}
	query := target.Query()
	query.Set("error", code)
	target.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, target.String())
}

func secureRandomString(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func constantTimeEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func bearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func maxAge(expiry time.Time) int {
	seconds := int(time.Until(expiry).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (h *AuthHandler) writeError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, 1500, "An unexpected error occurred."
	switch {
	case errors.Is(err, apperror.ErrInvalidInput):
		status, code, message = http.StatusBadRequest, 1001, "The request is invalid."
	case errors.Is(err, apperror.ErrInvalidCredentials):
		status, code, message = http.StatusUnauthorized, 1002, "The email or password is incorrect."
	case errors.Is(err, apperror.ErrInvalidToken):
		status, code, message = http.StatusUnauthorized, 1003, "The authentication token is invalid or expired."
	case errors.Is(err, apperror.ErrConflict):
		status, code, message = http.StatusConflict, 1004, "An account already exists for this email."
	case errors.Is(err, apperror.ErrNotFound):
		status, code, message = http.StatusNotFound, 1005, "The requested resource was not found."
	}
	h.responder.Failure(c.Request.Context(), c.Writer, status, code, message, err)
}
